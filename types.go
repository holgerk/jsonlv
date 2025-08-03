package main

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
