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

type LogEntry struct {
	id  uint
	Raw map[string]any
}

type BufferedLogsResult struct {
	Logs        []map[string]any
	IndexCounts map[string]map[string]int
	HasLogs     bool
}

type LogManagerConfig struct {
	MaxIndexValues        int
	MaxLogs               int
	MaxIndexValueLength   int
	UpdateIndexCallback   func(indexCounts map[string]map[string]int)
	DropIndexKeysCallback func(droppedKeys []string)
}

func DefaultLogManagerConfig() LogManagerConfig {
	return LogManagerConfig{
		MaxIndexValues:        10,
		MaxLogs:               10000,
		MaxIndexValueLength:   50,
		UpdateIndexCallback:   func(indexCounts map[string]map[string]int) {}, // no-op default
		DropIndexKeysCallback: func(droppedKeys []string) {},                  // no-op default
	}
}

type LogManager struct {
	// Storage
	logOrder   []uint
	logStore   map[uint]LogEntry
	logStoreMu sync.RWMutex

	// Buffering
	logBuffer   []map[string]any
	logBufferMu sync.RWMutex

	// Indexing
	index     map[string]map[string][]uint
	blacklist map[string]bool

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
		logOrder:  []uint{},
		logStore:  make(map[uint]LogEntry),
		logBuffer: []map[string]any{},
		index:     make(map[string]map[string][]uint),
		blacklist: make(map[string]bool),
		config:    config,
		idCounter: 0,
	}
}

func (lm *LogManager) AddLogEntry(raw map[string]any) uint {
	lm.idCounterMu.Lock()
	lm.idCounter++
	id := lm.idCounter
	lm.idCounterMu.Unlock()

	flat := make(map[string]any)
	flattenMap(&raw, flat, "")

	lm.logStoreMu.Lock()
	lm.logStore[id] = LogEntry{
		id:  id,
		Raw: raw,
	}
	lm.logOrder = append(lm.logOrder, id)
	lm.updateIndex(id, flat)
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
		Logs:        make([]map[string]any, len(lm.logBuffer)),
		IndexCounts: lm.GetIndexCounts(),
		HasLogs:     true,
	}
	copy(result.Logs, lm.logBuffer)
	lm.logBuffer = lm.logBuffer[:0]

	return result
}

// GetLastLogs returns the last n log entries
func (lm *LogManager) GetLastLogs(n int) []map[string]any {
	lm.logStoreMu.RLock()
	defer lm.logStoreMu.RUnlock()

	res := []map[string]any{}
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
func (lm *LogManager) SearchLogs(payload SearchPayload, maxLogs int) []map[string]any {
	lm.logStoreMu.RLock()
	defer lm.logStoreMu.RUnlock()

	result := []map[string]any{}
	count := 0

	// Start from the end (most recent logs)
	for i := len(lm.logOrder) - 1; i >= 0 && count < maxLogs; i-- {
		entryId := lm.logOrder[i]
		if entry, ok := lm.logStore[entryId]; ok {
			if lm.logMatches(&entry.Raw, &payload) {
				result = append([]map[string]any{entry.Raw}, result...)
				count++
			}
		}
	}

	return result
}

func (lm *LogManager) FilterLogs(logs []map[string]any, payload SearchPayload) []map[string]any {
	filteredLogs := []map[string]any{}
	for _, log := range logs {
		if lm.logMatches(&log, &payload) {
			filteredLogs = append(filteredLogs, log)
		}
	}
	return filteredLogs
}

func (lm *LogManager) logMatches(raw *map[string]any, payload *SearchPayload) bool {
	return lm.logMatchesFilter(raw, &payload.Filters) && lm.logMatchesSearch(raw, payload.SearchTerm)
}

// logMatchesFilter checks if a log entry matches the given filters
func (lm *LogManager) logMatchesFilter(raw *map[string]any, filter *map[string][]string) bool {
	if filter == nil {
		return true
	}
	flat := make(map[string]any)
	flattenMap(raw, flat, "")
	for k, vals := range *filter {
		valStr := toString(flat[k])
		match := slices.Contains(vals, valStr)
		if !match {
			return false
		}
	}
	return true
}

// logMatchesSearch checks if a log entry matches the search term
func (lm *LogManager) logMatchesSearch(raw *map[string]any, searchTerm string) bool {
	if searchTerm == "" {
		return true
	}
	searchTerm = strings.ToLower(searchTerm)

	// Search in flattened structure
	flat := make(map[string]any)
	flattenMap(raw, flat, "")

	for _, value := range flat {
		valueStr := toString(value)
		if strings.Contains(strings.ToLower(valueStr), searchTerm) {
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
			flatOld := make(map[string]any)
			flattenMap(&entry.Raw, flatOld, "")
			lm.removeFromIndex(oldest, flatOld)
			delete(lm.logStore, oldest)
			// Notify via callback about index update
			if lm.config.UpdateIndexCallback != nil {
				lm.config.UpdateIndexCallback(lm.GetIndexCounts())
			}
		}
	}
}

// updateIndex adds a log entry to the search index
func (lm *LogManager) updateIndex(entryId uint, flat map[string]any) {
	for k, v := range flat {
		valStr := toString(v)
		if valStr == "" {
			continue // omit empty values
		}
		if len(valStr) > lm.config.MaxIndexValueLength {
			continue // omit very long values
		}
		if lm.blacklist[k] {
			continue // skip blacklisted properties
		}
		if _, ok := lm.index[k]; !ok {
			lm.index[k] = make(map[string][]uint)
		}
		valMap := lm.index[k]
		valMap[valStr] = append(valMap[valStr], entryId)
		// Blacklist if too many unique values
		if len(valMap) > lm.config.MaxIndexValues {
			delete(lm.index, k)
			lm.blacklist[k] = true
			// Notify via callback about dropped index
			if lm.config.DropIndexKeysCallback != nil {
				lm.config.DropIndexKeysCallback([]string{k})
			}
		}
	}
}

// removeFromIndex removes a log entry from the search index
func (lm *LogManager) removeFromIndex(entryId uint, flat map[string]any) {
	for k, v := range flat {
		valStr := toString(v)
		if len(valStr) > lm.config.MaxIndexValueLength {
			continue // omit very long values
		}
		if valMap, ok := lm.index[k]; ok {
			if entryIds, ok := valMap[valStr]; ok {
				// Remove uint from slice
				newEntryIds := []uint{}
				for _, id := range entryIds {
					if id != entryId {
						newEntryIds = append(newEntryIds, id)
					}
				}
				if len(newEntryIds) == 0 {
					delete(valMap, valStr)
				} else {
					valMap[valStr] = newEntryIds
				}
			}
			if len(valMap) == 0 {
				delete(lm.index, k)
			}
		}
	}
}

// GetIndexCounts returns the count of entries for each indexed property value
func (lm *LogManager) GetIndexCounts() map[string]map[string]int {
	result := make(map[string]map[string]int)
	for k, valMap := range lm.index {
		result[k] = make(map[string]int)
		for v, entryIds := range valMap {
			result[k][v] = len(entryIds)
		}
	}
	return result
}

// GetLogsCount returns the number of logs currently stored
func (lm *LogManager) GetLogsCount() int {
	lm.logStoreMu.RLock()
	defer lm.logStoreMu.RUnlock()
	return len(lm.logStore)
}

// IsBlacklisted returns whether a property is blacklisted
func (lm *LogManager) IsBlacklisted(property string) bool {
	return lm.blacklist[property]
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
func flattenMap(data *map[string]any, out map[string]any, prefix string) {
	for k, v := range *data {
		key := prefix + k
		switch val := v.(type) {
		case map[string]any:
			flattenMap(&val, out, key+".")
		default:
			out[key] = val
		}
	}
}
