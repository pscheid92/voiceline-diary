package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
	"github.com/pscheid92/voiceline-diary/internal/diary"
)

const farewellGrace = 5 * time.Second

type Session struct {
	user      User
	companion Companion
	diary     Diary

	notes   *notes
	day     diary.Day
	asked   bool
	draft   diary.Entry
	unheard int
	unseen  int

	commands chan Command
	events   chan conversation.Event
	lost     error
}

func New(user User, companion Companion, book Diary) *Session {
	return &Session{
		user:      user,
		companion: companion,
		diary:     book,
		notes:     &notes{},
		commands:  make(chan Command, 8),
		events:    make(chan conversation.Event, 32),
	}
}

func (s *Session) Run(ctx context.Context) {
	ctx, hangUp := context.WithCancel(ctx)
	defer hangUp()

	if err := s.user.Emit(ctx, Opened{}); err != nil {
		return
	}

	if err := s.companion.Greet(ctx); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("session: greeting failed")
	}

	s.listen(ctx)

	ending := s.converse(ctx)
	defer s.report(ctx, ending)

	if ending == EndingClientGone {
		return
	}
	s.review(ctx, ending)
}

func (s *Session) listen(ctx context.Context) {
	go func() {
		defer close(s.commands)
		_ = pump(ctx, s.user.Read, s.commands)
	}()

	go func() {
		defer close(s.events)
		s.lost = pump(ctx, s.companion.Receive, s.events)
	}()
}

func pump[T any](ctx context.Context, read func(context.Context) (T, error), out chan<- T) error {
	for {
		v, err := read(ctx)
		if err != nil {
			return err
		}

		select {
		case out <- v:
			// empty
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Session) converse(ctx context.Context) Ending {
	var farewell <-chan time.Time

	for {
		select {
		case ev, ok := <-s.events:
			if !ok {
				zerolog.Ctx(ctx).Error().Err(s.lost).Msg("session: companion lost")
				return EndingCompanionLost
			}

			if s.hear(ctx, ev) && farewell == nil {
				farewell = time.After(farewellGrace)
			}

			if farewell != nil && ev.TurnComplete {
				return EndingGoodbye
			}

		case <-farewell:
			zerolog.Ctx(ctx).Debug().Msg("session: the farewell outlasted its welcome")
			return EndingGoodbye

		case cmd, ok := <-s.commands:
			if !ok {
				return EndingClientGone
			}
			if ending := s.obey(ctx, cmd); ending != "" {
				return ending
			}
		}
	}
}

func (s *Session) hear(ctx context.Context, ev conversation.Event) (leaving bool) {
	if ev.Interrupted {
		s.user.Flush(ctx)
	}

	if len(ev.Audio) > 0 {
		s.user.Play(ctx, ev.Audio)
	}

	s.write(ctx, conversation.User, ev.InputTranscript)
	s.write(ctx, conversation.Companion, ev.OutputTranscript)

	for _, call := range ev.Calls {
		refusal, ending := s.settle(ctx, call)

		if err := s.companion.Answer(ctx, call, refusal); err != nil {
			zerolog.Ctx(ctx).
				Warn().
				Err(err).
				Str("call", string(call.Kind)).
				Msg("session: answering the companion failed")
		}

		leaving = leaving || ending
	}

	if ev.TurnComplete {
		s.notes.CloseTurn()
	}

	return leaving
}

func (s *Session) settle(ctx context.Context, call conversation.Call) (refusal string, leaving bool) {
	switch call.Kind {
	case conversation.CallRating:
		return s.noteRating(ctx, call.Rating), false
	case conversation.CallEmotion:
		return s.noteEmotion(ctx, call.Text), false
	case conversation.CallWentWell:
		return s.noteWentWell(ctx, call.Text), false
	case conversation.CallWentBadly:
		return s.noteWentBadly(ctx, call.Text), false
	case conversation.CallTodo:
		return s.noteTodo(ctx, call.Text), false
	case conversation.CallFinish:
		if missing, ok := s.finishEntry(ctx); !ok {
			return owed(missing), false
		}
		return "", true
	}

	return "", false
}

func owed(missing []string) string {
	return fmt.Sprintf("the entry still has no %s. Ask them for that one thing before saying goodbye.", missing[0])
}

func (s *Session) obey(ctx context.Context, cmd Command) Ending {
	switch cmd.Kind {
	case CmdSpeak:
		s.speak(ctx, cmd.Pcm)
	case CmdEndTurn:
		s.endTurn(ctx)
	case CmdFinish:
		return s.finish(ctx)
	case CmdAbandon:
		return EndingAbandoned
	}

	return ""
}

func (s *Session) finish(ctx context.Context) Ending {
	missing, ok := s.finishEntry(ctx)
	if ok {
		return EndingFinished
	}

	if s.asked {
		return ""
	}

	s.asked = true
	zerolog.Ctx(ctx).Info().Strs("missing", missing).Msg("session: finishing waits for the missing answers")

	if err := s.companion.AskFor(ctx, missing); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("session: could not ask for the missing answers")
	}

	return ""
}

func (s *Session) review(ctx context.Context, ending Ending) {
	s.companion.Close()

	transcript := s.notes.Transcript()
	s.draft = diary.NewEntry(s.day, transcript)

	if err := s.draft.Validate(); err != nil {
		reason := ReasonReadFailed
		if errors.Is(err, diary.ErrTranscriptEmpty) {
			reason = ReasonNothingSaid
		}

		zerolog.Ctx(ctx).Error().Err(err).Str("ending", string(ending)).Msg("session: nothing to draft")

		s.emit(ctx, Failed{Reason: reason})
		return
	}

	caveat := NoCaveat
	switch {
	case !s.draft.IsComplete():
		caveat = CaveatIncomplete
	case ending == EndingCompanionLost:
		caveat = CaveatSalvaged
	}

	event := Concluded{Draft: s.draft, Caveat: caveat}
	if err := s.user.Emit(ctx, event); err != nil {
		return
	}

	for cmd := range s.commands {
		switch cmd.Kind {
		case CmdSave:
			s.save(ctx, cmd.At)
			return
		case CmdDiscard:
			s.discard(ctx)
			return
		}
	}
}

func (s *Session) write(ctx context.Context, role conversation.Role, fragment string) {
	if turn, ok := s.notes.Append(role, fragment); ok {
		s.emit(ctx, TurnGrew{Turn: turn})
	}
}

func (s *Session) emit(ctx context.Context, ev Event) {
	if err := s.user.Emit(ctx, ev); err != nil {
		s.unseen++
	}
}

func (s *Session) report(ctx context.Context, ending Ending) {
	zerolog.Ctx(ctx).Info().
		Str("ending", string(ending)).
		Int("turns", len(s.notes.Turns)).
		Int("unheard", s.unheard).
		Int("unseen", s.unseen).
		Strs("unanswered", s.day.Owes()).
		Msg("session: conversation ended")
}
