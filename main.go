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

// ============================================================================
// Global Variables
// ============================================================================

var (
	logOrder    = []uint{}
	logStore    = make(map[uint]LogEntry)
	logStoreMu  sync.RWMutex
	logBuffer   = []map[string]any{}
	logBufferMu sync.RWMutex
	// structure: map[propertyKey][propertyValue][]id
	index          = make(map[string]map[string][]uint)
	blacklist      = make(map[string]bool)
	maxIndexValues = 10
	maxLogs        = 10000
	idCounter      = uint(0)
	// flags
	maxIndexValueLength = 50 // default, can be overridden by flag
	devMode             = false
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
	flag.BoolVar(&devMode, "dev", false, "Serve web files from filesystem (for development)")
	flag.IntVar(&maxIndexValueLength, "maxIndexValueLength", 50, "Maximum length of values to index (omit longer values)")
	flag.Parse()

	reader := bufio.NewReader(os.Stdin)

	go startWebserver()
	go wsBroadcastLoop()
	go statusBroadcastLoop()
	go sendBufferedLogs()

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
			idCounter++
			id := idCounter
			flat := make(map[string]any)
			flattenMap(&raw, flat, "")

			logStoreMu.Lock()
			logStore[id] = LogEntry{
				id:  id,
				Raw: raw,
			}
			logOrder = append(logOrder, id)
			updateIndex(id, flat)

			// Enforce maxLogs
			enforeMaxLogs()
			logStoreMu.Unlock()

			logBufferMu.Lock()
			logBuffer = append(logBuffer, raw)
			logBufferMu.Unlock()
		}
		// else: not JSON, just echo
	}
}

// ============================================================================
// Log Storage and Filtering Functions
// ============================================================================

// returns the last n log entries
func getLastLogs(n int) []map[string]any {
	logStoreMu.RLock()
	defer logStoreMu.RUnlock()

	res := []map[string]any{}
	start := 0
	if len(logOrder) > n {
		start = len(logOrder) - n
	}
	for _, uuid := range logOrder[start:] {
		if entry, ok := logStore[uuid]; ok {
			res = append(res, entry.Raw)
		}
	}
	return res
}

// returns filtered logs based on filters and search term
func searchLogs(payload SearchPayload, maxLogs int) []map[string]any {
	logStoreMu.RLock()
	defer logStoreMu.RUnlock()

	result := []map[string]any{}
	count := 0

	// Start from the end (most recent logs)
	for i := len(logOrder) - 1; i >= 0 && count < maxLogs; i-- {
		entryId := logOrder[i]
		if entry, ok := logStore[entryId]; ok {
			if logMatches(&entry.Raw, &payload) {
				result = append([]map[string]any{entry.Raw}, result...)
				count++
			}
		}
	}

	return result
}

func logMatches(raw *map[string]any, payload *SearchPayload) bool {
	return logMatchesFilter(raw, &payload.Filters) && logMatchesSearch(raw, payload.SearchTerm)
}

// checks if a log entry matches the given filters
func logMatchesFilter(raw *map[string]any, filter *map[string][]string) bool {
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

// checks if a log entry matches the search term
func logMatchesSearch(raw *map[string]any, searchTerm string) bool {
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

// enforces the maximum number of stored logs
func enforeMaxLogs() {
	if len(logOrder) > maxLogs {
		oldest := logOrder[0]
		logOrder = logOrder[1:]
		if entry, ok := logStore[oldest]; ok {
			flatOld := make(map[string]any)
			flattenMap(&entry.Raw, flatOld, "")
			removeFromIndex(oldest, flatOld)
			delete(logStore, oldest)
			// Notify clients with update_index (send full index for now)
			wsBroadcastMsg(map[string]any{
				"type":    "update_index",
				"payload": getIndexCounts(),
			})
		}
	}
}

// ============================================================================
// Index Management Functions
// ============================================================================

// updateIndex adds a log entry to the search index
func updateIndex(entryId uint, flat map[string]any) {
	for k, v := range flat {
		valStr := toString(v)
		if valStr == "" {
			continue // omit empty values
		}
		if len(valStr) > maxIndexValueLength {
			continue // omit very long values
		}
		if blacklist[k] {
			continue // skip blacklisted properties
		}
		if _, ok := index[k]; !ok {
			index[k] = make(map[string][]uint)
		}
		valMap := index[k]
		valMap[valStr] = append(valMap[valStr], entryId)
		// Blacklist if too many unique values
		if len(valMap) > maxIndexValues {
			delete(index, k)
			blacklist[k] = true
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
func removeFromIndex(entryId uint, flat map[string]any) {
	for k, v := range flat {
		valStr := toString(v)
		if len(valStr) > maxIndexValueLength {
			continue // omit very long values
		}
		if valMap, ok := index[k]; ok {
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
				delete(index, k)
			}
		}
	}
}

// getIndexCounts returns the count of entries for each indexed property value
func getIndexCounts() map[string]map[string]int {
	result := make(map[string]map[string]int)
	for k, valMap := range index {
		result[k] = make(map[string]int)
		for v, entryIds := range valMap {
			result[k][v] = len(entryIds)
		}
	}
	return result
}

// ============================================================================
// WebSocket Functions
// ============================================================================

func sendBufferedLogs() {
	for {
		time.Sleep(100 * time.Millisecond)
		logBufferMu.Lock()
		if len(logBuffer) > 0 {
			// Broadcast add_logs (send only raw log)
			wsClientsMu.Lock()
			for _, c := range wsClients {
				logsToSend := []map[string]any{}
				for _, l := range logBuffer {
					if logMatches(&l, &c.searchPayload) {
						logsToSend = append(logsToSend, l)
					}
				}
				if len(logsToSend) > 0 {
					wsSend(c, map[string]any{
						"type":    "add_logs",
						"payload": logsToSend,
					})
				}
			}
			wsClientsMu.Unlock()
			logBuffer = []map[string]any{}

			// Broadcast update_index (send full index for now)
			wsBroadcastMsg(map[string]any{
				"type":    "update_index",
				"payload": getIndexCounts(),
			})
		}
		logBufferMu.Unlock()
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
		"payload": getIndexCounts(),
	}
	wsSend(client, setIndex)

	setLogs := map[string]any{
		"type":    "set_logs",
		"payload": getLastLogs(1000),
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
			filtered := searchLogs(req.Payload, 1000)
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

	logStoreMu.RLock()
	logsCount := len(logStore)
	logStoreMu.RUnlock()

	return map[string]any{
		"type": "set_status",
		"payload": map[string]any{
			"allocatedMemory": m.Alloc,
			"logsStored":      logsCount,
		},
	}
}

// ============================================================================
// Utility Functions
// ============================================================================

func startWebserver() {
	if devMode {
		log.Println("[dev mode] Serving web files from ./web directory")
		http.Handle("/", http.FileServer(http.Dir("web")))
	} else {
		http.Handle("/", http.FileServer(http.FS(webFiles)))
	}
	http.HandleFunc("/ws", wsHandler)

	log.Println("Web server listening on :8181")
	if err := http.ListenAndServe(":8181", nil); err != nil {
		log.Fatalf("Web server error: %v", err)
	}
}

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
