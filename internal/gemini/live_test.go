package gemini

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
)

func speaks(t *testing.T, now time.Time) *LiveSession {
	t.Helper()

	key := os.Getenv("GEMINI_API_KEY")
	if os.Getenv("GEMINI_LIVE_TEST") == "" || key == "" {
		t.Skip("live check: set GEMINI_LIVE_TEST=1 and GEMINI_API_KEY to run it")
	}

	model := os.Getenv("GEMINI_LIVE_MODEL")
	if model == "" {
		model = "gemini-3.1-flash-live-preview"
	}

	ctx := context.Background()
	companion, err := NewCompanion(ctx, key, model)
	require.NoError(t, err)

	session, err := companion.Dial(ctx, now)
	require.NoError(t, err)
	t.Cleanup(session.Close)
	return session
}

func turn(t *testing.T, s *LiveSession, line string, refuse func(conversation.Call) string) (said string, calls []conversation.Call) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	require.NoError(t, s.send(genai.LiveRealtimeInput{Text: line}))

	var heard strings.Builder
	for {
		ev, err := s.Receive(ctx)
		require.NoError(t, err, "the model stopped talking: %q so far", heard.String())

		heard.WriteString(ev.OutputTranscript)
		for _, call := range ev.Calls {
			calls = append(calls, call)
			refusal := ""
			if refuse != nil {
				refusal = refuse(call)
			}
			require.NoError(t, s.Answer(ctx, call, refusal))
		}

		if ev.TurnComplete {
			return strings.TrimSpace(heard.String()), calls
		}
	}
}

func TestLive_KeepsTalkingAfterItRecordsSomething(t *testing.T) {
	s := speaks(t, time.Date(2026, time.August, 20, 21, 30, 0, 0, time.UTC))

	lines := []string{
		"I finally called my mum today, it was really good to hear her voice.",
		"The other good thing was that I fixed a flaky test that had been bothering me for weeks.",
		"And my run felt easy for once.",
	}
	for _, line := range lines {
		said, calls := turn(t, s, line, nil)
		if len(calls) == 0 {
			continue
		}
		assert.NotEmpty(t, said, "it recorded and then said nothing at all")
		return
	}
	t.Skip("the model never wrote anything down, so there was no silence to catch")
}

func TestLive_ARefusedLineDoesNotDerailTheTalk(t *testing.T) {
	s := speaks(t, time.Date(2026, time.August, 20, 21, 30, 0, 0, time.UTC))

	kept := 0
	refuse := func(call conversation.Call) string {
		if call.Kind != conversation.CallWentWell {
			return ""
		}
		if kept++; kept > 3 {
			return "a day holds at most three things that went well and already has that many, so that one was not recorded."
		}
		return ""
	}

	said, _ := turn(t, s,
		"Four good things today: I fixed the flaky test, I had a good lunch, my run felt easy, and I finally called my mum.",
		refuse)

	require.Greater(t, kept, 3, "the test needs a fourth item to refuse")
	assert.NotEmpty(t, said, "a refusal must not leave the person talking to silence")
}

func TestLive_AsksAgainWhenItIsNotAllowedToFinish(t *testing.T) {
	s := speaks(t, time.Date(2026, time.August, 20, 21, 30, 0, 0, time.UTC))

	turn(t, s, "Today was fine, I fixed a flaky test that had been bothering me.", nil)

	refuse := func(call conversation.Call) string {
		if call.Kind == conversation.CallFinish {
			return owedRating
		}
		return ""
	}
	said, calls := turn(t, s, "Right, that's everything. I'm done, good night!", refuse)

	if len(calls) == 0 {
		t.Log("it never asked to end: the prompt held it back before the refusal had to")
	}
	require.NotEmpty(t, said, "a person who cannot leave has to be told why")
	assert.Contains(t, said, "?", "it should be asking for what is missing, not signing off: %q", said)
}

const owedRating = "the entry still has no rating. Ask for what is missing before saying goodbye."

func TestLive_WalksThroughTheWholeDayAndWritesItDown(t *testing.T) {
	s := speaks(t, time.Date(2026, time.August, 20, 21, 30, 0, 0, time.UTC))

	var got []conversation.CallKind
	for _, line := range []string{
		"My day is great.",
		"The weather. It's perfect weather right now.",
		"I read a book, which was great. And also I overslept, which wasn't that great.",
		"I was too late to an appointment.",
		"No, nothing else went badly.",
		"I have to run a couple of errands tomorrow.",
		"I would say a solid five.",
		"Warm.",
	} {
		_, calls := turn(t, s, line, nil)
		for _, call := range calls {
			got = append(got, call.Kind)
		}
	}
	t.Logf("recorded: %v", got)

	for _, want := range []conversation.CallKind{
		conversation.CallWentWell, conversation.CallWentBadly, conversation.CallTodo,
		conversation.CallRating, conversation.CallEmotion,
	} {
		assert.Contains(t, got, want, "they said it plainly and it never reached the entry")
	}
}
