package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	mappingMu sync.RWMutex
	prefixMap = map[string]string{} // remote prefix → local prefix
)

func mappingsFile() string {
	return filepath.Join(configDir(), "mappings.json")
}

func initMappings() {
	data, err := os.ReadFile(mappingsFile())
	if err != nil {
		return
	}
	var m map[string]string
	if json.Unmarshal(data, &m) == nil {
		mappingMu.Lock()
		prefixMap = m
		mappingMu.Unlock()
	}
}

// resolveLocalPath maps a remote path to a local one using cached prefix pairs.
// Returns the local path and true, or ("", false) if no mapping is known.
func resolveLocalPath(remote string) (string, bool) {
	mappingMu.RLock()
	defer mappingMu.RUnlock()
	for rem, loc := range prefixMap {
		if strings.HasPrefix(remote, rem+"/") || remote == rem {
			return loc + remote[len(rem):], true
		}
	}
	if _, err := os.Stat(remote); err == nil {
		return remote, true // path is accessible as-is
	}
	return "", false
}

// addMapping learns a remote→local prefix pair from one concrete file example
// and persists all mappings to disk.
func addMapping(remote, local string) {
	remParts := strings.Split(strings.Trim(remote, "/"), "/")
	locParts := strings.Split(strings.Trim(local, "/"), "/")

	// Find how many trailing path components are identical.
	var common int
	for i := 1; i <= min(len(remParts), len(locParts)); i++ {
		if remParts[len(remParts)-i] == locParts[len(locParts)-i] {
			common = i
		} else {
			break
		}
	}

	var remPrefix, locPrefix string
	if common > 0 && common < len(remParts) {
		remPrefix = "/" + strings.Join(remParts[:len(remParts)-common], "/")
		locPrefix = "/" + strings.Join(locParts[:len(locParts)-common], "/")
	} else {
		// No deducible prefix — store an exact file mapping.
		remPrefix = remote
		locPrefix = local
	}

	mappingMu.Lock()
	prefixMap[remPrefix] = locPrefix
	mappingMu.Unlock()
	saveMappings()
}

func saveMappings() {
	f := mappingsFile()
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		return
	}
	mappingMu.RLock()
	data, _ := json.MarshalIndent(prefixMap, "", "  ")
	mappingMu.RUnlock()
	os.WriteFile(f, data, 0o644) //nolint:errcheck
}
