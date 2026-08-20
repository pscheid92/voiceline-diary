package gemini

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
)

type socket struct {
	*mockConnection
	messages chan *genai.LiveServerMessage
}

func newSocket(t *testing.T, drop error) *socket {
	t.Helper()

	c := &socket{
		mockConnection: newMockConnection(t),
		messages:       make(chan *genai.LiveServerMessage, 8),
	}

	c.EXPECT().Receive().RunAndReturn(func() (*genai.LiveServerMessage, error) {
		msg, ok := <-c.messages
		if !ok {
			return nil, drop
		}
		return msg, nil
	}).Maybe()
	c.EXPECT().SendRealtimeInput(mock.Anything).Return(nil).Maybe()
	c.EXPECT().SendToolResponse(mock.Anything).Return(nil).Maybe()
	c.EXPECT().Close().Return(nil).Maybe()

	return c
}

func (c *socket) says(msg *genai.LiveServerMessage) { c.messages <- msg }

func (c *socket) drops() { close(c.messages) }

func (c *socket) sent() []genai.LiveRealtimeInput {
	var inputs []genai.LiveRealtimeInput
	for _, call := range c.Calls {
		if call.Method == "SendRealtimeInput" {
			inputs = append(inputs, call.Arguments.Get(0).(genai.LiveRealtimeInput))
		}
	}
	return inputs
}

func (c *socket) answered() []*genai.FunctionResponse {
	var responses []*genai.FunctionResponse
	for _, call := range c.Calls {
		if call.Method == "SendToolResponse" {
			input := call.Arguments.Get(0).(genai.LiveToolResponseInput)
			responses = append(responses, input.FunctionResponses...)
		}
	}
	return responses
}

func transcribed(text string) *genai.LiveServerMessage {
	return &genai.LiveServerMessage{
		ServerContent: &genai.LiveServerContent{
			InputTranscription: &genai.Transcription{Text: text},
		},
	}
}

func resumable(handle string) *genai.LiveServerMessage {
	return &genai.LiveServerMessage{
		SessionResumptionUpdate: &genai.LiveServerSessionResumptionUpdate{Resumable: true, NewHandle: handle},
	}
}

type provider struct {
	conns []*socket
	dials int
}

func offering(conns ...*socket) *provider { return &provider{conns: conns} }

func (p *provider) redial(context.Context, *genai.LiveConnectConfig) (connection, error) {
	p.dials++
	if p.dials > len(p.conns) {
		return p.conns[len(p.conns)-1], nil
	}
	return p.conns[p.dials-1], nil
}

func session(t *testing.T, redial dialer) *LiveSession {
	t.Helper()

	s := &LiveSession{redial: redial, cfg: *liveConfig(time.Now())}
	require.NoError(t, s.open(context.Background()))
	return s
}

func nextEvent(t *testing.T, s *LiveSession) conversation.Event {
	t.Helper()

	type result struct {
		ev  conversation.Event
		err error
	}
	done := make(chan result, 1)
	go func() {
		ev, err := s.Receive(context.Background())
		done <- result{ev, err}
	}()

	select {
	case r := <-done:
		require.NoError(t, r.err, "the session stopped delivering")
		return r.ev
	case <-time.After(5 * time.Second):
		t.Fatal("no event arrived")
		return conversation.Event{}
	}
}

func TestLiveSession_SendsAudioAsThePCMTheAPIExpects(t *testing.T) {
	c := newSocket(t, nil)
	s := session(t, offering(c).redial)

	require.NoError(t, s.SendAudio(context.Background(), []byte{1, 2}))

	require.Len(t, c.sent(), 1)
	assert.Equal(t, inputMIME, c.sent()[0].Audio.MIMEType)
}

func TestLiveSession_ResumesADroppedConnectionInvisibly(t *testing.T) {
	first := newSocket(t, errors.New("connection reset"))
	second := newSocket(t, errors.New("connection reset"))
	provider := offering(first, second)
	s := session(t, provider.redial)

	first.says(resumable("h1"))
	first.says(transcribed("before the drop"))

	assert.Equal(t, "before the drop", nextEvent(t, s).InputTranscript)

	first.drops()
	second.says(transcribed("after the drop"))

	assert.Equal(t, "after the drop", nextEvent(t, s).InputTranscript,
		"the caller must not see the connection die and come back")
	assert.Equal(t, 2, provider.dials, "the session must have reconnected exactly once")
}

func TestLiveSession_AGoAwayIsRepairedLikeADrop(t *testing.T) {
	first := newSocket(t, errors.New("unused"))
	second := newSocket(t, errors.New("connection reset"))
	provider := offering(first, second)
	s := session(t, provider.redial)

	first.says(resumable("h1"))
	first.says(&genai.LiveServerMessage{GoAway: &genai.LiveServerGoAway{}})
	second.says(transcribed("after the goaway"))

	assert.Equal(t, "after the goaway", nextEvent(t, s).InputTranscript,
		"the conversation must resume after a GoAway")
	assert.Equal(t, 2, provider.dials, "a GoAway must lead to one reconnect")
}

func TestLiveSession_SurvivesMoreRotationsThanTheResumeCap(t *testing.T) {
	const rotations = maxResumes + 3

	var dials int
	s := session(t, func(context.Context, *genai.LiveConnectConfig) (connection, error) {
		dials++
		c := newSocket(t, errGoingAway)
		c.says(resumable("h"))
		c.says(transcribed("still here"))
		if dials <= rotations {
			c.drops()
		}
		return c, nil
	})

	for i := 0; i <= rotations; i++ {
		assert.Equal(t, "still here", nextEvent(t, s).InputTranscript, "rotation %d", i)
	}
}

func TestLiveSession_GivesUpAfterMaxResumes(t *testing.T) {
	var dials int
	s := session(t, func(context.Context, *genai.LiveConnectConfig) (connection, error) {
		dials++
		c := newSocket(t, errors.New("connection reset"))
		if dials == 1 {
			c.says(resumable("h"))
		}
		c.drops()
		return c, nil
	})

	done := make(chan error, 1)
	go func() {
		_, err := s.Receive(context.Background())
		done <- err
	}()

	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Receive never gave up")
	}

	assert.ErrorIs(t, err, ErrSessionLost, "the caller is told the session is lost for good")
	assert.Equal(t, 1+maxResumes, dials, "one dial, then maxResumes attempts to repair it")
}

func TestLiveSession_WithoutAHandleADropIsFinal(t *testing.T) {
	var dials int
	s := session(t, func(context.Context, *genai.LiveConnectConfig) (connection, error) {
		dials++
		c := newSocket(t, errors.New("connection reset"))
		c.drops()
		return c, nil
	})

	_, err := s.Receive(context.Background())

	assert.ErrorIs(t, err, ErrSessionLost)
	assert.Equal(t, 1, dials, "no handle means no attempt to resume")
}

func TestLiveSession_CloseEndsTheRead(t *testing.T) {
	c := newSocket(t, errors.New("connection reset"))
	s := session(t, offering(c).redial)

	s.Close()
	c.drops()

	_, err := s.Receive(context.Background())

	assert.ErrorIs(t, err, io.EOF, "a closed session ends the read rather than failing it")
	c.AssertCalled(t, "Close")
}

func TestLiveSession_AnswersACallWithWhatTheSessionDecided(t *testing.T) {
	c := newSocket(t, nil)
	s := session(t, offering(c).redial)
	call := conversation.Call{ID: "c1", Kind: conversation.CallWentWell}

	require.NoError(t, s.Answer(context.Background(), call, ""))
	require.NoError(t, s.Answer(context.Background(), call, "it did not fit"))

	require.Len(t, c.answered(), 2)

	accepted := c.answered()[0]
	assert.Equal(t, "c1", accepted.ID)
	assert.Equal(t, wentWellTool, accepted.Name, "the answer has to name the tool it came from")
	assert.Contains(t, accepted.Response, "output")

	refused := c.answered()[1]
	assert.Equal(t, "it did not fit", refused.Response["error"],
		"the reason is what the companion repeats, so it travels verbatim")

	assert.Empty(t, accepted.Scheduling)
	assert.Empty(t, refused.Scheduling)
}

func TestLiveSession_AnswersOnWhicheverConnectionItNowHas(t *testing.T) {
	first, second := newSocket(t, nil), newSocket(t, nil)
	s := session(t, offering(first, second).redial)

	call := conversation.Call{ID: "c1", Kind: conversation.CallWentWell}
	require.NoError(t, s.open(context.Background()))

	require.NoError(t, s.Answer(context.Background(), call, ""))

	assert.Empty(t, first.answered(), "the connection that asked is gone")
	require.Len(t, second.answered(), 1,
		"a resumed conversation may still be waiting on this call, and every tool here blocks")
	assert.Equal(t, "c1", second.answered()[0].ID)
}

func TestLiveSession_OpensTheConversationWithoutWaitingToBeSpokenTo(t *testing.T) {
	c := newSocket(t, nil)
	s := session(t, offering(c).redial)

	require.NoError(t, s.Greet(context.Background()))

	require.Len(t, c.sent(), 1)
	assert.Equal(t, greetNudgePrompt, c.sent()[0].Text,
		"nobody has said anything yet, so the companion is nudged rather than answered")
}

func TestLiveSession_AsksForWhatTheDayStillOwes(t *testing.T) {
	c := newSocket(t, nil)
	s := session(t, offering(c).redial)

	require.NoError(t, s.AskFor(context.Background(), []string{"rating", "emotion"}))

	require.Len(t, c.sent(), 1)
	assert.Contains(t, c.sent()[0].Text, "no rating and no emotion")
}

func TestLiveSession_EndsTheTurnOnDemand(t *testing.T) {
	c := newSocket(t, nil)
	s := session(t, offering(c).redial)

	require.NoError(t, s.EndTurn(context.Background()))

	require.Len(t, c.sent(), 1)
	assert.True(t, c.sent()[0].AudioStreamEnd,
		"the person said they were done rather than waiting for the room to go quiet")
}

func TestLiveSession_WritesNothingOnceItIsClosed(t *testing.T) {
	c := newSocket(t, nil)
	s := session(t, offering(c).redial)

	s.Close()

	assert.ErrorIs(t, s.SendAudio(context.Background(), []byte{1}), ErrSessionLost)
	assert.ErrorIs(t, s.Answer(context.Background(), conversation.Call{}, ""), ErrSessionLost)
	assert.Empty(t, c.sent())
	assert.Empty(t, c.answered())
}
