package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)


type windowFrame struct {
	X, Y, W, H float64
}

type appPrefs struct {
	Theme        string             `json:"theme,omitempty"`
	Window       windowFrame        `json:"window,omitempty"`
	ColumnWidths map[string]float64 `json:"columnWidths,omitempty"`
}

var (
	prefsMu  sync.Mutex
	curPrefs appPrefs
)

func prefsFilePath() string {
	return filepath.Join(configDir(), "prefs.json")
}

func loadPrefs() appPrefs {
	data, err := os.ReadFile(prefsFilePath())
	if err != nil {
		return appPrefs{Theme: "light"}
	}
	var p appPrefs
	if json.Unmarshal(data, &p) == nil {
		if p.Theme == "" {
			p.Theme = "light"
		}
		return p
	}
	return appPrefs{Theme: "light"}
}

func savePrefs() {
	prefsMu.Lock()
	p := curPrefs
	prefsMu.Unlock()
	f := prefsFilePath()
	os.MkdirAll(filepath.Dir(f), 0o755) //nolint:errcheck
	data, _ := json.MarshalIndent(p, "", "  ")
	os.WriteFile(f, data, 0o644) //nolint:errcheck
}

func setThemePref(theme string) {
	prefsMu.Lock()
	curPrefs.Theme = theme
	prefsMu.Unlock()
	savePrefs()
}

func setWindowPref(x, y, w, h float64) {
	prefsMu.Lock()
	curPrefs.Window = windowFrame{x, y, w, h}
	prefsMu.Unlock()
	savePrefs()
}
