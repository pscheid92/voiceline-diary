package gemini

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/genai"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
	"github.com/pscheid92/voiceline-diary/internal/diary"
)

const maxResumes = 3
const inputMIME = "audio/pcm;rate=16000"

var ErrSessionLost = errors.New("live session lost")
var errGoingAway = errors.New("provider is closing the connection")

type LiveSession struct {
	redial dialer
	cfg    genai.LiveConnectConfig
	handle string

	mu     sync.RWMutex
	conn   connection
	closed bool

	writeMu sync.Mutex
}

func newLiveSession(ctx context.Context, redial dialer, now time.Time) (*LiveSession, error) {
	s := &LiveSession{redial: redial, cfg: *liveConfig(now)}
	if err := s.open(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *LiveSession) Greet(_ context.Context) error {
	return s.send(genai.LiveRealtimeInput{Text: greetNudgePrompt})
}

func (s *LiveSession) AskFor(_ context.Context, missing []string) error {
	return s.send(genai.LiveRealtimeInput{Text: finishNudge(missing)})
}

func (s *LiveSession) SendAudio(_ context.Context, pcm []byte) error {
	return s.send(genai.LiveRealtimeInput{Audio: &genai.Blob{Data: pcm, MIMEType: inputMIME}})
}

func (s *LiveSession) EndTurn(_ context.Context) error {
	return s.send(genai.LiveRealtimeInput{AudioStreamEnd: true})
}

func (s *LiveSession) Receive(ctx context.Context) (conversation.Event, error) {
	resumes := 0
	for {
		connection := s.current()
		if connection == nil {
			return conversation.Event{}, io.EOF
		}

		msg, err := connection.Receive()
		if err == nil && (msg == nil || msg.GoAway != nil) {
			err = errGoingAway
		}
		if err != nil {
			if s.current() == nil {
				return conversation.Event{}, io.EOF
			}
			if s.handle != "" && resumes < maxResumes && s.open(ctx) == nil {
				resumes++
				continue
			}
			return conversation.Event{}, fmt.Errorf("%w: %v", ErrSessionLost, err)
		}

		if u := msg.SessionResumptionUpdate; u != nil && u.Resumable && u.NewHandle != "" {
			s.handle = u.NewHandle
		}
		if ev, ok := heard(msg); ok {
			return ev, nil
		}
	}
}

func (s *LiveSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	if s.conn != nil {
		_ = s.conn.Close()
	}
}

func (s *LiveSession) open(ctx context.Context) error {
	cfg := s.cfg
	cfg.SessionResumption = &genai.SessionResumptionConfig{Handle: s.handle}

	connection, err := s.redial(ctx, &cfg)
	if err != nil {
		return fmt.Errorf("%w: %v", diary.ErrProvider, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		_ = connection.Close()
		return ErrSessionLost
	}
	s.conn = connection
	return nil
}

func (s *LiveSession) send(input genai.LiveRealtimeInput) error {
	connection := s.current()
	if connection == nil {
		return ErrSessionLost
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	if err := connection.SendRealtimeInput(input); err != nil {
		return fmt.Errorf("%w: %v", ErrSessionLost, err)
	}
	return nil
}

func (s *LiveSession) Answer(_ context.Context, call conversation.Call, refusal string) error {
	connection := s.current()
	if connection == nil {
		return ErrSessionLost
	}

	response := &genai.FunctionResponse{
		ID:       call.ID,
		Name:     toolFor(call.Kind),
		Response: map[string]any{"output": "recorded"},
	}
	if refusal != "" {
		response.Response = map[string]any{"error": refusal}
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	input := genai.LiveToolResponseInput{FunctionResponses: []*genai.FunctionResponse{response}}
	if err := connection.SendToolResponse(input); err != nil {
		return fmt.Errorf("%w: %v", ErrSessionLost, err)
	}
	return nil
}

func (s *LiveSession) current() connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil
	}
	return s.conn
}
