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
		regexp     bool
		expected   bool
	}{
		{
			name:       "empty search term returns true",
			log:        JsonObject{"message": "test log"},
			searchTerm: "",
			regexp:     false,
			expected:   true,
		},
		{
			name:       "exact match in string field",
			log:        JsonObject{"message": "error occurred"},
			searchTerm: "error",
			regexp:     false,
			expected:   true,
		},
		{
			name:       "case insensitive match",
			log:        JsonObject{"level": "ERROR"},
			searchTerm: "error",
			regexp:     false,
			expected:   true,
		},
		{
			name:       "partial match",
			log:        JsonObject{"message": "authentication failed"},
			searchTerm: "auth",
			regexp:     false,
			expected:   true,
		},
		{
			name:       "no match",
			log:        JsonObject{"message": "success"},
			searchTerm: "error",
			regexp:     false,
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
			regexp:     false,
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
			regexp:     false,
			expected:   true,
		},
		{
			name: "match numeric value as string",
			log: JsonObject{
				"status": 404,
				"count":  42,
			},
			searchTerm: "404",
			regexp:     false,
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
			regexp:     false,
			expected:   false,
		},
		{
			name: "match with whitespace",
			log: JsonObject{
				"message": "user login successful",
			},
			searchTerm: "login",
			regexp:     false,
			expected:   true,
		},
		{
			name:       "regexp pattern match",
			log:        JsonObject{"message": "error 404 occurred"},
			searchTerm: "error \\d+",
			regexp:     true,
			expected:   true,
		},
		{
			name:       "regexp pattern no match",
			log:        JsonObject{"message": "error abc occurred"},
			searchTerm: "error \\d+",
			regexp:     true,
			expected:   false,
		},
		{
			name:       "invalid regexp falls back to string search",
			log:        JsonObject{"message": "error [occurred"},
			searchTerm: "error [",
			regexp:     true,
			expected:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ls.logMatchesSearch(tt.log, tt.searchTerm, tt.regexp)
			if result != tt.expected {
				t.Errorf("logMatchesSearch() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

