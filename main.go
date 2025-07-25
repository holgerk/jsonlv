// turbo-tail: see README.md for full specification
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"log"
	"net/http"

	"sync"

	"embed"
	"io/fs"

	"flag"

	"github.com/gorilla/websocket"
)

// ============================================================================
// Type Definitions
// ============================================================================

type LogEntry struct {
	id  uint
	Raw map[string]any
}

type SearchPayload struct {
	SearchTerm string              `json:"searchTerm"`
	Filters    map[string][]string `json:"filters"`
}

type wsClient struct {
	conn          *websocket.Conn
	searchPayload SearchPayload
	writeMu       sync.Mutex
}

type BufferedLogsResult struct {
	Logs        []map[string]any
	IndexCounts map[string]map[string]int
	HasLogs     bool
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
	maxIndexValues      int
	maxLogs             int
	maxIndexValueLength int

	// ID generation
	idCounter   uint
	idCounterMu sync.Mutex
}

// ============================================================================
// Global Variables
// ============================================================================

var (
	logManager *LogManager
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var wsClients = make(map[*websocket.Conn]*wsClient)
var wsBroadcast = make(chan []byte)
var wsClientsMu sync.Mutex

//go:embed web/index.html web/style.css web/app.js
var webFS embed.FS

var webFiles, _ = fs.Sub(webFS, "web")

// ============================================================================
// Main Function
// ============================================================================

func main() {
	devMode := flag.Bool("dev", false, "Serve web files from filesystem (for development)")
	maxIndexValueLengthFlag := flag.Int("maxIndexValueLength", 50, "Maximum length of values to index (omit longer values)")
	flag.Parse()

	// Initialize LogManager
	logManager = NewLogManager(10, 10000, *maxIndexValueLengthFlag)

	reader := bufio.NewReader(os.Stdin)

	go wsBroadcastLoop()
	go statusBroadcastLoop()
	go sendBufferedLogs(logManager)

	if *devMode {
		log.Println("[dev mode] Serving web files from ./web directory")
		http.Handle("/", http.FileServer(http.Dir("web")))
	} else {
		http.Handle("/", http.FileServer(http.FS(webFiles)))
	}
	http.HandleFunc("/ws", wsHandler)
	go func() {
		log.Println("Web server listening on :8181")
		if err := http.ListenAndServe(":8181", nil); err != nil {
			log.Fatalf("Web server error: %v", err)
		}
	}()

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "Error reading stdin: %v\n", err)
			continue
		}
		line = strings.TrimRight(line, "\r\n")
		fmt.Println(line) // Echo to stdout

		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err == nil {
			logManager.AddLogEntry(raw)
		}
		// else: not JSON, just echo
	}
}

// ============================================================================
// LogManager Constructor and Methods
// ============================================================================

func NewLogManager(maxIndexValues, maxLogs, maxIndexValueLength int) *LogManager {
	return &LogManager{
		logOrder:            []uint{},
		logStore:            make(map[uint]LogEntry),
		logBuffer:           []map[string]any{},
		index:               make(map[string]map[string][]uint),
		blacklist:           make(map[string]bool),
		maxIndexValues:      maxIndexValues,
		maxLogs:             maxLogs,
		maxIndexValueLength: maxIndexValueLength,
		idCounter:           0,
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

func (lm *LogManager) LogMatches(raw *map[string]any, payload *SearchPayload) bool {
	return lm.logMatches(raw, payload)
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
	if len(lm.logOrder) > lm.maxLogs {
		oldest := lm.logOrder[0]
		lm.logOrder = lm.logOrder[1:]
		if entry, ok := lm.logStore[oldest]; ok {
			flatOld := make(map[string]any)
			flattenMap(&entry.Raw, flatOld, "")
			lm.removeFromIndex(oldest, flatOld)
			delete(lm.logStore, oldest)
			// Notify clients with update_index (send full index for now)
			wsBroadcastMsg(map[string]any{
				"type":    "update_index",
				"payload": lm.GetIndexCounts(),
			})
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
		if len(valStr) > lm.maxIndexValueLength {
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
		if len(valMap) > lm.maxIndexValues {
			delete(lm.index, k)
			lm.blacklist[k] = true
			// Notify websocket clients with drop_index
			dropMsg := map[string]any{
				"type":    "drop_index",
				"payload": []string{k},
			}
			wsBroadcastMsg(dropMsg)
		}
	}
}

// removeFromIndex removes a log entry from the search index
func (lm *LogManager) removeFromIndex(entryId uint, flat map[string]any) {
	for k, v := range flat {
		valStr := toString(v)
		if len(valStr) > lm.maxIndexValueLength {
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
// WebSocket Functions
// ============================================================================

func sendBufferedLogs(logManager *LogManager) {
	for {
		time.Sleep(100 * time.Millisecond)

		result := logManager.GetAndClearBufferedLogs()
		if !result.HasLogs {
			continue
		}

		// Handle WebSocket broadcasting
		wsClientsMu.Lock()
		for _, client := range wsClients {
			logsToSend := []map[string]any{}
			for _, logEntry := range result.Logs {
				if logManager.LogMatches(&logEntry, &client.searchPayload) {
					logsToSend = append(logsToSend, logEntry)
				}
			}
			if len(logsToSend) > 0 {
				wsSend(client, map[string]any{
					"type":    "add_logs",
					"payload": logsToSend,
				})
			}
		}
		wsClientsMu.Unlock()

		// Broadcast index update
		wsBroadcastMsg(map[string]any{
			"type":    "update_index",
			"payload": result.IndexCounts,
		})
	}
}

// wsSend sends a message to a specific WebSocket client
func wsSend(client *wsClient, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	return client.conn.WriteMessage(websocket.TextMessage, data)
}

// wsBroadcastMsg broadcasts a message to all connected WebSocket clients
func wsBroadcastMsg(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case wsBroadcast <- data:
	default:
		// Channel full, skip this broadcast
	}
}

// wsBroadcastLoop handles broadcasting messages to all WebSocket clients
func wsBroadcastLoop() {
	for msg := range wsBroadcast {
		wsClientsMu.Lock()
		for conn, client := range wsClients {
			client.writeMu.Lock()
			err := conn.WriteMessage(websocket.TextMessage, msg)
			client.writeMu.Unlock()
			if err != nil {
				conn.Close()
				delete(wsClients, conn)
			}
		}
		wsClientsMu.Unlock()
	}
}

// wsHandler handles WebSocket connections
func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()
	client := &wsClient{conn: conn}
	wsClientsMu.Lock()
	wsClients[conn] = client
	wsClientsMu.Unlock()

	// On connect: send set_index and set_logs (unfiltered)
	setIndex := map[string]any{
		"type":    "set_index",
		"payload": logManager.GetIndexCounts(),
	}
	wsSend(client, setIndex)

	setLogs := map[string]any{
		"type":    "set_logs",
		"payload": logManager.GetLastLogs(1000),
	}
	wsSend(client, setLogs)

	wsSend(client, getStatusMessage())

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			wsClientsMu.Lock()
			delete(wsClients, conn)
			wsClientsMu.Unlock()
			break
		}
		// Handle set_search
		var req struct {
			Type    string        `json:"type"`
			Payload SearchPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &req); err == nil && req.Type == "set_search" {
			wsClientsMu.Lock()
			if c, ok := wsClients[conn]; ok {
				c.searchPayload = req.Payload
			}
			wsClientsMu.Unlock()
			// Send set_logs with filtered logs
			filtered := logManager.SearchLogs(req.Payload, 1000)
			wsSend(client, map[string]any{
				"type":    "set_logs",
				"payload": filtered,
			})
		}
	}
}

// statusBroadcastLoop periodically broadcasts status updates
func statusBroadcastLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		wsBroadcastMsg(getStatusMessage())
	}
}

// getStatusMessage creates a status message with system information
func getStatusMessage() map[string]any {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return map[string]any{
		"type": "set_status",
		"payload": map[string]any{
			"allocatedMemory": m.Alloc,
			"logsStored":      logManager.GetLogsCount(),
		},
	}
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
