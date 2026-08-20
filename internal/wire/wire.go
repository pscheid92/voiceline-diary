package wire

import (
	"context"
	"encoding/base64"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/pscheid92/voiceline-diary/internal/session"
)

type Conn struct {
	ws *websocket.Conn
}

func New(ws *websocket.Conn) *Conn {
	return &Conn{ws: ws}
}

func (c *Conn) Read(ctx context.Context) (session.Command, error) {
	for {
		var msg clientMsg
		if err := wsjson.Read(ctx, c.ws, &msg); err != nil {
			return session.Command{}, err
		}

		if cmd, ok := command(msg); ok {
			return cmd, nil
		}
	}
}

func (c *Conn) Play(ctx context.Context, pcm []byte) {
	c.write(ctx, serverMsg{Type: msgAudio, Data: base64.StdEncoding.EncodeToString(pcm)})
}

func (c *Conn) Flush(ctx context.Context) {
	c.write(ctx, serverMsg{Type: msgInterrupted})
}

func (c *Conn) Emit(ctx context.Context, ev session.Event) error {
	msg, ok := message(ev)
	if !ok {
		return nil
	}

	return wsjson.Write(ctx, c.ws, msg)
}

func message(ev session.Event) (serverMsg, bool) {
	switch e := ev.(type) {
	case session.Opened:
		return serverMsg{Type: msgReady}, true
	case session.TurnGrew:
		return serverMsg{Type: msgTurn, Turn: &e.Turn}, true
	case session.DraftChanged:
		return serverMsg{Type: msgDraft, Draft: e.Day, Complete: e.Complete}, true
	case session.Asking:
		return serverMsg{Type: msgAsking}, true
	case session.Concluded:
		return serverMsg{Type: msgConcluded, Draft: e.Draft, Code: string(e.Caveat)}, true
	case session.Filed:
		return serverMsg{Type: msgFiled, Filed: &e.Entry}, true
	case session.Discarded:
		return serverMsg{Type: msgDiscarded}, true
	case session.Failed:
		return serverMsg{Type: msgError, Code: string(e.Reason)}, true
	default:
		return serverMsg{}, false
	}
}

func (c *Conn) Close(reason string) {
	_ = c.ws.Close(websocket.StatusNormalClosure, reason)
}

func (c *Conn) write(ctx context.Context, msg serverMsg) {
	_ = wsjson.Write(ctx, c.ws, msg)
}
