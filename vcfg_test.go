package typedcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type VcfgTestConfig struct {
	Name    string `json:"name"`
	Port    int    `json:"port"`
	Enabled bool   `json:"enabled"`
}

func TestMustLoad(t *testing.T) {
	t.Run("successful load", func(t *testing.T) {
		// Create a temporary config file
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.json")
		configContent := `{"name":"must-load-test","port":8080,"enabled":true}`
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		assert.NotPanics(t, func() {
			cm := MustLoad[VcfgTestConfig](configFile)
			assert.NotNil(t, cm)

			// Verify config was loaded correctly
			config := cm.Get()
			assert.NotNil(t, config)
			assert.Equal(t, "must-load-test", config.Name)
			assert.Equal(t, 8080, config.Port)
			assert.True(t, config.Enabled)

			// Clean up
			cm.Close()
		})
	})

	t.Run("failed load should panic", func(t *testing.T) {
		assert.Panics(t, func() {
			MustLoad[VcfgTestConfig]("/nonexistent/config.json")
		})
	})

	t.Run("invalid JSON should panic", func(t *testing.T) {
		// Create a temporary config file with invalid JSON
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "invalid.json")
		invalidContent := `{"name":"test","port":}`
		err := os.WriteFile(configFile, []byte(invalidContent), 0644)
		require.NoError(t, err)

		assert.Panics(t, func() {
			MustLoad[VcfgTestConfig](configFile)
		})
	})
}

func TestMustBuild(t *testing.T) {
	t.Run("successful build", func(t *testing.T) {
		// Create a temporary config file
		tmpDir := t.TempDir()
		configFile := filepath.Join(tmpDir, "config.json")
		configContent := `{"name":"must-build-test","port":9090,"enabled":false}`
		err := os.WriteFile(configFile, []byte(configContent), 0644)
		require.NoError(t, err)

		assert.NotPanics(t, func() {
			cm := MustBuild[VcfgTestConfig](configFile)
			assert.NotNil(t, cm)

			// Verify config was loaded correctly
			config := cm.Get()
			assert.NotNil(t, config)
			assert.Equal(t, "must-build-test", config.Name)
			assert.Equal(t, 9090, config.Port)
			assert.False(t, config.Enabled)

			// Clean up
			cm.Close()
		})
	})

	t.Run("failed build should panic", func(t *testing.T) {
		assert.Panics(t, func() {
			MustBuild[VcfgTestConfig]("/nonexistent/config.json")
		})
	})

	t.Run("multiple sources", func(t *testing.T) {
		// Create multiple temporary config files
		tmpDir := t.TempDir()
		configFile1 := filepath.Join(tmpDir, "config1.json")
		configFile2 := filepath.Join(tmpDir, "config2.json")

		// First file with base config
		configContent1 := `{"name":"base","port":8080}`
		err := os.WriteFile(configFile1, []byte(configContent1), 0644)
		require.NoError(t, err)

		// Second file with override
		configContent2 := `{"enabled":true}`
		err = os.WriteFile(configFile2, []byte(configContent2), 0644)
		require.NoError(t, err)

		assert.NotPanics(t, func() {
			cm := MustBuild[VcfgTestConfig](configFile1, configFile2)
			assert.NotNil(t, cm)

			// Verify merged config
			config := cm.Get()
			assert.NotNil(t, config)
			assert.Equal(t, "base", config.Name)
			assert.Equal(t, 8080, config.Port)
			assert.True(t, config.Enabled)

			// Clean up
			cm.Close()
		})
	})
}

func TestMustLoad_EmptyFile(t *testing.T) {
	// Create an empty config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "empty.json")
	err := os.WriteFile(configFile, []byte(`{}`), 0644)
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		cm := MustLoad[VcfgTestConfig](configFile)
		assert.NotNil(t, cm)

		// Verify config has zero values
		config := cm.Get()
		assert.NotNil(t, config)
		assert.Equal(t, "", config.Name)
		assert.Equal(t, 0, config.Port)
		assert.False(t, config.Enabled)

		// Clean up
		cm.Close()
	})
}

func TestMustBuild_EmptyFile(t *testing.T) {
	// Create an empty config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "empty.json")
	err := os.WriteFile(configFile, []byte(`{}`), 0644)
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		cm := MustBuild[VcfgTestConfig](configFile)
		assert.NotNil(t, cm)

		// Verify config has zero values
		config := cm.Get()
		assert.NotNil(t, config)
		assert.Equal(t, "", config.Name)
		assert.Equal(t, 0, config.Port)
		assert.False(t, config.Enabled)

		// Clean up
		cm.Close()
	})
}
