package main

import (
	"encoding/json"
	"testing"
)

func TestFlattenMap(t *testing.T) {
	log := map[string]any{
		"level": "INFO",
		"context": map[string]any{
			"requestId": "123",
		},
	}
	flat := make(map[string]any)
	flattenMap(&log, flat, "")

	if flat["context.requestId"] != "123" {
		t.Errorf("Expected %v but got %v", "123", flat["context.requestId"])
	}
	if flat["level"] != "INFO" {
		t.Errorf("Expected %v but got %v", "INFO", flat["level"])
	}
}

func TestIndexCreation(t *testing.T) {
	// Use custom config with lower maxIndexValues for easier testing
	config := LogManagerConfig{
		MaxIndexValues:        2,
		MaxLogs:               10000,
		MaxIndexValueLength:   50,
		UpdateIndexCallback:   func(indexCounts map[string]map[string]int) {},
		DropIndexKeysCallback: func(droppedKeys []string) {},
	}
	lm := NewLogManager(config)

	entries := []string{
		`{"level": "INFO", "user": "alice"}`,
		`{"level": "ERROR", "user": "alice"}`,
		`{"level": "DEBUG", "user": "carol"}`,
	}

	for _, entry := range entries {
		var raw map[string]any
		if err := json.Unmarshal([]byte(entry), &raw); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}
		lm.AddLogEntry(raw)
	}

	indexCounts := lm.GetIndexCounts()

	// 'level' should be blacklisted (3 unique values > maxIndexValues=2)
	if _, ok := indexCounts["level"]; ok {
		t.Errorf("Expected 'level' to be blacklisted")
	}
	if !lm.IsBlacklisted("level") {
		t.Errorf("Expected 'level' to be blacklisted")
	}

	// 'user' should be in index with 2 unique values (not blacklisted)
	if len(indexCounts["user"]) != 2 {
		t.Errorf("Expected 2 unique users in index, got %d", len(indexCounts["user"]))
	}
}

func TestSetFilterMessage(t *testing.T) {
	// Create LogManager instance
	lm := NewLogManager(DefaultLogManagerConfig())

	// Add some test logs
	testLogs := []string{
		`{"level": "INFO", "message": "test message 1", "user": "alice"}`,
		`{"level": "ERROR", "message": "test message 2", "user": "bob"}`,
		`{"level": "WARN", "message": "test message 3", "user": "alice"}`,
		`{"level": "INFO", "message": "another message", "user": "charlie"}`,
	}

	for _, logStr := range testLogs {
		var raw map[string]any
		if err := json.Unmarshal([]byte(logStr), &raw); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}
		lm.AddLogEntry(raw)
	}

	// Test 1: Filter by level
	t.Run("Filter by level", func(t *testing.T) {
		payload := SearchPayload{
			Filters: map[string][]string{
				"level": {"INFO"},
			},
		}
		result := lm.SearchLogs(payload, 1000)
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
		payload := SearchPayload{
			Filters: map[string][]string{
				"user": {"alice"},
			},
		}
		result := lm.SearchLogs(payload, 1000)
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
		payload := SearchPayload{
			Filters: map[string][]string{
				"level": {"INFO"},
				"user":  {"alice"},
			},
		}
		result := lm.SearchLogs(payload, 1000)
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
		payload := SearchPayload{
			Filters: map[string][]string{
				"level": {"INFO", "ERROR"},
			},
		}
		result := lm.SearchLogs(payload, 1000)
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
		payload := SearchPayload{
			SearchTerm: "message",
		}
		result := lm.SearchLogs(payload, 1000)
		if len(result) != 4 {
			t.Errorf("Expected 4 logs containing 'message', got %d", len(result))
		}
	})

	// Test 6: Search term with filter
	t.Run("Search term with filter", func(t *testing.T) {
		payload := SearchPayload{
			SearchTerm: "message",
			Filters: map[string][]string{
				"level": {"INFO"},
			},
		}
		result := lm.SearchLogs(payload, 1000)
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
		payload := SearchPayload{}
		result := lm.SearchLogs(payload, 1000)
		if len(result) != 4 {
			t.Errorf("Expected 4 logs with no filters, got %d", len(result))
		}
	})

	// Test 8: Empty search term (should return all logs)
	t.Run("Empty search term", func(t *testing.T) {
		payload := SearchPayload{
			SearchTerm: "",
		}
		result := lm.SearchLogs(payload, 1000)
		if len(result) != 4 {
			t.Errorf("Expected 4 logs with empty search term, got %d", len(result))
		}
	})

	// Test 9: Non-existent filter (should return no logs)
	t.Run("Non-existent filter", func(t *testing.T) {
		payload := SearchPayload{
			Filters: map[string][]string{
				"level": {"NONEXISTENT"},
			},
		}
		result := lm.SearchLogs(payload, 1000)
		if len(result) != 0 {
			t.Errorf("Expected 0 logs with non-existent level, got %d", len(result))
		}
	})

	// Test 10: Non-existent search term (should return no logs)
	t.Run("Non-existent search term", func(t *testing.T) {
		payload := SearchPayload{
			SearchTerm: "nonexistent",
		}
		result := lm.SearchLogs(payload, 1000)
		if len(result) != 0 {
			t.Errorf("Expected 0 logs with non-existent search term, got %d", len(result))
		}
	})
}

func TestLogMatchesFilter(t *testing.T) {
	lm := NewLogManager(DefaultLogManagerConfig())

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
			payload := SearchPayload{Filters: tt.filter}
			result := lm.logMatches(&log, &payload)
			if result != tt.expected {
				t.Errorf("%v - Expected %v, got %v", tt.name, tt.expected, result)
			}
		})
	}
}

func TestLogMatchesSearch(t *testing.T) {
	lm := NewLogManager(DefaultLogManagerConfig())

	// Test log
	log := map[string]any{
		"level":   "INFO",
		"message": "test message",
		"user":    "alice",
		"count":   456,
		"context": map[string]any{
			"requestId": "123",
			"count":     789,
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
		{
			name:       "Numeric match",
			searchTerm: "456",
			expected:   true,
		},
		{
			name:       "Numeric partial match",
			searchTerm: "45",
			expected:   true,
		},
		{
			name:       "Nested numeric match",
			searchTerm: "789",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := SearchPayload{SearchTerm: tt.searchTerm}
			result := lm.logMatches(&log, &payload)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestCallbacks(t *testing.T) {
	var updateIndexCalled bool
	var dropIndexCalled bool
	var receivedIndexCounts map[string]map[string]int
	var receivedDroppedKeys []string

	config := LogManagerConfig{
		MaxIndexValues:      2, // Low value to trigger blacklisting
		MaxLogs:             10000,
		MaxIndexValueLength: 50,
		UpdateIndexCallback: func(indexCounts map[string]map[string]int) {
			updateIndexCalled = true
			receivedIndexCounts = indexCounts
		},
		DropIndexKeysCallback: func(droppedKeys []string) {
			dropIndexCalled = true
			receivedDroppedKeys = droppedKeys
		},
	}
	lm := NewLogManager(config)

	// Add entries to trigger drop index callback
	entries := []string{
		`{"level": "INFO", "user": "alice"}`,
		`{"level": "ERROR", "user": "bob"}`,
		`{"level": "DEBUG", "user": "carol"}`, // This should trigger blacklisting
	}

	for _, entry := range entries {
		var raw map[string]any
		if err := json.Unmarshal([]byte(entry), &raw); err != nil {
			t.Fatalf("Failed to parse JSON: %v", err)
		}
		lm.AddLogEntry(raw)
	}

	// Verify drop index callback was called
	if !dropIndexCalled {
		t.Errorf("Expected DropIndexCallback to be called")
	}
	if len(receivedDroppedKeys) != 1 {
		t.Errorf("Expected 1 dropped key, got %v", receivedDroppedKeys)
	}
	// Either 'level' or 'user' could be blacklisted since both have 3 unique values
	droppedKey := receivedDroppedKeys[0]
	if droppedKey != "level" && droppedKey != "user" {
		t.Errorf("Expected dropped key to be 'level' or 'user', got %v", droppedKey)
	}

	// Test update index callback by forcing log removal
	// Add many logs to trigger enforceMaxLogs
	config2 := LogManagerConfig{
		MaxIndexValues:      10,
		MaxLogs:             2, // Very low to trigger log removal quickly
		MaxIndexValueLength: 50,
		UpdateIndexCallback: func(indexCounts map[string]map[string]int) {
			updateIndexCalled = true
			receivedIndexCounts = indexCounts
		},
		DropIndexKeysCallback: func(droppedKeys []string) {},
	}
	lm2 := NewLogManager(config2)

	// Add 3 logs to trigger removal of oldest
	for i := range 3 {
		raw := map[string]any{"id": i}
		lm2.AddLogEntry(raw)
	}

	// Verify update index callback was called
	if !updateIndexCalled {
		t.Errorf("Expected UpdateIndexCallback to be called")
	}
	if receivedIndexCounts == nil {
		t.Errorf("Expected to receive index counts")
	}
}

func TestGetLastLogs(t *testing.T) {
	lm := NewLogManager(DefaultLogManagerConfig())

	// Add some test logs
	testLogs := []map[string]any{
		{"id": 1, "message": "first"},
		{"id": 2, "message": "second"},
		{"id": 3, "message": "third"},
		{"id": 4, "message": "fourth"},
		{"id": 5, "message": "fifth"},
	}

	for _, log := range testLogs {
		lm.AddLogEntry(log)
	}

	// Test getting last 3 logs
	result := lm.GetLastLogs(3)
	if len(result) != 3 {
		t.Errorf("Expected 3 logs, got %d", len(result))
	}

	// Should get logs 3, 4, 5 in that order
	expectedIds := []int{3, 4, 5}
	for i, log := range result {
		if log["id"] != expectedIds[i] {
			t.Errorf("Expected id %v at position %d, got %v", expectedIds[i], i, log["id"])
		}
	}

	// Test getting more logs than available
	result = lm.GetLastLogs(10)
	if len(result) != 5 {
		t.Errorf("Expected 5 logs (all available), got %d", len(result))
	}
}

func TestGetLogsCount(t *testing.T) {
	lm := NewLogManager(DefaultLogManagerConfig())

	// Initially should be 0
	if lm.GetLogsCount() != 0 {
		t.Errorf("Expected 0 logs initially, got %d", lm.GetLogsCount())
	}

	// Add some logs
	for i := range 5 {
		lm.AddLogEntry(map[string]any{"id": i})
	}

	if lm.GetLogsCount() != 5 {
		t.Errorf("Expected 5 logs, got %d", lm.GetLogsCount())
	}
}

func TestGetAndClearBufferedLogs(t *testing.T) {
	lm := NewLogManager(DefaultLogManagerConfig())

	// Initially should have no logs
	result := lm.GetAndClearBufferedLogs()
	if result.HasLogs {
		t.Errorf("Expected no buffered logs initially")
	}

	// Add some logs
	testLogs := []map[string]any{
		{"id": 1, "message": "first"},
		{"id": 2, "message": "second"},
	}

	for _, log := range testLogs {
		lm.AddLogEntry(log)
	}

	// Should now have buffered logs
	result = lm.GetAndClearBufferedLogs()
	if !result.HasLogs {
		t.Errorf("Expected to have buffered logs")
	}
	if len(result.Logs) != 2 {
		t.Errorf("Expected 2 buffered logs, got %d", len(result.Logs))
	}

	// Buffer should be cleared after getting
	result = lm.GetAndClearBufferedLogs()
	if result.HasLogs {
		t.Errorf("Expected no buffered logs after clearing")
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "String",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "Integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "Float64",
			input:    3.14,
			expected: "3.14",
		},
		{
			name:     "Scientific number int (but json.Unmarshal -> float64)",
			input:    4.44928742e+08,
			expected: "444928742",
		},
		{
			name:     "Float number",
			input:    4.44928742e+06,
			expected: "4449287.42",
		},
		{
			name:     "Boolean true",
			input:    true,
			expected: "true",
		},
		{
			name:     "Boolean false",
			input:    false,
			expected: "false",
		},
		{
			name:     "Nil",
			input:    nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
