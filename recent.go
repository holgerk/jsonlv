package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const maxRecent = 10

// menuFileCh carries actions from native menu callbacks to the main goroutine.
// Values: "open" = show file picker, "clear" = clear recent list, anything
// else is treated as a file path to tail directly.
var menuFileCh = make(chan string, 10)

func recentFilePath() string {
	return filepath.Join(configDir(), "recent.json")
}

func loadRecent() []string {
	data, err := os.ReadFile(recentFilePath())
	if err != nil {
		return nil
	}
	var files []string
	json.Unmarshal(data, &files) //nolint:errcheck
	return files
}

func addRecent(paths []string) []string {
	cur := loadRecent()
	for i := len(paths) - 1; i >= 0; i-- {
		p := paths[i]
		j := 0
		for _, c := range cur {
			if c != p {
				cur[j] = c
				j++
			}
		}
		cur = append([]string{p}, cur[:j]...)
	}
	if len(cur) > maxRecent {
		cur = cur[:maxRecent]
	}
	f := recentFilePath()
	os.MkdirAll(filepath.Dir(f), 0o755) //nolint:errcheck
	data, _ := json.MarshalIndent(cur, "", "  ")
	os.WriteFile(f, data, 0o644) //nolint:errcheck
	return cur
}

func clearRecent() {
	os.WriteFile(recentFilePath(), []byte("[]"), 0o644) //nolint:errcheck
}
