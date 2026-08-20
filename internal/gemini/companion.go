package gemini

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"time"

	"google.golang.org/genai"

	"github.com/pscheid92/voiceline-diary/internal/diary"
)

//go:embed prompt.md
var diaryPrompt string

const languageCode = "en-US"

type Companion struct {
	api   *genai.Client
	model string
}

func NewCompanion(ctx context.Context, apiKey, model string) (*Companion, error) {
	api, err := connect(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	companion := Companion{api: api, model: model}
	return &companion, nil
}

func (c *Companion) Dial(ctx context.Context, now time.Time) (*LiveSession, error) {
	redial := func(ctx context.Context, cfg *genai.LiveConnectConfig) (connection, error) {
		return c.api.Live.Connect(ctx, c.model, cfg)
	}

	return newLiveSession(ctx, redial, now)
}

const (
	finishTool    = "finish_entry"
	ratingTool    = "record_rating"
	emotionTool   = "record_emotion"
	wentWellTool  = "record_went_well"
	wentBadlyTool = "record_went_badly"
	todoTool      = "record_todo"
)

const greetNudgePrompt = `[The person just opened their diary and has not spoken yet. Greet them in one short sentence that suits the time of day, and ask how their day was.]`

const finishNudgePrompt = `[The person wants to finish, but the entry still has no %s. Warmly acknowledge they are ready to go, but ask for what is missing in one short sentence. Offer a value they only have to confirm, keeping the conversation open until they reply.]`

func finishNudge(missing []string) string {
	return fmt.Sprintf(finishNudgePrompt, missing[0])
}

func companionPrompt(now time.Time) string {
	filled := map[string]string{
		"{{now}}":        now.Format("Monday, 2 January, 15:04"),
		"{{min_rating}}": strconv.Itoa(diary.MinRating),
		"{{max_rating}}": strconv.Itoa(diary.MaxRating),
	}

	prompt := diaryPrompt
	for placeholder, value := range filled {
		prompt = strings.ReplaceAll(prompt, placeholder, value)
	}
	return prompt
}

func liveConfig(now time.Time) *genai.LiveConnectConfig {
	text := func(name, desc, field, fieldDesc string) *genai.FunctionDeclaration {
		return &genai.FunctionDeclaration{
			Name:        name,
			Description: desc,
			Behavior:    genai.BehaviorBlocking,
			Parameters: &genai.Schema{
				Type:       genai.TypeObject,
				Properties: map[string]*genai.Schema{field: {Type: genai.TypeString, Description: fieldDesc}},
				Required:   []string{field},
			},
		}
	}

	toolFuncDeclaration := []*genai.FunctionDeclaration{
		{
			Name:        finishTool,
			Description: "Ends the diary conversation. Call this only when the person has said they are finished or has said goodbye.",
			Behavior:    genai.BehaviorBlocking,
		},
		{
			Name:        ratingTool,
			Description: "Record the number the person gave their day — only a value they stated or explicitly confirmed, never one you inferred. Call again with the new number if they correct it.",
			Behavior:    genai.BehaviorBlocking,
			Parameters: &genai.Schema{
				Type: genai.TypeObject,
				Properties: map[string]*genai.Schema{
					"rating": {
						Type:        genai.TypeInteger,
						Description: fmt.Sprintf("%d to %d", diary.MinRating, diary.MaxRating),
						Minimum:     new(float64(diary.MinRating)),
						Maximum:     new(float64(diary.MaxRating)),
					},
				},
				Required: []string{"rating"},
			},
		},
		text(emotionTool, "Record how the day felt, in a word or two — only a word they said or explicitly confirmed, never one you inferred. Call again if they correct it.", "emotion", "one or two words, in their own words"),
		text(wentWellTool, "Record one thing that went well. One call per item.", "item", "short, in their own words"),
		text(wentBadlyTool, "Record one thing that went badly. One call per item.", "item", "short, in their own words"),
		text(todoTool, "Record one thing they intend to do. One call per item.", "item", "short, in their own words"),
	}

	realtimeInputConfig := &genai.RealtimeInputConfig{
		AutomaticActivityDetection: &genai.AutomaticActivityDetection{
			StartOfSpeechSensitivity: genai.StartSensitivityHigh,
			EndOfSpeechSensitivity:   genai.EndSensitivityLow,
			SilenceDurationMs:        new(int32(1800)),
		},
	}

	contextWindowCompressionConfig := &genai.ContextWindowCompressionConfig{
		SlidingWindow: &genai.SlidingWindow{},
	}

	return &genai.LiveConnectConfig{
		ResponseModalities:       []genai.Modality{genai.ModalityAudio},
		SystemInstruction:        genai.NewContentFromText(companionPrompt(now), genai.RoleUser),
		InputAudioTranscription:  &genai.AudioTranscriptionConfig{LanguageCodes: []string{languageCode}},
		OutputAudioTranscription: &genai.AudioTranscriptionConfig{LanguageCodes: []string{languageCode}},
		SpeechConfig:             &genai.SpeechConfig{LanguageCode: languageCode},
		RealtimeInputConfig:      realtimeInputConfig,
		ContextWindowCompression: contextWindowCompressionConfig,
		Tools:                    []*genai.Tool{{FunctionDeclarations: toolFuncDeclaration}},
	}
}
