package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// ============================================================================
// Type Definitions
// ============================================================================

type LogId = uint
type PropName = string
type PropValue = string
type JsonObject = map[string]any
type FlatJsonObject = map[string]string
type IndexCounts = map[PropName]map[PropValue]uint
type SearchFilters = map[PropName][]PropValue

type LogRecord struct {
	id  LogId
	Raw JsonObject
}

type BufferedLogsResult struct {
	Logs    []JsonObject
	HasLogs bool
}

type SearchPayload struct {
	SearchTerm string        `json:"searchTerm"`
	Filters    SearchFilters `json:"filters"`
}

type SearchLogsResult struct {
	Logs        []JsonObject
	IndexCounts IndexCounts
}

type LogManagerConfig struct {
	MaxIndexValues        int
	MaxLogs               int
	MaxIndexValueLength   int
	DropIndexKeysCallback func(droppedKeys []PropName)
}

func DefaultLogManagerConfig() LogManagerConfig {
	return LogManagerConfig{
		MaxIndexValues:        10,
		MaxLogs:               10000,
		MaxIndexValueLength:   50,
		DropIndexKeysCallback: func(droppedKeys []PropName) {}, // no-op default
	}
}

type LogManager struct {
	// Storage
	logOrder   []LogId
	logStore   map[LogId]LogRecord
	logStoreMu sync.RWMutex

	// Buffering
	logBuffer   []JsonObject
	logBufferMu sync.RWMutex

	// Indexing
	index     map[PropName]map[PropValue][]LogId
	blacklist map[PropName]bool

	// Configuration
	config LogManagerConfig

	// ID generation
	idCounter   uint
	idCounterMu sync.Mutex
}

// ============================================================================
// LogManager Constructor and Methods
// ============================================================================

func NewLogManager(config LogManagerConfig) *LogManager {
	return &LogManager{
		logOrder:  []LogId{},
		logStore:  make(map[LogId]LogRecord),
		logBuffer: []JsonObject{},
		index:     make(map[PropName]map[PropValue][]uint),
		blacklist: make(map[PropName]bool),
		config:    config,
		idCounter: 0,
	}
}

func (lm *LogManager) AddLogEntry(raw JsonObject) uint {
	lm.idCounterMu.Lock()
	lm.idCounter++
	id := lm.idCounter
	lm.idCounterMu.Unlock()
	flat := flattenMap(raw)

	lm.logStoreMu.Lock()
	lm.logStore[id] = LogRecord{
		id:  id,
		Raw: raw,
	}
	lm.logOrder = append(lm.logOrder, id)
	lm.addToIndex(id, flat)
	lm.enforceMaxLogs()
	lm.logStoreMu.Unlock()

	lm.logBufferMu.Lock()
	lm.logBuffer = append(lm.logBuffer, raw)
	lm.logBufferMu.Unlock()

	return id
}

func (lm *LogManager) GetAndClearBufferedLogs() BufferedLogsResult {
	lm.logBufferMu.Lock()
	defer lm.logBufferMu.Unlock()

	if len(lm.logBuffer) == 0 {
		return BufferedLogsResult{HasLogs: false}
	}

	result := BufferedLogsResult{
		Logs:    make([]JsonObject, len(lm.logBuffer)),
		HasLogs: true,
	}
	copy(result.Logs, lm.logBuffer)
	lm.logBuffer = lm.logBuffer[:0]

	return result
}

// GetLastLogs returns the last n log entries
func (lm *LogManager) GetLastLogs(n int) []JsonObject {
	lm.logStoreMu.RLock()
	defer lm.logStoreMu.RUnlock()

	res := []JsonObject{}
	start := 0
	if len(lm.logOrder) > n {
		start = len(lm.logOrder) - n
	}
	for _, uuid := range lm.logOrder[start:] {
		if entry, ok := lm.logStore[uuid]; ok {
			res = append(res, entry.Raw)
		}
	}
	return res
}

// SearchLogs returns filtered logs based on filters and search term
func (lm *LogManager) SearchLogs(searchPayload SearchPayload, maxLogs int) SearchLogsResult {
	lm.logStoreMu.RLock()
	defer lm.logStoreMu.RUnlock()

	result := []JsonObject{}
	count := 0

	globalCounts := lm.GetIndexCounts()
	newCounts := make(IndexCounts)
	for propName, propValues := range globalCounts {
		if _, ok := newCounts[propName]; !ok {
			newCounts[propName] = make(map[PropValue]uint)
		}
		for propValue, propValueCount := range propValues {
			if _, ok := searchPayload.Filters[propName]; ok {
				if slices.Contains(searchPayload.Filters[propName], propValue) {
					newCounts[propName][propValue] = 0
				} else {
					newCounts[propName][propValue] = propValueCount
				}
			} else {
				newCounts[propName][propValue] = 0
			}
		}
	}

	// Start from the end (most recent logs)
	for i := len(lm.logOrder) - 1; i >= 0; i-- {
		entryId := lm.logOrder[i]
		if entry, ok := lm.logStore[entryId]; ok {
			if lm.logMatches(entry.Raw, &searchPayload) {
				if count < maxLogs {
					result = append([]JsonObject{entry.Raw}, result...)
				}
				count++

				flat := flattenMap(entry.Raw)
				for propName, propValue := range flat {
					if lm.omitIndexValue(propName, propValue) {
						continue
					}
					if _, ok := newCounts[propName]; !ok {
						newCounts[propName] = make(map[PropValue]uint)
					}
					newCounts[propName][propValue]++
				}
			}
		}
	}

	return SearchLogsResult{
		Logs:        result,
		IndexCounts: newCounts,
	}
}

func (lm *LogManager) FilterLogs(logs []JsonObject, payload SearchPayload) []JsonObject {
	filteredLogs := []JsonObject{}
	for _, log := range logs {
		if lm.logMatches(log, &payload) {
			filteredLogs = append(filteredLogs, log)
		}
	}
	return filteredLogs
}

func (lm *LogManager) logMatches(raw JsonObject, payload *SearchPayload) bool {
	return lm.logMatchesFilter(raw, payload.Filters) && lm.logMatchesSearch(raw, payload.SearchTerm)
}

// logMatchesFilter checks if a log entry matches the given filters
func (lm *LogManager) logMatchesFilter(raw JsonObject, filter map[PropName][]PropValue) bool {
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
func (lm *LogManager) logMatchesSearch(raw JsonObject, searchTerm string) bool {
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

// enforceMaxLogs enforces the maximum number of stored logs
func (lm *LogManager) enforceMaxLogs() {
	if len(lm.logOrder) > lm.config.MaxLogs {
		oldest := lm.logOrder[0]
		lm.logOrder = lm.logOrder[1:]
		if entry, ok := lm.logStore[oldest]; ok {
			flatOld := flattenMap(entry.Raw)
			lm.removeFromIndex(oldest, flatOld)
			delete(lm.logStore, oldest)
			// tod o Notify via callback about index update
		}
	}
}

// addToIndex adds a log entry to the search index
func (lm *LogManager) addToIndex(entryId uint, flat FlatJsonObject) {
	for propName, propValue := range flat {
		if lm.omitIndexValue(propName, propValue) {
			continue
		}
		if _, ok := lm.index[propName]; !ok {
			lm.index[propName] = make(map[string][]uint)
		}
		valMap := lm.index[propName]
		valMap[propValue] = append(valMap[propValue], entryId)
		// Blacklist if too many unique values
		if len(valMap) > lm.config.MaxIndexValues {
			delete(lm.index, propName)
			lm.blacklist[propName] = true
			// Notify via callback about dropped index
			if lm.config.DropIndexKeysCallback != nil {
				lm.config.DropIndexKeysCallback([]string{propName})
			}
		}
	}
}

// removeFromIndex removes a log entry from the search index
func (lm *LogManager) removeFromIndex(entryId uint, flat FlatJsonObject) {
	for propName, propValue := range flat {
		if len(propValue) > lm.config.MaxIndexValueLength {
			continue // omit very long values
		}
		if propValueMap, ok := lm.index[propName]; ok {
			if entryIds, ok := propValueMap[propValue]; ok {
				// Remove uint from slice
				newEntryIds := []uint{}
				for _, id := range entryIds {
					if id != entryId {
						newEntryIds = append(newEntryIds, id)
					}
				}
				if len(newEntryIds) == 0 {
					delete(propValueMap, propValue)
				} else {
					propValueMap[propValue] = newEntryIds
				}
			}
			if len(propValueMap) == 0 {
				delete(lm.index, propName)
			}
		}
	}
}

func (lm *LogManager) increaseClientIndexCounts(indexCounts IndexCounts, _ SearchFilters, logs []JsonObject) {
	for _, log := range logs {
		flat := flattenMap(log)
		for propName, propValue := range flat {
			if lm.omitIndexValue(propName, propValue) {
				continue
			}
			if _, ok := indexCounts[propName]; !ok {
				indexCounts[propName] = make(map[PropValue]uint)
			}
			indexCounts[propName][propValue]++
		}
	}
}

func (lm *LogManager) omitIndexValue(propName string, propValue string) bool {
	if propValue == "" {
		return true // omit empty values
	}
	if len(propValue) > lm.config.MaxIndexValueLength {
		return true // omit very long values
	}
	if lm.blacklist[propName] {
		return true // skip blacklisted properties
	}
	return false
}

// GetIndexCounts returns the count of entries for each indexed property value
func (lm *LogManager) GetIndexCounts() IndexCounts {
	result := make(IndexCounts)
	for propName, valMap := range lm.index {
		result[propName] = make(map[PropValue]uint)
		for v, entryIds := range valMap {
			result[propName][v] = uint(len(entryIds))
		}
	}
	return result
}

// GetLogsCount returns the number of logs currently stored
func (lm *LogManager) GetLogsCount() uint {
	lm.logStoreMu.RLock()
	defer lm.logStoreMu.RUnlock()
	return uint(len(lm.logStore))
}

// ============================================================================
// Utility Functions
// ============================================================================

// toString converts any value to its string representation
func toString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return fmt.Sprintf("%g", v)
	case int, int8, int16, int32, int64:
		return fmt.Sprintf("%d", v)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", v)
	}
}

// flattenMap flattens a nested map using dot notation
func flattenMap(data JsonObject) FlatJsonObject {
	out := make(FlatJsonObject)
	flattenMapInternal(data, out, "")
	return out
}

func flattenMapInternal(data JsonObject, out FlatJsonObject, prefix string) {
	for k, v := range data {
		key := prefix + k
		switch val := v.(type) {
		case JsonObject:
			flattenMapInternal(val, out, key+".")
		default:
			out[key] = toString(val)
		}
	}

}
