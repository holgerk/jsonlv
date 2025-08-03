package main

import (
	"testing"
)

func TestLogMatchesSearch(t *testing.T) {
	ls := &LogSearch{}

	tests := []struct {
		name       string
		log        JsonObject
		searchTerm string
		expected   bool
	}{
		{
			name:       "empty search term returns true",
			log:        JsonObject{"message": "test log"},
			searchTerm: "",
			expected:   true,
		},
		{
			name:       "exact match in string field",
			log:        JsonObject{"message": "error occurred"},
			searchTerm: "error",
			expected:   true,
		},
		{
			name:       "case insensitive match",
			log:        JsonObject{"level": "ERROR"},
			searchTerm: "error",
			expected:   true,
		},
		{
			name:       "partial match",
			log:        JsonObject{"message": "authentication failed"},
			searchTerm: "auth",
			expected:   true,
		},
		{
			name:       "no match",
			log:        JsonObject{"message": "success"},
			searchTerm: "error",
			expected:   false,
		},
		{
			name: "match in nested object",
			log: JsonObject{
				"request": map[string]any{
					"method": "POST",
					"url":    "/api/users",
				},
			},
			searchTerm: "POST",
			expected:   true,
		},
		{
			name: "match in deeply nested object",
			log: JsonObject{
				"metadata": map[string]any{
					"user": map[string]any{
						"name": "john",
						"id":   123,
					},
				},
			},
			searchTerm: "john",
			expected:   true,
		},
		{
			name: "match numeric value as string",
			log: JsonObject{
				"status": 404,
				"count":  42,
			},
			searchTerm: "404",
			expected:   true,
		},
		{
			name: "no match in multiple fields",
			log: JsonObject{
				"level":   "INFO",
				"message": "request processed",
				"status":  200,
			},
			searchTerm: "error",
			expected:   false,
		},
		{
			name: "match with whitespace",
			log: JsonObject{
				"message": "user login successful",
			},
			searchTerm: "login",
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ls.logMatchesSearch(tt.log, tt.searchTerm)
			if result != tt.expected {
				t.Errorf("logMatchesSearch() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestLogMatchesSearchWithRegexp(t *testing.T) {
	ls := &LogSearch{}

	tests := []struct {
		name       string
		log        JsonObject
		payload    SearchPayload
		expected   bool
	}{
		{
			name: "regexp pattern should match",
			log: JsonObject{
				"message": "error 404 occurred",
			},
			payload: SearchPayload{
				SearchTerm: "error \\d+",
				Regexp:     true,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ls.logMatches(tt.log, tt.payload)
			if result != tt.expected {
				t.Errorf("logMatches() with regexp = %v, expected %v", result, tt.expected)
			}
		})
	}
}
