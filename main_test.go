package main

import (
	"encoding/json"
	"testing"
)

func TestFlattenMap(t *testing.T) {
	log := map[string]any{
		"level":   "INFO",
		"context": map[string]any{
			"requestId": "123",
		},
	}
	flat := make(map[string]any)
	flattenMap(log, flat, "")

	if flat["context.requestId"] != "123" {
		t.Errorf("Expected %v but got %v", "123", flat["context.requestId"])
	}
	if flat["level"] != "INFO" {
		t.Errorf("Expected %v but got %v", "INFO", flat["level"])
	}
}

func TestIndexCreation(t *testing.T) {
	index = make(map[string]map[string][]uint)
	blacklist = make(map[string]bool)
	maxIndexValues = 2 // lower for test

	entries := []string{
		`{"level": "INFO", "user": "alice"}`,
		`{"level": "ERROR", "user": "alice"}`,
		`{"level": "DEBUG", "user": "carol"}`,
	}

	for id, entry := range entries {
		var raw map[string]any
		if err := json.Unmarshal([]byte(entry), &raw); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}
		u := uint(id)
		flat := make(map[string]any)
		flattenMap(raw, flat, "")
		updateIndex(u, flat)
	}

	// 'level' should be blacklisted (3 unique values > maxIndexValues=2)
	if _, ok := index["level"]; ok {
		t.Errorf("Expected 'level' to be blacklisted")
	}
	if !blacklist["level"] {
		t.Errorf("Expected 'level' to be blacklisted")
	}

	// 'user' should be in index with 2 unique values
	if len(index["user"]) != 2 {
		t.Errorf("Expected 2 unique users in index, got %d", len(index["user"]))
	}
}

func TestSetFilterMessage(t *testing.T) {
	// Reset global state
	index = make(map[string]map[string][]uint)
	blacklist = make(map[string]bool)
	maxIndexValues = 10
	logStore = make(map[uint]LogEntry)
	logOrder = []uint{}

	// Add some test logs
	testLogs := []string{
		`{"level": "INFO", "message": "test message 1", "user": "alice"}`,
		`{"level": "ERROR", "message": "test message 2", "user": "bob"}`,
		`{"level": "WARN", "message": "test message 3", "user": "alice"}`,
		`{"level": "INFO", "message": "another message", "user": "charlie"}`,
	}

	for i, logStr := range testLogs {
		var raw map[string]any
		if err := json.Unmarshal([]byte(logStr), &raw); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}
		u := uint(i)
		flat := make(map[string]any)
		flattenMap(raw, flat, "")
		logStore[u] = LogEntry{id: u, Raw: raw}
		logOrder = append(logOrder, u)
		updateIndex(u, flat)
	}

	// Test 1: Filter by level
	t.Run("Filter by level", func(t *testing.T) {
		payload := SetFilterPayload{
			Filters: map[string][]string{
				"level": {"INFO"},
			},
		}
		result := filterLogsWithSearch(payload, 1000)
		if len(result) != 2 {
			t.Errorf("Expected 2 logs with level INFO, got %d", len(result))
		}
		for _, log := range result {
			if log["level"] != "INFO" {
				t.Errorf("Expected level INFO, got %v", log["level"])
			}
		}
	})

	// Test 2: Filter by user
	t.Run("Filter by user", func(t *testing.T) {
		payload := SetFilterPayload{
			Filters: map[string][]string{
				"user": {"alice"},
			},
		}
		result := filterLogsWithSearch(payload, 1000)
		if len(result) != 2 {
			t.Errorf("Expected 2 logs with user alice, got %d", len(result))
		}
		for _, log := range result {
			if log["user"] != "alice" {
				t.Errorf("Expected user alice, got %v", log["user"])
			}
		}
	})

	// Test 3: Multiple filters (AND logic)
	t.Run("Multiple filters", func(t *testing.T) {
		payload := SetFilterPayload{
			Filters: map[string][]string{
				"level": {"INFO"},
				"user":  {"alice"},
			},
		}
		result := filterLogsWithSearch(payload, 1000)
		if len(result) != 1 {
			t.Errorf("Expected 1 log with level INFO and user alice, got %d", len(result))
		}
		log := result[0]
		if log["level"] != "INFO" || log["user"] != "alice" {
			t.Errorf("Expected level INFO and user alice, got level=%v user=%v", log["level"], log["user"])
		}
	})

	// Test 4: Multiple values for same property (OR logic)
	t.Run("Multiple values for same property", func(t *testing.T) {
		payload := SetFilterPayload{
			Filters: map[string][]string{
				"level": {"INFO", "ERROR"},
			},
		}
		result := filterLogsWithSearch(payload, 1000)
		if len(result) != 3 {
			t.Errorf("Expected 3 logs with level INFO or ERROR, got %d", len(result))
		}
		for _, log := range result {
			level := log["level"].(string)
			if level != "INFO" && level != "ERROR" {
				t.Errorf("Expected level INFO or ERROR, got %v", level)
			}
		}
	})

	// Test 5: Search term
	t.Run("Search term", func(t *testing.T) {
		payload := SetFilterPayload{
			SearchTerm: "message",
		}
		result := filterLogsWithSearch(payload, 1000)
		if len(result) != 4 {
			t.Errorf("Expected 4 logs containing 'message', got %d", len(result))
		}
	})

	// Test 6: Search term with filter
	t.Run("Search term with filter", func(t *testing.T) {
		payload := SetFilterPayload{
			SearchTerm: "message",
			Filters: map[string][]string{
				"level": {"INFO"},
			},
		}
		result := filterLogsWithSearch(payload, 1000)
		if len(result) != 2 {
			t.Errorf("Expected 2 logs with level INFO containing 'message', got %d", len(result))
		}
		for _, log := range result {
			if log["level"] != "INFO" {
				t.Errorf("Expected level INFO, got %v", log["level"])
			}
		}
	})

	// Test 7: No filters (should return all logs)
	t.Run("No filters", func(t *testing.T) {
		payload := SetFilterPayload{}
		result := filterLogsWithSearch(payload, 1000)
		if len(result) != 4 {
			t.Errorf("Expected 4 logs with no filters, got %d", len(result))
		}
	})

	// Test 8: Empty search term (should return all logs)
	t.Run("Empty search term", func(t *testing.T) {
		payload := SetFilterPayload{
			SearchTerm: "",
		}
		result := filterLogsWithSearch(payload, 1000)
		if len(result) != 4 {
			t.Errorf("Expected 4 logs with empty search term, got %d", len(result))
		}
	})

	// Test 9: Non-existent filter (should return no logs)
	t.Run("Non-existent filter", func(t *testing.T) {
		payload := SetFilterPayload{
			Filters: map[string][]string{
				"level": {"NONEXISTENT"},
			},
		}
		result := filterLogsWithSearch(payload, 1000)
		if len(result) != 0 {
			t.Errorf("Expected 0 logs with non-existent level, got %d", len(result))
		}
	})

	// Test 10: Non-existent search term (should return no logs)
	t.Run("Non-existent search term", func(t *testing.T) {
		payload := SetFilterPayload{
			SearchTerm: "nonexistent",
		}
		result := filterLogsWithSearch(payload, 1000)
		if len(result) != 0 {
			t.Errorf("Expected 0 logs with non-existent search term, got %d", len(result))
		}
	})
}

func TestLogMatchesFilter(t *testing.T) {
	// Test log
	log := map[string]any{
		"level":   "INFO",
		"message": "test message",
		"user":    "alice",
		"context": map[string]any{
			"requestId": "123",
		},
	}

	tests := []struct {
		name     string
		filter   map[string][]string
		expected bool
	}{
		{
			name:     "Match nested property",
			filter:   map[string][]string{"context.requestId": {"123"}},
			expected: true,
		},
		{
			name:     "No filter",
			filter:   map[string][]string{},
			expected: true,
		},
		{
			name:     "Match single value",
			filter:   map[string][]string{"level": {"INFO"}},
			expected: true,
		},
		{
			name:     "Match multiple values (OR)",
			filter:   map[string][]string{"level": {"INFO", "ERROR"}},
			expected: true,
		},
		{
			name:     "No match",
			filter:   map[string][]string{"level": {"ERROR"}},
			expected: false,
		},
		{
			name:     "Multiple properties (AND)",
			filter:   map[string][]string{"level": {"INFO"}, "user": {"alice"}},
			expected: true,
		},
		{
			name:     "Multiple properties no match",
			filter:   map[string][]string{"level": {"INFO"}, "user": {"bob"}},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logMatchesFilter(log, tt.filter)
			if result != tt.expected {
				t.Errorf("%v - Expected %v, got %v", tt.name, tt.expected, result)
			}
		})
	}
}

func TestLogMatchesSearch(t *testing.T) {
	// Test log
	log := map[string]any{
		"level":   "INFO",
		"message": "test message",
		"user":    "alice",
		"context": map[string]any{
			"requestId": "123",
		},
	}

	tests := []struct {
		name       string
		searchTerm string
		expected   bool
	}{
		{
			name:       "Empty search term",
			searchTerm: "",
			expected:   true,
		},
		{
			name:       "Match in level",
			searchTerm: "INFO",
			expected:   true,
		},
		{
			name:       "Match in message",
			searchTerm: "test",
			expected:   true,
		},
		{
			name:       "Match in nested property",
			searchTerm: "123",
			expected:   true,
		},
		{
			name:       "Case insensitive match",
			searchTerm: "info",
			expected:   true,
		},
		{
			name:       "No match",
			searchTerm: "nonexistent",
			expected:   false,
		},
		{
			name:       "Partial match",
			searchTerm: "mess",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logMatchesSearch(log, tt.searchTerm)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
