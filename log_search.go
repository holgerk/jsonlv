package main

import (
	"regexp"
	"slices"
	"strings"
)

type SearchPayload struct {
	SearchTerm string        `json:"searchTerm"`
	Filters    SearchFilters `json:"filters"`
	Regexp     bool          `json:"regexp"`
}

// Matches any Unicode whitespace:
// \p{Zs} -> space separators
// \p{Zl} -> line separator
// \p{Zp} -> paragraph separator
// \t\n\f\r -> ASCII whitespace controls
var unicodeWhitespace = regexp.MustCompile(`[\p{Zs}\p{Zl}\p{Zp}\t\n\f\r]+`)

type LogSearch struct{}

func (ls *LogSearch) FilterLogs(logs []JsonObject, payload SearchPayload) []JsonObject {
	filteredLogs := []JsonObject{}
	for _, log := range logs {
		if ls.logMatches(log, payload) {
			filteredLogs = append(filteredLogs, log)
		}
	}
	return filteredLogs
}

func (ls *LogSearch) logMatches(raw JsonObject, payload SearchPayload) bool {
	return ls.logMatchesFilter(raw, payload.Filters) && ls.logMatchesSearch(raw, payload.SearchTerm, payload.Regexp)
}

// logMatchesFilter checks if a log entry matches the given filters
func (ls *LogSearch) logMatchesFilter(raw JsonObject, filter map[PropName][]PropValue) bool {
	if filter == nil {
		return true
	}
	flat := flattenMap(raw)
	for propName, propValues := range filter {
		propValue := flat[propName]
		match := slices.Contains(propValues, propValue)
		if !match {
			return false
		}
	}
	return true
}

// logMatchesSearch checks if a log entry matches the search term
func (ls *LogSearch) logMatchesSearch(raw JsonObject, searchTerm string, useRegexp bool) bool {
	if searchTerm == "" {
		return true
	}

	// Search in flattened structure
	flat := flattenMap(raw)

	if useRegexp {
		// Regexp search - case insensitive
		re, err := regexp.Compile("(?i)" + searchTerm)
		if err != nil {
			// If regexp is invalid, fall back to string search
			return stringSearch(searchTerm, flat)
		}
		for _, propValue := range flat {
			if re.MatchString(propValue) {
				return true
			}
		}
		return false
	} else {
		// Regular string search
		return stringSearch(searchTerm, flat)
	}
}

func stringSearch(searchTerm string, flat FlatJsonObject) bool {
	searchTerm = strings.ToLower(searchTerm)
	searchTermChunks := splitOnWhitespace(searchTerm)
	for _, searchTermChunk := range searchTermChunks {
		found := false
		for _, propValue := range flat {
			if strings.Contains(strings.ToLower(propValue), searchTermChunk) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
		
		
	}
	return true
}

func splitOnWhitespace(s string) []string {
	parts := unicodeWhitespace.Split(s, -1)
	// Remove any empty strings
	result := parts[:0] // reuse the same slice
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
