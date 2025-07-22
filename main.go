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
	"strings"
	"time"

	"log"
	"net/http"

	"sync"

	"embed"
	"io/fs"

	"flag"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type LogEntry struct {
	UUID string         `json:"uuid"`
	Raw  map[string]any `json:"raw"`
}

type SetFilterPayload struct {
	SearchTerm string              `json:"searchTerm"`
	Filters    map[string][]string `json:"filters"`
}

// flattenMap flattens a nested map using dot notation
func flattenMap(data map[string]any, prefix string, out map[string]any) {
	for k, v := range data {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			flattenMap(val, key, out)
		default:
			out[key] = val
		}
	}
}

// Index structure: map[propertyKey][propertyValue][]uuid
var (
	logStore            = make(map[string]LogEntry)
	logOrder            = []string{}
	index               = make(map[string]map[string][]string)
	blacklist           = make(map[string]bool)
	maxIndexValues      = 10
	maxLogs             = 10000
	maxIndexValueLength = 50 // default, can be overridden by flag
	logStoreMu          sync.RWMutex
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsClient struct {
	conn       *websocket.Conn
	filter     map[string][]string
	searchTerm string
	writeMu    sync.Mutex
}

var wsClients = make(map[*websocket.Conn]*wsClient)
var wsBroadcast = make(chan []byte)
var wsClientsMu sync.Mutex

func wsSend(client *wsClient, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	return client.conn.WriteMessage(websocket.TextMessage, data)
}

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

func getIndexCounts() map[string]map[string]int {
	result := make(map[string]map[string]int)
	for k, valMap := range index {
		result[k] = make(map[string]int)
		for v, uuids := range valMap {
			result[k][v] = len(uuids)
		}
	}
	return result
}

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

func logMatchesFilter(raw map[string]any, filter map[string][]string) bool {
	flat := make(map[string]any)
	flattenMap(raw, "", flat)
	for k, vals := range filter {
		valStr := toString(flat[k])
		match := slices.Contains(vals, valStr)
		if !match {
			return false
		}
	}
	return true
}

func logMatchesSearch(raw map[string]any, searchTerm string) bool {
	if searchTerm == "" {
		return true
	}
	searchTerm = strings.ToLower(searchTerm)

	// Search in flattened structure
	flat := make(map[string]any)
	flattenMap(raw, "", flat)

	for _, value := range flat {
		valueStr := toString(value)
		if strings.Contains(strings.ToLower(valueStr), searchTerm) {
			return true
		}
	}
	return false
}

func filterLogsWithSearch(payload SetFilterPayload, n int) []map[string]any {
	logStoreMu.RLock()
	defer logStoreMu.RUnlock()

	result := []map[string]any{}
	count := 0

	// Start from the end (most recent logs)
	for i := len(logOrder) - 1; i >= 0 && count < n; i-- {
		uuid := logOrder[i]
		if entry, ok := logStore[uuid]; ok {
			if logMatchesFilter(entry.Raw, payload.Filters) && logMatchesSearch(entry.Raw, payload.SearchTerm) {
				result = append([]map[string]any{entry.Raw}, result...)
				count++
			}
		}
	}

	return result
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()
	client := &wsClient{conn: conn, filter: nil}
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
		// Handle set_filter
		var req struct {
			Type    string           `json:"type"`
			Payload SetFilterPayload `json:"payload"`
		}
		if err := json.Unmarshal(msg, &req); err == nil && req.Type == "set_filter" {
			wsClientsMu.Lock()
			if c, ok := wsClients[conn]; ok {
				c.filter = req.Payload.Filters
				c.searchTerm = req.Payload.SearchTerm
			}
			wsClientsMu.Unlock()
			// Send set_logs with filtered logs
			filtered := filterLogsWithSearch(req.Payload, 1000)
			wsSend(client, map[string]any{
				"type":    "set_logs",
				"payload": filtered,
			})
		}
	}
}

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

func statusBroadcastLoop() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		wsBroadcastMsg(getStatusMessage())
	}
}

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

//go:embed web/index.html web/style.css web/app.js
var webFS embed.FS

var webFiles, _ = fs.Sub(webFS, "web")

func toString(val any) string {
	switch v := val.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case float64:
		return fmt.Sprintf("%g", v)
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

func updateIndex(uuid string, flat map[string]any) {
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
			index[k] = make(map[string][]string)
		}
		valMap := index[k]
		valMap[valStr] = append(valMap[valStr], uuid)
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

func removeFromIndex(uuid string, flat map[string]any) {
	for k, v := range flat {
		valStr := toString(v)
		if len(valStr) > maxIndexValueLength {
			continue // omit very long values
		}
		if valMap, ok := index[k]; ok {
			if uuids, ok := valMap[valStr]; ok {
				// Remove uuid from slice
				newUuids := []string{}
				for _, id := range uuids {
					if id != uuid {
						newUuids = append(newUuids, id)
					}
				}
				if len(newUuids) == 0 {
					delete(valMap, valStr)
				} else {
					valMap[valStr] = newUuids
				}
			}
			if len(valMap) == 0 {
				delete(index, k)
			}
		}
	}
}

func main() {
	devMode := flag.Bool("dev", false, "Serve web files from filesystem (for development)")
	flag.IntVar(&maxIndexValueLength, "maxIndexValueLength", 50, "Maximum length of values to index (omit longer values)")
	flag.Parse()

	reader := bufio.NewReader(os.Stdin)

	go wsBroadcastLoop()
	go statusBroadcastLoop()

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
			u := uuid.New().String()
			flat := make(map[string]any)
			flattenMap(raw, "", flat)

			logStoreMu.Lock()
			logStore[u] = LogEntry{
				UUID: u,
				Raw:  raw,
			}
			logOrder = append(logOrder, u)
			updateIndex(u, flat)

			// Enforce maxLogs
			enforeMaxLogs()
			logStoreMu.Unlock()

			// Broadcast add_logs (send only raw log)
			wsClientsMu.Lock()
			for _, c := range wsClients {
				if (c.filter == nil || logMatchesFilter(raw, c.filter)) && logMatchesSearch(raw, c.searchTerm) {
					wsSend(c, map[string]any{
						"type":    "add_logs",
						"payload": []map[string]any{raw},
					})
				}
			}
			wsClientsMu.Unlock()

			// Broadcast update_index (send full index for now)
			wsBroadcastMsg(map[string]any{
				"type":    "update_index",
				"payload": getIndexCounts(),
			})
		}
		// else: not JSON, just echo
	}
}

func enforeMaxLogs() {
	if len(logOrder) > maxLogs {
		oldest := logOrder[0]
		logOrder = logOrder[1:]
		if entry, ok := logStore[oldest]; ok {
			flatOld := make(map[string]any)
			flattenMap(entry.Raw, "", flatOld)
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
