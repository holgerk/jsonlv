package main

import (
	"os"
	"path/filepath"
)

// configDirOverride is empty in production; tests set it to t.TempDir().
var configDirOverride string

func configDir() string {
	if configDirOverride != "" {
		return configDirOverride
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "jsonlv")
}
