package diary

import (
	"fmt"
	"strings"
)

type Entry struct {
	Day
	Transcript string `json:"transcript"`
	URL        string `json:"url,omitempty"`
}

func NewEntry(day Day, transcript string) Entry {
	return Entry{Day: day, Transcript: transcript}
}

func (e Entry) Validate() error {
	if err := e.Day.Validate(); err != nil {
		return err
	}

	if strings.TrimSpace(e.Transcript) == "" {
		return ErrTranscriptEmpty
	}

	return nil
}

func (e Entry) Complete() error {
	if e.IsComplete() {
		return nil
	}

	return fmt.Errorf("%w: %v", ErrIncomplete, e.Owes())
}
