package gemini

import (
	"context"

	"google.golang.org/genai"
)

type connection interface {
	SendRealtimeInput(input genai.LiveRealtimeInput) error
	SendToolResponse(input genai.LiveToolResponseInput) error
	Receive() (*genai.LiveServerMessage, error)
	Close() error
}

type dialer func(ctx context.Context, cfg *genai.LiveConnectConfig) (connection, error)
