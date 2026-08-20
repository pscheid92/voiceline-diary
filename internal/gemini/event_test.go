package gemini

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"github.com/pscheid92/voiceline-diary/internal/conversation"
)

func TestHeard_TakesTheTranscribersTagsOut(t *testing.T) {
	msg := &genai.LiveServerMessage{
		ServerContent: &genai.LiveServerContent{
			InputTranscription: &genai.Transcription{Text: "a good day <noise> mostly"},
		},
	}

	ev, ok := heard(msg)

	assert.True(t, ok)
	assert.Equal(t, "a good day  mostly", ev.InputTranscript,
		"the tag goes, the spacing around it stays: fragments are joined downstream")
}

func TestHeard_AFragmentThatIsOnlyATagIsNotAnEvent(t *testing.T) {
	msg := &genai.LiveServerMessage{
		ServerContent: &genai.LiveServerContent{
			InputTranscription: &genai.Transcription{Text: "<BACKGROUND NOISE>"},
		},
	}

	_, ok := heard(msg)

	assert.False(t, ok, "nothing was said")
}

func TestHeard_DeliversAMessageThatOnlyAsks(t *testing.T) {
	msg := &genai.LiveServerMessage{ToolCall: &genai.LiveServerToolCall{
		FunctionCalls: []*genai.FunctionCall{
			{ID: "a", Name: ratingTool, Args: map[string]any{"rating": float64(7)}},
			{ID: "b", Name: emotionTool, Args: map[string]any{"emotion": "decent"}},
			{ID: "c", Name: wentWellTool, Args: map[string]any{"item": "a run"}},
			{ID: "d", Name: finishTool},
		},
	}}

	ev, ok := heard(msg)

	require.True(t, ok, "a message asking for the day to be written down is worth reacting to")
	require.Len(t, ev.Calls, 4, "the order they were asked in is the order they are answered in")
	assert.Equal(t, conversation.Call{ID: "a", Kind: conversation.CallRating, Rating: 7}, ev.Calls[0])
	assert.Equal(t, conversation.Call{ID: "b", Kind: conversation.CallEmotion, Text: "decent"}, ev.Calls[1])
	assert.Equal(t, conversation.Call{ID: "c", Kind: conversation.CallWentWell, Text: "a run"}, ev.Calls[2])
	assert.Equal(t, conversation.Call{ID: "d", Kind: conversation.CallFinish}, ev.Calls[3])
}

func TestHeard_PassesOnACallItCannotMakeSenseOf(t *testing.T) {
	msg := &genai.LiveServerMessage{ToolCall: &genai.LiveServerToolCall{
		FunctionCalls: []*genai.FunctionCall{{ID: "a", Name: ratingTool, Args: map[string]any{"rating": "seven"}}},
	}}

	ev, ok := heard(msg)

	require.True(t, ok)
	require.Len(t, ev.Calls, 1, "the model is owed an answer to everything it asked")
	assert.Zero(t, ev.Calls[0].Rating, "and no scale contains nothing, so it will be refused")
}

func TestHeard_DropsAToolItNeverOffered(t *testing.T) {
	msg := &genai.LiveServerMessage{ToolCall: &genai.LiveServerToolCall{
		FunctionCalls: []*genai.FunctionCall{{ID: "a", Name: "delete_everything"}},
	}}

	_, ok := heard(msg)

	assert.False(t, ok)
}

func TestToolFor_NamesEveryKindThisPackageCanProduce(t *testing.T) {
	kinds := []conversation.CallKind{
		conversation.CallRating, conversation.CallEmotion, conversation.CallWentWell,
		conversation.CallWentBadly, conversation.CallTodo, conversation.CallFinish,
	}

	for _, kind := range kinds {
		assert.NotEmpty(t, toolFor(kind), "no tool answers to %q", kind)
	}
}
