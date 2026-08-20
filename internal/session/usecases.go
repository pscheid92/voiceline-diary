package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/pscheid92/voiceline-diary/internal/diary"
)

func (s *Session) speak(ctx context.Context, pcm []byte) {
	if err := s.companion.SendAudio(ctx, pcm); err != nil {
		s.unheard++
	}
}

func (s *Session) endTurn(ctx context.Context) {
	if err := s.companion.EndTurn(ctx); err != nil {
		zerolog.Ctx(ctx).Warn().Err(err).Msg("session: forcing turn end failed")
		return
	}

	zerolog.Ctx(ctx).Info().Msg("session: turn ended by the person")
}

func (s *Session) finishEntry(ctx context.Context) (missing []string, ok bool) {
	missing = s.day.Owes()
	if len(missing) == 0 {
		return nil, true
	}

	s.emit(ctx, Asking{Missing: missing})
	return missing, false
}

func (s *Session) save(ctx context.Context, at time.Time) {
	entry, err := s.file(ctx, at)

	// positive case!
	if err == nil {
		s.emit(ctx, Filed{Entry: entry})
		s.user.Close("saved")
		return
	}

	reason := ReasonSaveFailed
	if errors.Is(err, diary.ErrIncomplete) {
		reason = ReasonIncomplete
	}

	zerolog.Ctx(ctx).Error().Err(err).Msg("session: saving failed")
	s.emit(ctx, Failed{Reason: reason})
}

func (s *Session) file(ctx context.Context, at time.Time) (diary.Entry, error) {
	if err := s.draft.Validate(); err != nil {
		return diary.Entry{}, err
	}

	if err := s.draft.Complete(); err != nil {
		return diary.Entry{}, err
	}

	url, err := s.diary.File(ctx, s.draft, at)
	if err != nil {
		return diary.Entry{}, err
	}

	zerolog.Ctx(ctx).Info().Str("url", url).Msg("session: entry filed")

	entry := s.draft
	entry.URL = url
	return entry, nil
}

func (s *Session) discard(ctx context.Context) {
	s.emit(ctx, Discarded{})
	s.user.Close("discarded")
}

func (s *Session) noteRating(ctx context.Context, rating int) string {
	if rating < diary.MinRating || rating > diary.MaxRating {
		return fmt.Sprintf(
			"a day is rated from %d to %d and that was not, so nothing was recorded. Ask them for a number in that range.",
			diary.MinRating,
			diary.MaxRating,
		)
	}

	s.day.Rating = rating
	s.noted(ctx)
	return ""
}

func (s *Session) noteEmotion(ctx context.Context, emotion string) string {
	emotion = strings.TrimSpace(emotion)
	if emotion == "" {
		return "not recorded: no mood was given"
	}

	s.day.Emotion = emotion
	s.noted(ctx)
	return ""
}

func (s *Session) noteWentWell(ctx context.Context, item string) string {
	if len(s.day.WentWell) >= diary.MaxListItems {
		return noRoomFor("things that went well", item)
	}

	return s.jot(ctx, &s.day.WentWell, item)
}

func (s *Session) noteWentBadly(ctx context.Context, item string) string {
	if len(s.day.WentBadly) >= diary.MaxListItems {
		return noRoomFor("things that went badly", item)
	}

	return s.jot(ctx, &s.day.WentBadly, item)
}

func (s *Session) noteTodo(ctx context.Context, item string) string {
	return s.jot(ctx, &s.day.Todos, item)
}

func (s *Session) jot(ctx context.Context, held *[]string, item string) string {
	item = strings.TrimSpace(item)
	if item == "" {
		return "not recorded: nothing was said"
	}

	*held = append(*held, item)
	s.noted(ctx)
	return ""
}

func noRoomFor(what, item string) string {
	return fmt.Sprintf(
		"not recorded: a day holds at most %d %s and already has that many, so %q did not fit",
		diary.MaxListItems,
		what,
		item,
	)
}

func (s *Session) noted(ctx context.Context) {
	s.asked = false
	event := DraftChanged{Day: s.day, Complete: s.day.IsComplete()}
	s.emit(ctx, event)
}
