package config

import (
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"go-simpler.org/env"
)

type Config struct {
	Port     string        `env:"PORT" default:"8080"`
	LogLevel zerolog.Level `env:"LOG_LEVEL" default:"info"`

	GeminiAPIKey    string `env:"GEMINI_API_KEY,required"`
	GeminiLiveModel string `env:"GEMINI_LIVE_MODEL" default:"gemini-3.1-flash-live-preview"`

	NotionToken      string `env:"NOTION_TOKEN,required"`
	NotionDatabaseID string `env:"NOTION_DATABASE_ID,required"`
}

func Load() (Config, error) {
	_ = godotenv.Load()

	var cfg Config
	if err := env.Load(&cfg, nil); err != nil {
		return Config{}, err
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	required := []struct{ name, value string }{
		{"GEMINI_API_KEY", c.GeminiAPIKey},
		{"NOTION_TOKEN", c.NotionToken},
		{"NOTION_DATABASE_ID", c.NotionDatabaseID},
	}

	var missing []string
	for _, v := range required {
		if strings.TrimSpace(v.value) == "" {
			missing = append(missing, v.name)
		}
	}

	if len(missing) > 0 {
		joined := strings.Join(missing, ", ")
		return fmt.Errorf("missing required environment variables: %s", joined)
	}

	return nil
}
