package wire

import (
	"encoding/base64"
	"time"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
	"github.com/pscheid92/voiceline-diary/internal/diary"
	"github.com/pscheid92/voiceline-diary/internal/session"
)

const (
	cmdAudio   = "audio"
	cmdEndTurn = "turn_end"
	cmdEnd     = "end"
	cmdAbort   = "abort"
	cmdSave    = "save"
	cmdDiscard = "discard"
)

const (
	msgReady       = "ready"
	msgAudio       = "audio"
	msgTurn        = "turn"
	msgInterrupted = "interrupted"
	msgDraft       = "draft"
	msgConcluded   = "concluded"
	msgFiled       = "filed"
	msgDiscarded   = "discarded"
	msgAsking      = "asking"
	msgError       = "error"
)

type clientMsg struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	At   string `json:"at,omitempty"`
}

type draft interface {
	IsComplete() bool
	Owes() []string
}

type serverMsg struct {
	Type  string             `json:"type"`
	Data  string             `json:"data,omitempty"`
	Turn  *conversation.Turn `json:"turn,omitempty"`
	Code  string             `json:"code,omitempty"`
	Draft draft              `json:"draft,omitempty"`

	Complete bool         `json:"complete,omitempty"`
	Filed    *diary.Entry `json:"filed,omitempty"`
}

func command(msg clientMsg) (session.Command, bool) {
	switch msg.Type {
	case cmdAudio:
		pcm, err := base64.StdEncoding.DecodeString(msg.Data)
		if err != nil {
			return session.Command{}, false
		}
		return session.NewSpeakCommand(pcm), true

	case cmdSave:
		return session.NewSaveCommand(instant(msg.At)), true

	case cmdEndTurn:
		return session.NewCommand(session.CmdEndTurn), true

	case cmdEnd:
		return session.NewCommand(session.CmdFinish), true

	case cmdAbort:
		return session.NewCommand(session.CmdAbandon), true

	case cmdDiscard:
		return session.NewCommand(session.CmdDiscard), true
	}

	return session.Command{}, false
}

func instant(value string) time.Time {
	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Now()
	}

	return at
}
