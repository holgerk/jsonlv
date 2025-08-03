package main

import (
	"slices"
	"strings"
)

type SearchPayload struct {
	SearchTerm string        `json:"searchTerm"`
	Filters    SearchFilters `json:"filters"`
	Regexp     bool          `json:"regexp"`
}

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
	return ls.logMatchesFilter(raw, payload.Filters) && ls.logMatchesSearch(raw, payload.SearchTerm)
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
func (ls *LogSearch) logMatchesSearch(raw JsonObject, searchTerm string) bool {
	if searchTerm == "" {
		return true
	}
	searchTerm = strings.ToLower(searchTerm)

	// Search in flattened structure
	flat := flattenMap(raw)

	for _, propValue := range flat {
		if strings.Contains(strings.ToLower(propValue), searchTerm) {
			return true
		}
	}
	return false
}
