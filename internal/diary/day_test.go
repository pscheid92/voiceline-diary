package diary

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDay_AnUnansweredFieldStaysAbsent(t *testing.T) {
	unrated, err := json.Marshal(Day{Emotion: "content"})
	require.NoError(t, err)
	assert.NotContains(t, string(unrated), "day_rating", "an unrated day must not carry the field")

	moodless, err := json.Marshal(Day{Rating: 7})
	require.NoError(t, err)
	assert.NotContains(t, string(moodless), "emotion", "a day without a mood must not carry the field")
	assert.Contains(t, string(moodless), `"day_rating":7`, "a rating that was given must travel")

	for _, answer := range []string{`{"day_rating":null}`, `{"emotion":"tired"}`} {
		var d Day
		require.NoError(t, json.Unmarshal([]byte(answer), &d), answer)
		assert.Zero(t, d.Rating, "%s must read back as an unstated rating", answer)
	}
}

func TestDay_Validate(t *testing.T) {
	four := []string{"a", "b", "c", "d"}

	tests := []struct {
		name string
		day  Day
		ok   bool
	}{
		{"an unstated rating", Day{}, true},
		{"the lower bound", Day{Rating: MinRating}, true},
		{"the upper bound", Day{Rating: MaxRating}, true},
		{"one over the scale", Day{Rating: MaxRating + 1}, false},
		{"a negative rating", Day{Rating: -1}, false},
		{"three things that went well", Day{WentWell: four[:MaxListItems]}, true},
		{"a fourth thing that went well", Day{WentWell: four}, false},
		{"a fourth thing that went badly", Day{WentBadly: four}, false},
		{"a fourth thing to do", Day{Todos: four}, true},
		{"nothing said at all", Day{WentWell: []string{"quiet afternoon"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.day.Validate()
			if tt.ok {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, ErrMalformed, "every rule a day is filed under carries the same sentinel")
		})
	}
}

func TestDay_OwesTheTwoAnswersItCannotBeInferred(t *testing.T) {
	for _, day := range []Day{
		{WentWell: []string{"a walk"}},
		{Rating: 7},
		{Emotion: "content"},
		{Rating: 7, Emotion: "content"},
	} {
		assert.Equal(t, len(day.Owes()) == 0, day.IsComplete(),
			"a day is complete exactly when it owes nothing: %+v", day)
	}

	assert.Equal(t, []string{"rating", "emotion"}, Day{WentWell: []string{"a walk"}}.Owes())
	assert.Equal(t, []string{"emotion"}, Day{Rating: 7}.Owes())
	assert.True(t, Day{Rating: 7, Emotion: "content"}.IsComplete(), "both answers are there")
}
