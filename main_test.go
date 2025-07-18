package main

import (
	"encoding/json"
	"testing"
)

func TestIndexCreation(t *testing.T) {
	// Reset global state
	index = make(map[string]map[string][]string)
	blacklist = make(map[string]bool)
	maxIndexValues = 2 // lower for test

	entries := []string{
		`{"level": "INFO", "user": "alice"}`,
		`{"level": "ERROR", "user": "alice"}`,
		`{"level": "DEBUG", "user": "carol"}`,
	}

	for _, line := range entries {
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}
		uuid := line // use line as fake uuid for test
		flat := make(map[string]interface{})
		flattenMap(raw, "", flat)
		updateIndex(uuid, flat)
	}

	// Check index and blacklist
	if _, ok := index["level"]; ok {
		t.Errorf("Expected 'level' to be blacklisted and removed from index")
	}
	if !blacklist["level"] {
		t.Errorf("Expected 'level' to be blacklisted")
	}
	if _, ok := index["user"]; !ok {
		t.Errorf("Expected 'user' to be in index")
	}
	if len(index["user"]) != 2 {
		t.Errorf("Expected 2 unique users in index, got %d", len(index["user"]))
	}
}
