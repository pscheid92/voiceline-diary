package diary

import (
	"fmt"
	"strings"
)

const (
	MaxListItems = 3
	MinRating    = 1
	MaxRating    = 10
)

type Day struct {
	Rating    int      `json:"day_rating,omitempty"`
	Emotion   string   `json:"emotion,omitempty"`
	WentWell  []string `json:"went_well"`
	WentBadly []string `json:"went_badly"`
	Todos     []string `json:"todos"`
}

func (d Day) Validate() error {
	switch {
	case d.Rating != 0 && (d.Rating < MinRating || d.Rating > MaxRating):
		return fmt.Errorf("%w: rating %d not in %d..%d", ErrMalformed, d.Rating, MinRating, MaxRating)

	case len(d.WentWell) > MaxListItems:
		return fmt.Errorf("%w: went well has %d items, at most %d", ErrMalformed, len(d.WentWell), MaxListItems)

	case len(d.WentBadly) > MaxListItems:
		return fmt.Errorf("%w: went badly has %d items, at most %d", ErrMalformed, len(d.WentBadly), MaxListItems)
	}

	return nil
}

func (d Day) IsComplete() bool {
	return len(d.Owes()) == 0
}

func (d Day) Owes() []string {
	var unanswered []string

	if d.Rating == 0 {
		unanswered = append(unanswered, "rating")
	}

	if strings.TrimSpace(d.Emotion) == "" {
		unanswered = append(unanswered, "emotion")
	}

	return unanswered
}
