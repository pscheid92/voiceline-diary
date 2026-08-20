package diary

import "errors"

var (
	ErrProvider        = errors.New("upstream provider failed")
	ErrMalformed       = errors.New("the day read back malformed")
	ErrIncomplete      = errors.New("entry is missing answers a day owes")
	ErrTranscriptEmpty = errors.New("transcript is empty")
)
