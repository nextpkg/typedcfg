package typedcfg

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigSourcePriority(t *testing.T) {
	// Test that environment variables override config file values
	type PriorityConfig struct {
		Server struct {
			Host string `json:"host" yaml:"host"`
			Port int    `json:"port" yaml:"port"`
		} `json:"server" yaml:"server"`
	}

	// Create a temporary config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "test_config.yaml")
	configContent := `server:
  host: "filehost"
  port: 8080
`
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	require.NoError(t, err)

	// Set environment variables that should override file values
	t.Setenv("TEST_SERVER__HOST", "envhost")
	t.Setenv("TEST_SERVER__PORT", "9090")

	// Test 1: File only (no env override)
	cm1, err := NewBuilder[PriorityConfig]().
		AddFile(configFile).
		Build(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm1.Close()) })

	cfg1 := cm1.Get()
	assert.Equal(t, "filehost", cfg1.Server.Host)
	assert.Equal(t, 8080, cfg1.Server.Port)

	// Test 2: File + Env (env should override)
	cm2, err := NewBuilder[PriorityConfig]().
		AddFile(configFile). // Lower priority
		AddEnv("TEST_").     // Higher priority
		Build(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm2.Close()) })

	cfg2 := cm2.Get()
	assert.Equal(t, "envhost", cfg2.Server.Host) // Should be overridden by env
	assert.Equal(t, 9090, cfg2.Server.Port)      // Should be overridden by env

	// Test 3: Env + File (file should override env)
	cm3, err := NewBuilder[PriorityConfig]().
		AddEnv("TEST_").     // Lower priority (added first)
		AddFile(configFile). // Higher priority (added last)
		Build(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cm3.Close()) })

	cfg3 := cm3.Get()
	assert.Equal(t, "filehost", cfg3.Server.Host) // Should be overridden by file
	assert.Equal(t, 8080, cfg3.Server.Port)       // Should be overridden by file
}
