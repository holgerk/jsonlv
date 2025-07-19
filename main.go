// turbo-tail: see README.md for full specification
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

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
	UUID string                 `json:"uuid"`
	Raw  map[string]interface{} `json:"raw"`
}

// flattenMap flattens a nested map using dot notation
func flattenMap(data map[string]interface{}, prefix string, out map[string]interface{}) {
	for k, v := range data {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]interface{}:
			flattenMap(val, key, out)
		default:
			out[key] = val
		}
	}
}

// Index structure: map[propertyKey][propertyValue][]uuid
var (
	index               = make(map[string]map[string][]string)
	blacklist           = make(map[string]bool)
	maxIndexValues      = 10
	maxLogs             = 10000
	maxIndexValueLength = 50 // default, can be overridden by flag
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type wsClient struct {
	conn       *websocket.Conn
	filter     map[string][]string
	searchTerm string
}

var wsClients = make(map[*websocket.Conn]*wsClient)
var wsBroadcast = make(chan []byte)
var wsClientsMu sync.Mutex

func wsSend(conn *websocket.Conn, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

func wsBroadcastMsg(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	wsClientsMu.Lock()
	defer wsClientsMu.Unlock()
	for client := range wsClients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			client.Close()
			delete(wsClients, client)
		}
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

func getLastLogs(logStore map[string]LogEntry, logOrder []string, n int) []map[string]interface{} {
	res := []map[string]interface{}{}
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

func logMatchesFilter(raw map[string]interface{}, filter map[string][]string) bool {
	flat := make(map[string]interface{})
	flattenMap(raw, "", flat)
	for k, vals := range filter {
		valStr := toString(flat[k])
		match := false
		for _, v := range vals {
			if v == valStr {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

func logMatchesSearch(raw map[string]interface{}, searchTerm string) bool {
	if searchTerm == "" {
		return true
	}
	searchTerm = strings.ToLower(searchTerm)

	// Search in flattened structure
	flat := make(map[string]interface{})
	flattenMap(raw, "", flat)

	for _, value := range flat {
		valueStr := toString(value)
		if strings.Contains(strings.ToLower(valueStr), searchTerm) {
			return true
		}
	}
	return false
}

func filterLogsWithSearch(logStore map[string]LogEntry, logOrder []string, payload map[string]interface{}, n int) []map[string]interface{} {
	// Extract filter and search term from payload
	filter := make(map[string][]string)
	searchTerm := ""

	// Extract searchTerm
	if searchStr, ok := payload["searchTerm"].(string); ok {
		searchTerm = searchStr
	}

	// Extract filters
	if filtersMap, ok := payload["filters"].(map[string]interface{}); ok {
		for k, v := range filtersMap {
			if values, ok := v.([]interface{}); ok {
				for _, val := range values {
					if str, ok := val.(string); ok {
						filter[k] = append(filter[k], str)
					}
				}
			}
		}
	}

	result := []map[string]interface{}{}
	count := 0

	// Start from the end (most recent logs)
	for i := len(logOrder) - 1; i >= 0 && count < n; i-- {
		uuid := logOrder[i]
		if entry, ok := logStore[uuid]; ok {
			if logMatchesFilter(entry.Raw, filter) && logMatchesSearch(entry.Raw, searchTerm) {
				result = append([]map[string]interface{}{entry.Raw}, result...)
				count++
			}
		}
	}

	return result
}

func wsHandler(logStore *map[string]LogEntry, logOrder *[]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}
		defer conn.Close()
		wsClientsMu.Lock()
		wsClients[conn] = &wsClient{conn: conn, filter: nil}
		wsClientsMu.Unlock()

		// On connect: send set_index and set_logs (unfiltered)
		setIndex := map[string]interface{}{
			"type":    "set_index",
			"payload": getIndexCounts(),
		}
		wsSend(conn, setIndex)

		setLogs := map[string]interface{}{
			"type":    "set_logs",
			"payload": getLastLogs(*logStore, *logOrder, 1000),
		}
		wsSend(conn, setLogs)

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
				Type    string                 `json:"type"`
				Payload map[string]interface{} `json:"payload"`
			}
			if err := json.Unmarshal(msg, &req); err == nil && req.Type == "set_filter" {
				wsClientsMu.Lock()
				if c, ok := wsClients[conn]; ok {
					// Extract filter and search term from payload
					filter := make(map[string][]string)
					searchTerm := ""

					// Extract searchTerm
					if searchStr, ok := req.Payload["searchTerm"].(string); ok {
						searchTerm = searchStr
					}

					// Extract filters
					if filtersMap, ok := req.Payload["filters"].(map[string]interface{}); ok {
						for k, v := range filtersMap {
							if values, ok := v.([]interface{}); ok {
								for _, val := range values {
									if str, ok := val.(string); ok {
										filter[k] = append(filter[k], str)
									}
								}
							}
						}
					}

					c.filter = filter
					c.searchTerm = searchTerm
				}
				wsClientsMu.Unlock()
				// Send set_logs with filtered logs
				filtered := filterLogsWithSearch(*logStore, *logOrder, req.Payload, 1000)
				wsSend(conn, map[string]interface{}{
					"type":    "set_logs",
					"payload": filtered,
				})
			}
		}
	}
}

func wsBroadcastLoop() {
	for msg := range wsBroadcast {
		for client := range wsClients {
			if err := client.WriteMessage(websocket.TextMessage, msg); err != nil {
				client.Close()
				delete(wsClients, client)
			}
		}
	}
}

//go:embed web/index.html web/style.css web/app.js
var webFS embed.FS

var webFiles, _ = fs.Sub(webFS, "web")

func toString(val interface{}) string {
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

func updateIndex(uuid string, flat map[string]interface{}) {
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
			dropMsg := map[string]interface{}{
				"type":    "drop_index",
				"payload": []string{k},
			}
			wsBroadcastMsg(dropMsg)
		}
	}
}

func removeFromIndex(uuid string, flat map[string]interface{}) {
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

	logStore := make(map[string]LogEntry)
	logOrder := []string{} // maintain insertion order for maxLogs
	reader := bufio.NewReader(os.Stdin)

	go wsBroadcastLoop()

	if *devMode {
		log.Println("[dev mode] Serving web files from ./web directory")
		http.Handle("/", http.FileServer(http.Dir("web")))
	} else {
		http.Handle("/", http.FileServer(http.FS(webFiles)))
	}
	http.HandleFunc("/ws", wsHandler(&logStore, &logOrder))
	go func() {
		log.Println("Web server listening on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
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

		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err == nil {
			u := uuid.New().String()
			flat := make(map[string]interface{})
			flattenMap(raw, "", flat)
			logStore[u] = LogEntry{
				UUID: u,
				Raw:  raw,
			}
			logOrder = append(logOrder, u)
			updateIndex(u, flat)

			// Enforce maxLogs
			if len(logOrder) > maxLogs {
				oldest := logOrder[0]
				logOrder = logOrder[1:]
				if entry, ok := logStore[oldest]; ok {
					flatOld := make(map[string]interface{})
					flattenMap(entry.Raw, "", flatOld)
					removeFromIndex(oldest, flatOld)
					delete(logStore, oldest)
					// Notify clients with update_index (send full index for now)
					wsBroadcastMsg(map[string]interface{}{
						"type":    "update_index",
						"payload": getIndexCounts(),
					})
				}
			}

			// Broadcast add_logs (send only raw log)
			wsClientsMu.Lock()
			for _, c := range wsClients {
				if (c.filter == nil || logMatchesFilter(raw, c.filter)) && logMatchesSearch(raw, c.searchTerm) {
					wsSend(c.conn, map[string]interface{}{
						"type":    "add_logs",
						"payload": []map[string]interface{}{raw},
					})
				}
			}
			wsClientsMu.Unlock()

			// Broadcast update_index (send full index for now)
			wsBroadcastMsg(map[string]interface{}{
				"type":    "update_index",
				"payload": getIndexCounts(),
			})
		}
		// else: not JSON, just echo
	}
}
