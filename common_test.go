package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommon(t *testing.T) {

	t.Run("Test config", func(t *testing.T) {
		defaultConfig = nil
		err := os.Symlink("test-data/valid-config.yaml", configPath)
		assert.NoError(t, err)
		readConfig()
		assert.Equal(t, defaultConfig.RunMode, runModeSync)
		assert.Equal(t, defaultConfig.LogLevel, logLevelDebug)
	})

}
