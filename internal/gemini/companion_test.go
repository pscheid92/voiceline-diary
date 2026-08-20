package gemini

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/genai"

	"github.com/pscheid92/voiceline-diary/internal/diary"
)

func TestPrompt_SpeaksEnglish(t *testing.T) {
	assert.Contains(t, companionPrompt(time.Time{}), "Speak only English")
}

func TestPrompt_TellsTheTime(t *testing.T) {
	at := time.Date(2026, time.August, 17, 21, 4, 0, 0, time.UTC)
	assert.Contains(t, companionPrompt(at), "It is currently Monday, 17 August, 21:04.")
}

func TestPrompt_TakesNoTextFromTheClient(t *testing.T) {
	p := companionPrompt(time.Date(2026, time.August, 17, 21, 4, 0, 0, time.UTC))
	assert.NotContains(t, p, "[", "a bracket in the prompt is the app-note marker")
	assert.NotContains(t, p, "]", "a bracket in the prompt is the app-note marker")
}

func TestPrompt_LeavesNoPlaceholderUnfilled(t *testing.T) {
	assert.NotContains(t, companionPrompt(time.Now()), "{{",
		"a placeholder the prompt file names and the code does not fill reaches the model verbatim")
}

func TestFinishNudge_NamesWhatIsMissing(t *testing.T) {
	nudge := finishNudge([]string{"rating", "emotion"})
	assert.Contains(t, nudge, "no rating and no emotion",
		"the companion is told everything the day still owes, so it can plan the close")
	assert.True(t, len(nudge) > 2 && nudge[0] == '[' && nudge[len(nudge)-1] == ']', "an app note must be bracketed, or the companion reads it aloud: %q", nudge)
}

func TestLiveConfig_EveryToolBlocks(t *testing.T) {
	for _, tool := range liveConfig(time.Time{}).Tools {
		for _, fn := range tool.FunctionDeclarations {
			assert.Equal(t, genai.BehaviorBlocking, fn.Behavior, "%s", fn.Name)
		}
	}
}

func TestLiveConfig_LetsTheProviderDecideWhenToCompress(t *testing.T) {
	compression := liveConfig(time.Time{}).ContextWindowCompression

	require.NotNil(t, compression, "without it the provider ends an audio session at fifteen minutes")
	require.NotNil(t, compression.SlidingWindow)
	assert.Nil(t, compression.TriggerTokens,
		"the default is 80%% of the model's window; a number of our own only ever compresses sooner")
}

func TestPrompt_TakesTheScaleFromTheDomain(t *testing.T) {
	p := companionPrompt(time.Now())

	assert.Contains(t, p, fmt.Sprintf("%d to %d", diary.MinRating, diary.MaxRating))
	assert.NotContains(t, p, "{{", "a placeholder the prompt names and the code does not fill reaches the model verbatim")
}

func TestLiveConfig_PinsTheLanguageItSpeaks(t *testing.T) {
	cfg := liveConfig(time.Time{})

	require.NotNil(t, cfg.SpeechConfig, "transcription config only settles the transcript, not the voice")
	assert.Equal(t, languageCode, cfg.SpeechConfig.LanguageCode)
	assert.Equal(t, languageCode, cfg.InputAudioTranscription.LanguageCodes[0])
	assert.Equal(t, languageCode, cfg.OutputAudioTranscription.LanguageCodes[0])
}
