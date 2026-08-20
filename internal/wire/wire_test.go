package wire

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
	"github.com/pscheid92/voiceline-diary/internal/diary"
	"github.com/pscheid92/voiceline-diary/internal/session"
)

func TestCommand_TranslatesEveryThingTheBrowserCanDo(t *testing.T) {
	at := time.Date(2026, time.August, 20, 22, 30, 0, 0, time.FixedZone("CEST", 2*60*60))

	for _, tc := range []struct {
		msg  clientMsg
		want session.Command
	}{
		{clientMsg{Type: cmdAudio, Data: base64.StdEncoding.EncodeToString([]byte{1, 2})}, session.NewSpeakCommand([]byte{1, 2})},
		{clientMsg{Type: cmdEndTurn}, session.NewCommand(session.CmdEndTurn)},
		{clientMsg{Type: cmdEnd}, session.NewCommand(session.CmdFinish)},
		{clientMsg{Type: cmdAbort}, session.NewCommand(session.CmdAbandon)},
		{clientMsg{Type: cmdDiscard}, session.NewCommand(session.CmdDiscard)},
		{clientMsg{Type: cmdSave, At: at.Format(time.RFC3339)}, session.NewSaveCommand(at)},
	} {
		got, ok := command(tc.msg)

		require.True(t, ok, "%q reached nothing", tc.msg.Type)
		assert.Equal(t, tc.want.Kind, got.Kind, "%q", tc.msg.Type)
		assert.Equal(t, tc.want.Pcm, got.Pcm, "%q", tc.msg.Type)
		assert.True(t, tc.want.At.Equal(got.At), "%q: want %v, got %v", tc.msg.Type, tc.want.At, got.At)
	}
}

func TestCommand_IgnoresWhatIsNotOne(t *testing.T) {
	_, ok := command(clientMsg{Type: cmdAudio, Data: "not base64 at all!!"})
	assert.False(t, ok, "a garbled chunk is dropped where a lost packet would be")

	_, ok = command(clientMsg{Type: "something_else"})
	assert.False(t, ok, "a message this protocol never agreed to is not a command")
}

func TestEmit_GivesEveryEventExactlyOneMessage(t *testing.T) {
	entry := diary.NewEntry(diary.Day{Rating: 7, Emotion: "content"}, "Me: a good day\n")

	pairs := []struct {
		event session.Event
		want  string
	}{
		{session.Opened{}, msgReady},
		{session.TurnGrew{Turn: conversation.NewTurn(0, conversation.User)}, msgTurn},
		{session.DraftChanged{Day: entry.Day, Complete: true}, msgDraft},
		{session.Asking{Missing: []string{"rating"}}, msgAsking},
		{session.Concluded{Draft: entry, Caveat: session.CaveatSalvaged}, msgConcluded},
		{session.Filed{Entry: entry}, msgFiled},
		{session.Discarded{}, msgDiscarded},
		{session.Failed{Reason: session.ReasonSaveFailed}, msgError},
	}

	conn, browser := connected(t)

	var want []string
	for _, pair := range pairs {
		require.NoError(t, conn.Emit(context.Background(), pair.event))
		want = append(want, pair.want)
	}

	var got []string
	for range pairs {
		got = append(got, next(t, browser).Type)
	}

	assert.Equal(t, want, got)
}

func TestEmit_SaysWhetherTheDayChangedOrTheTalkEnded(t *testing.T) {
	entry := diary.NewEntry(diary.Day{Rating: 7, Emotion: "content"}, "Me: a good day\n")
	conn, browser := connected(t)
	ctx := context.Background()

	require.NoError(t, conn.Emit(ctx, session.DraftChanged{Day: entry.Day, Complete: true}))
	require.NoError(t, conn.Emit(ctx, session.Concluded{Draft: entry, Caveat: session.CaveatIncomplete}))

	changed := next(t, browser)
	assert.Equal(t, msgDraft, changed.Type)
	assert.True(t, changed.Complete, "the save button is a function of the draft")

	assert.EqualValues(t, 7, changed.Draft["day_rating"])
	assert.NotContains(t, changed.Draft, "transcript",
		"the browser is already given the words a turn at a time; sending them again on every note is the same words twice")

	over := next(t, browser)
	assert.Equal(t, msgConcluded, over.Type)
	assert.Equal(t, string(session.CaveatIncomplete), over.Code, "a code, not a sentence: the interface owns the language")
	assert.Contains(t, over.Draft, "transcript",
		"the entry they answer for carries the conversation it came from")
}

func TestPlay_SendsThePCMAsBase64(t *testing.T) {
	conn, browser := connected(t)

	conn.Play(context.Background(), []byte{1, 2, 3})

	msg := next(t, browser)
	assert.Equal(t, msgAudio, msg.Type)
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte{1, 2, 3}), msg.Data)
}

func TestHandler_ReportsAFailedDial(t *testing.T) {
	h := NewHandler(&mockDiary{}, func(context.Context, time.Time) (session.Companion, error) {
		return nil, errors.New("gemini is down")
	})

	server := httptest.NewServer(h)
	defer server.Close()

	browser := dial(t, server)
	msg := next(t, browser)

	assert.Equal(t, msgError, msg.Type)
	assert.Equal(t, string(session.ReasonDialFailed), msg.Code)
}

func TestHandler_DialsAtTheMomentTheBrowserSays(t *testing.T) {
	at := time.Date(2026, time.August, 20, 23, 59, 0, 0, time.UTC)
	dialed := make(chan time.Time, 1)

	h := NewHandler(&mockDiary{}, func(_ context.Context, now time.Time) (session.Companion, error) {
		dialed <- now
		return nil, errors.New("no companion needed for this")
	})

	server := httptest.NewServer(h)
	defer server.Close()

	ws, _, err := websocket.Dial(context.Background(),
		wsURL(server)+"?now="+at.Format(time.RFC3339), nil)
	require.NoError(t, err)
	defer func() { _ = ws.CloseNow() }()

	select {
	case got := <-dialed:
		assert.True(t, at.Equal(got), "want %v, got %v", at, got)
	case <-time.After(5 * time.Second):
		t.Fatal("the companion was never dialed")
	}
}

func connected(t *testing.T) (*Conn, *websocket.Conn) {
	t.Helper()

	conns := make(chan *Conn, 1)
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		conns <- New(ws)
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	t.Cleanup(func() {
		close(done)
		server.Close()
	})

	browser := dial(t, server)
	select {
	case conn := <-conns:
		return conn, browser
	case <-time.After(5 * time.Second):
		t.Fatal("the connection was never accepted")
		return nil, nil
	}
}

func dial(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()

	ws, _, err := websocket.Dial(context.Background(), wsURL(server), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.CloseNow() })
	return ws
}

func wsURL(server *httptest.Server) string {
	return "ws" + server.URL[len("http"):]
}

type received struct {
	Type     string         `json:"type"`
	Data     string         `json:"data"`
	Turn     map[string]any `json:"turn"`
	Code     string         `json:"code"`
	Draft    map[string]any `json:"draft"`
	Complete bool           `json:"complete"`
	Filed    map[string]any `json:"filed"`
}

func next(t *testing.T, browser *websocket.Conn) received {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var msg received
	require.NoError(t, wsjson.Read(ctx, browser, &msg))
	return msg
}

func TestHandler_RunsASessionOverTheSocketItAccepted(t *testing.T) {
	greeted := make(chan struct{}, 1)
	companion := &mockCompanion{}
	companion.EXPECT().Greet(mock.Anything).
		Run(func(context.Context) { greeted <- struct{}{} }).Return(nil)
	companion.EXPECT().Receive(mock.Anything).Return(conversation.Event{}, io.EOF)
	companion.EXPECT().Close().Return()

	h := NewHandler(&mockDiary{}, func(context.Context, time.Time) (session.Companion, error) {
		return companion, nil
	})

	server := httptest.NewServer(h)
	defer server.Close()

	browser := dial(t, server)

	assert.Equal(t, msgReady, next(t, browser).Type, "the browser is told it may start talking")

	select {
	case <-greeted:
	case <-time.After(5 * time.Second):
		t.Fatal("the session never had the companion open the conversation")
	}
}

func TestProtocol_TheBrowserHasWordsForEveryCodeWeSend(t *testing.T) {
	declared := regexp.MustCompile(`(?m)^\s*\w+\s+(?:Reason|Caveat)\s*=\s*"([a-z_]+)"`)

	events, err := os.ReadFile(filepath.Join("..", "session", "events.go"))
	require.NoError(t, err)

	var browser strings.Builder
	for _, name := range []string{"copy.js", "protocol.js", "useVoiceSession.js"} {
		part, err := os.ReadFile(filepath.Join("..", "..", "web", "frontend", "src", name))
		require.NoError(t, err)
		browser.Write(part)
	}

	codes := declared.FindAllStringSubmatch(string(events), -1)
	require.NotEmpty(t, codes, "the codes moved; this test can no longer find them")

	for _, code := range codes {
		assert.True(t, strings.Contains(browser.String(), code[1]+":"),
			"the service can send %q and the browser has nothing to say about it", code[1])
	}
}

func TestFlush_TellsTheBrowserToDropWhatIsQueued(t *testing.T) {
	conn, browser := connected(t)

	conn.Flush(context.Background())

	assert.Equal(t, msgInterrupted, next(t, browser).Type,
		"they talked over the companion; the rest of its answer is no longer an answer")
}

func TestClose_EndsTheSocketWithTheVerdict(t *testing.T) {
	conn, browser := connected(t)

	go conn.Close("saved")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var msg received
	err := wsjson.Read(ctx, browser, &msg)
	require.Error(t, err, "the browser must see the conversation end, not hang on a dead socket")
	assert.Equal(t, websocket.StatusNormalClosure, websocket.CloseStatus(err))
}

func TestInstant_FallsBackToAClockThatExists(t *testing.T) {
	sent := time.Date(2026, time.August, 20, 12, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	assert.True(t, sent.Equal(instant(sent.Format(time.RFC3339))),
		"a browser that sends its clock is believed, timezone and all")

	for _, nonsense := range []string{"", "not-a-time", "2026-13-45T99:99:99Z"} {
		got := instant(nonsense)

		assert.WithinDuration(t, time.Now(), got, time.Minute,
			"%q must not date somebody's diary to the year one", nonsense)
	}
}
