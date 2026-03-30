package agent_test

import (
	"testing"
	"time"

	"github.com/amer/aql/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoadModelCache(t *testing.T) {
	dir := t.TempDir()

	models := []agent.ModelInfo{
		{ID: "claude-opus-4-6", DisplayName: "Claude Opus 4.6", MaxInputTokens: 1000000},
		{ID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6", MaxInputTokens: 200000},
	}

	err := agent.SaveModelCache(dir, models)
	require.NoError(t, err)

	loaded, err := agent.LoadModelCache(dir)
	require.NoError(t, err)
	require.Len(t, loaded, 2)
	assert.Equal(t, "claude-opus-4-6", loaded[0].ID)
	assert.Equal(t, "Claude Opus 4.6", loaded[0].DisplayName)
	assert.Equal(t, int64(1000000), loaded[0].MaxInputTokens)
	assert.Equal(t, "claude-sonnet-4-6", loaded[1].ID)
}

func TestLoadModelCacheEmpty(t *testing.T) {
	dir := t.TempDir()

	loaded, err := agent.LoadModelCache(dir)
	assert.NoError(t, err)
	assert.Nil(t, loaded, "should return nil when no cache exists")
}

func TestLoadModelCacheExpired(t *testing.T) {
	dir := t.TempDir()

	models := []agent.ModelInfo{
		{ID: "claude-opus-4-6", DisplayName: "Claude Opus 4.6", MaxInputTokens: 1000000},
	}

	// Save with a custom TTL that's already expired
	err := agent.SaveModelCacheWithTTL(dir, models, -1*time.Hour)
	require.NoError(t, err)

	loaded, err := agent.LoadModelCache(dir)
	assert.NoError(t, err)
	assert.Nil(t, loaded, "expired cache should return nil")
}

func TestLoadModelCacheValid(t *testing.T) {
	dir := t.TempDir()

	models := []agent.ModelInfo{
		{ID: "claude-haiku-4-5", DisplayName: "Claude Haiku 4.5", MaxInputTokens: 200000},
	}

	err := agent.SaveModelCacheWithTTL(dir, models, 2*time.Hour)
	require.NoError(t, err)

	loaded, err := agent.LoadModelCache(dir)
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, "claude-haiku-4-5", loaded[0].ID)
}
