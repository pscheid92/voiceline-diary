package session

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	mock "github.com/stretchr/testify/mock"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
	"github.com/pscheid92/voiceline-diary/internal/diary"
)

var errBlind = errors.New("the browser stopped listening")

type answered struct {
	call    conversation.Call
	refusal string
}

type talk struct {
	*Session
	user      *mockUser
	companion *mockCompanion
	book      *mockDiary

	commands chan Command
	said     chan conversation.Event
	heard    chan Event
	answers  chan answered
	carried  chan []byte

	deaf  error
	blind atomic.Bool

	seen []Event
	done chan struct{}
}

func begin(t *testing.T) *talk { return beginWith(t, nil) }

func beginWith(t *testing.T, prepare func(*talk)) *talk {
	t.Helper()

	c := &talk{
		user:      newMockUser(t),
		companion: newMockCompanion(t),
		book:      newMockDiary(t),
		commands:  make(chan Command, 8),
		said:      make(chan conversation.Event, 16),
		heard:     make(chan Event, 64),
		answers:   make(chan answered, 16),
		carried:   make(chan []byte, 16),
		done:      make(chan struct{}),
	}

	c.user.EXPECT().Read(mock.Anything).RunAndReturn(func(ctx context.Context) (Command, error) {
		select {
		case cmd, ok := <-c.commands:
			if !ok {
				return Command{}, io.EOF
			}
			return cmd, nil
		case <-ctx.Done():
			return Command{}, ctx.Err()
		}
	}).Maybe()

	c.user.EXPECT().Emit(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, ev Event) error {
			c.heard <- ev
			if c.blind.Load() {
				return errBlind
			}
			return nil
		}).Maybe()
	c.user.EXPECT().Play(mock.Anything, mock.Anything).Return().Maybe()
	c.user.EXPECT().Flush(mock.Anything).Return().Maybe()
	c.user.EXPECT().Close(mock.Anything).Return().Maybe()

	c.companion.EXPECT().Receive(mock.Anything).RunAndReturn(func(ctx context.Context) (conversation.Event, error) {
		select {
		case ev, ok := <-c.said:
			if !ok {
				return conversation.Event{}, io.EOF
			}
			return ev, nil
		case <-ctx.Done():
			return conversation.Event{}, ctx.Err()
		}
	}).Maybe()

	c.companion.EXPECT().Answer(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, call conversation.Call, refusal string) {
			c.answers <- answered{call: call, refusal: refusal}
		}).Return(nil).Maybe()
	c.companion.EXPECT().Greet(mock.Anything).Return(nil).Maybe()
	c.companion.EXPECT().AskFor(mock.Anything, mock.Anything).Return(nil).Maybe()
	c.companion.EXPECT().SendAudio(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, pcm []byte) error {
			c.carried <- pcm
			return c.deaf
		}).Maybe()
	c.companion.EXPECT().EndTurn(mock.Anything).Return(nil).Maybe()
	c.companion.EXPECT().Close().Return().Maybe()

	if prepare != nil {
		prepare(c)
	}

	c.Session = New(c.user, c.companion, c.book)
	go func() {
		defer close(c.done)
		c.Run(context.Background())
	}()
	return c
}

func (c *talk) says(cmd Command) { c.commands <- cmd }

func (c *talk) leaves() { close(c.commands) }

func (c *talk) hears(ev conversation.Event) { c.said <- ev }

func (c *talk) loses() { close(c.said) }

func (c *talk) over(t *testing.T) {
	t.Helper()

	select {
	case <-c.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the conversation never ended")
	}

	for {
		select {
		case ev := <-c.heard:
			c.seen = append(c.seen, ev)
		default:
			return
		}
	}
}

func await[T Event](t *testing.T, c *talk) T {
	t.Helper()

	for {
		if found, ok := last[T](c.seen); ok {
			return found
		}
		select {
		case ev := <-c.heard:
			c.seen = append(c.seen, ev)
		case <-time.After(5 * time.Second):
			var want T
			t.Fatalf("the person was never told %T", want)
		}
	}
}

func spoken(t *testing.T, c *talk, n int) [][]byte {
	t.Helper()

	var got [][]byte
	for len(got) < n {
		select {
		case pcm := <-c.carried:
			got = append(got, pcm)
		case <-time.After(5 * time.Second):
			t.Fatalf("the companion was sent %d chunks, wanted %d", len(got), n)
		}
	}
	return got
}

func settled(t *testing.T, c *talk, n int) []answered {
	t.Helper()

	var got []answered
	for len(got) < n {
		select {
		case a := <-c.answers:
			got = append(got, a)
		case <-time.After(5 * time.Second):
			t.Fatalf("the companion was answered %d times, wanted %d", len(got), n)
		}
	}
	return got
}

func told[T Event](c *talk) []T {
	var found []T
	for _, ev := range c.seen {
		if e, ok := ev.(T); ok {
			found = append(found, e)
		}
	}
	return found
}

func last[T Event](events []Event) (T, bool) {
	var found T
	var ok bool
	for _, ev := range events {
		if e, is := ev.(T); is {
			found, ok = e, true
		}
	}
	return found, ok
}

func refusals(answers []answered) []answered {
	var refused []answered
	for _, a := range answers {
		if a.refusal != "" {
			refused = append(refused, a)
		}
	}
	return refused
}

func said(role conversation.Role, text string) conversation.Event {
	if role == conversation.User {
		return conversation.Event{InputTranscript: text}
	}
	return conversation.Event{OutputTranscript: text}
}

func calls(cs ...conversation.Call) conversation.Event { return conversation.Event{Calls: cs} }

func rating(n int) conversation.Call {
	return conversation.Call{ID: "r", Kind: conversation.CallRating, Rating: n}
}

func emotion(s string) conversation.Call {
	return conversation.Call{ID: "e", Kind: conversation.CallEmotion, Text: s}
}

func wentWell(item string) conversation.Call {
	return conversation.Call{ID: "w", Kind: conversation.CallWentWell, Text: item}
}

func wentBadly(item string) conversation.Call {
	return conversation.Call{ID: "b", Kind: conversation.CallWentBadly, Text: item}
}

func todo(item string) conversation.Call {
	return conversation.Call{ID: "t", Kind: conversation.CallTodo, Text: item}
}

var goodbye = conversation.Call{ID: "f", Kind: conversation.CallFinish}

func aDayWorthFiling(c *talk) {
	c.hears(said(conversation.User, "A quiet day, went for a run."))
	c.hears(calls(wentWell("a run"), rating(7), emotion("content")))
}

func filed(c *talk) []diary.Entry {
	var entries []diary.Entry
	for _, call := range c.book.Calls {
		if call.Method == "File" {
			entries = append(entries, call.Arguments.Get(1).(diary.Entry))
		}
	}
	return entries
}
