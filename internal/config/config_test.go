package config

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func credentials(t *testing.T) {
	t.Helper()
	t.Setenv("GEMINI_API_KEY", "key")
	t.Setenv("NOTION_TOKEN", "token")
	t.Setenv("NOTION_DATABASE_ID", "db")
}

func TestLoad_FillsInWhatWasNotSet(t *testing.T) {
	credentials(t)

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "8080", cfg.Port)
	assert.NotEmpty(t, cfg.GeminiLiveModel, "a model default is what makes the three credentials enough")
	assert.Equal(t, zerolog.InfoLevel, cfg.LogLevel)
}

func TestLoad_TakesWhatTheEnvironmentSets(t *testing.T) {
	credentials(t)
	t.Setenv("PORT", "9000")
	t.Setenv("GEMINI_LIVE_MODEL", "some-model")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, "9000", cfg.Port)
	assert.Equal(t, "some-model", cfg.GeminiLiveModel)
	assert.Equal(t, zerolog.DebugLevel, cfg.LogLevel)
}

func TestLoad_RefusesAValueItCannotUse(t *testing.T) {
	credentials(t)
	t.Setenv("LOG_LEVEL", "chatty")

	_, err := Load()

	assert.Error(t, err, "an unknown log level must not start the process")
}

func TestLoad_RejectsPresentButEmptyCredentials(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("NOTION_TOKEN", "")
	t.Setenv("NOTION_DATABASE_ID", "")

	_, err := Load()

	assert.Error(t, err, "empty credentials must not start the process")
}
