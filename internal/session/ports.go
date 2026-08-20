package session

import (
	"context"
	"time"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
	"github.com/pscheid92/voiceline-diary/internal/diary"
)

type User interface {
	Read(ctx context.Context) (Command, error)
	Play(ctx context.Context, pcm []byte)
	Flush(ctx context.Context)
	Emit(ctx context.Context, ev Event) error
	Close(reason string)
}

type Companion interface {
	Greet(ctx context.Context) error
	AskFor(ctx context.Context, missing []string) error
	SendAudio(ctx context.Context, pcm []byte) error
	EndTurn(ctx context.Context) error
	Receive(ctx context.Context) (conversation.Event, error)
	Answer(ctx context.Context, call conversation.Call, refusal string) error
	Close()
}

type Diary interface {
	File(ctx context.Context, entry diary.Entry, now time.Time) (string, error)
}

type Dial func(ctx context.Context, now time.Time) (Companion, error)

type CommandKind string

const (
	CmdSpeak   CommandKind = "speak"
	CmdEndTurn CommandKind = "end_turn"
	CmdFinish  CommandKind = "finish"
	CmdAbandon CommandKind = "abandon"
	CmdSave    CommandKind = "save"
	CmdDiscard CommandKind = "discard"
)

type Command struct {
	Kind CommandKind
	Pcm  []byte
	At   time.Time
}

func NewCommand(kind CommandKind) Command {
	return Command{Kind: kind}
}

func NewSpeakCommand(pcm []byte) Command {
	return Command{Kind: CmdSpeak, Pcm: pcm}
}

func NewSaveCommand(at time.Time) Command {
	return Command{Kind: CmdSave, At: at}
}
