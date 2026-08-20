package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/pscheid92/voiceline-diary/internal/config"
	"github.com/pscheid92/voiceline-diary/internal/gemini"
	"github.com/pscheid92/voiceline-diary/internal/notion"
	"github.com/pscheid92/voiceline-diary/internal/router"
	"github.com/pscheid92/voiceline-diary/internal/session"
	"github.com/pscheid92/voiceline-diary/internal/wire"
)

func main() {
	if err := run(); err != nil {
		log.Fatal().Err(err).Msg("startup failed")
	}
}

func run() error {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	root := zerolog.
		New(os.Stdout).
		Level(cfg.LogLevel).
		With().
		Timestamp().
		Logger()

	log.Logger = root
	zerolog.DefaultContextLogger = &root

	companion, err := gemini.NewCompanion(ctx, cfg.GeminiAPIKey, cfg.GeminiLiveModel)
	if err != nil {
		return fmt.Errorf("hire the companion: %w", err)
	}

	diary := notion.New(cfg.NotionToken, cfg.NotionDatabaseID)

	dial := func(ctx context.Context, now time.Time) (session.Companion, error) {
		return companion.Dial(ctx, now)
	}

	engine := router.New(wire.NewHandler(diary, dial), diary.DatabaseURL())

	addr := ":" + cfg.Port
	log.Info().Str("addr", addr).Msg("listening")
	return engine.Run(addr)
}
