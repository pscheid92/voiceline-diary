package session

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
)

func TestAppend_MergesFragmentsOfTheSameRole(t *testing.T) {
	var s notes
	s.Append(conversation.User, "Today was")
	s.Append(conversation.User, " a good")
	turn, ok := s.Append(conversation.User, " day.")

	require.True(t, ok)
	require.Len(t, s.Turns, 1)
	assert.Equal(t, "Today was a good day.", turn.Text, "the fragments join with their spacing")
	assert.Equal(t, 0, turn.Index)
}

func TestAppend_RoleChangeStartsANewTurn(t *testing.T) {
	var s notes
	s.Append(conversation.User, "A good day.")
	turn, _ := s.Append(conversation.Companion, "Glad to hear it.")

	require.Len(t, s.Turns, 2)
	assert.Equal(t, conversation.Companion, turn.Role)
	assert.Equal(t, 1, turn.Index)
	assert.Equal(t, "A good day.", s.Turns[0].Text, "the speaker's turn stays untouched")
}

func TestCloseTurn_NextFragmentStartsANewTurn(t *testing.T) {
	var s notes
	s.Append(conversation.User, "First thought.")
	s.CloseTurn()
	turn, _ := s.Append(conversation.User, "Second thought.")

	require.Len(t, s.Turns, 2, "a closed turn must not grow")
	assert.True(t, s.Turns[0].Closed)
	assert.Equal(t, 1, turn.Index)
	assert.Equal(t, "Second thought.", turn.Text)
}

func TestAppend_KeepsSpacingAndRefusesSilence(t *testing.T) {
	var s notes
	s.Append(conversation.User, "Hello")
	turn, _ := s.Append(conversation.User, " world")

	assert.Equal(t, "Hello world", turn.Text, "trimming fragments would glue words together")

	_, ok := s.Append(conversation.User, "  ")
	assert.False(t, ok, "whitespace is not speech")
	assert.Len(t, s.Turns, 1, "a rejected fragment must not open a turn")
}

func TestTranscript_RendersBothSides(t *testing.T) {
	var s notes
	s.Append(conversation.Companion, "How was your day?")
	s.CloseTurn()
	s.Append(conversation.User, "Maybe a seven.")

	assert.Equal(t, "Companion: How was your day?\nMe: Maybe a seven.\n", s.Transcript())
}
