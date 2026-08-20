package gemini

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/genai"

	"github.com/pscheid92/voiceline-diary/internal/diary"
)

var ErrGemini = errors.New("gemini")

func connect(ctx context.Context, apiKey string) (*genai.Client, error) {
	api, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		return nil, fmt.Errorf("%w: %w: %v", ErrGemini, diary.ErrProvider, err)
	}
	return api, nil
}
