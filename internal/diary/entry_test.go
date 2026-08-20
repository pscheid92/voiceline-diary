package diary

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntry_CompleteAsksADifferentQuestionThanValidate(t *testing.T) {
	unrated := Entry{
		Day:        Day{WentWell: []string{"a walk"}},
		Transcript: "Me: a quiet day.\n",
	}

	require.NoError(t, unrated.Validate(), "an unrated day is well formed")

	err := unrated.Complete()
	require.ErrorIs(t, err, ErrIncomplete)
	assert.NotErrorIs(t, err, ErrMalformed, "an incomplete entry must not read as a malformed one")

	answered := unrated
	answered.Rating, answered.Emotion = 7, "content"
	assert.NoError(t, answered.Complete(), "both answers are there")
}

func TestEntry_ValidateRequiresTheConversation(t *testing.T) {
	entry := Entry{Day: Day{Rating: 7}, Transcript: "  \n "}

	err := entry.Validate()
	assert.ErrorIs(t, err, ErrTranscriptEmpty, "a blank transcript is no conversation")
	assert.NotErrorIs(t, err, ErrMalformed,
		"an empty transcript is nobody's malformed answer: nothing was ever said")

	entry.Transcript = "Me: a quiet day."
	assert.NoError(t, entry.Validate())
}

func TestEntry_ValidateCarriesTheDaysOwnRules(t *testing.T) {
	entry := Entry{Day: Day{Rating: 11}, Transcript: "Me: a quiet day."}

	assert.ErrorIs(t, entry.Validate(), ErrMalformed,
		"an entry is no more filable than the day inside it")
}
