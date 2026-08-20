package session

import (
	"github.com/pscheid92/voiceline-diary/internal/conversation"
	"github.com/pscheid92/voiceline-diary/internal/diary"
)

type Event interface {
	isEvent()
}

type Opened struct{}

type TurnGrew struct {
	Turn conversation.Turn
}

type DraftChanged struct {
	Day      diary.Day
	Complete bool
}

type Asking struct {
	Missing []string
}

type Concluded struct {
	Draft  diary.Entry
	Caveat Caveat
}

type Caveat string

const (
	NoCaveat         Caveat = ""
	CaveatSalvaged   Caveat = "salvaged"
	CaveatIncomplete Caveat = "incomplete"
)

type Filed struct {
	Entry diary.Entry
}

type Discarded struct{}

type Failed struct {
	Reason Reason
}

func (Opened) isEvent()       {}
func (TurnGrew) isEvent()     {}
func (DraftChanged) isEvent() {}
func (Asking) isEvent()       {}
func (Concluded) isEvent()    {}
func (Filed) isEvent()        {}
func (Discarded) isEvent()    {}
func (Failed) isEvent()       {}

type Reason string

const (
	ReasonDialFailed  Reason = "dial_failed"
	ReasonReadFailed  Reason = "read_failed"
	ReasonNothingSaid Reason = "nothing_said"
	ReasonSaveFailed  Reason = "save_failed"
	ReasonIncomplete  Reason = "incomplete"
)

type Ending string

const (
	EndingGoodbye       Ending = "goodbye"
	EndingFinished      Ending = "finished"
	EndingAbandoned     Ending = "abandoned"
	EndingCompanionLost Ending = "companion_lost"
	EndingClientGone    Ending = "client_gone"
)
