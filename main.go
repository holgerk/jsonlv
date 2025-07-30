// turbo-tail: see README.md for full specification
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
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

type Client struct {
	conn          *websocket.Conn
	searchPayload SearchPayload
	indexCounts   IndexCounts
	writeMu       sync.Mutex
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

var wsClients = make(map[*websocket.Conn]*Client)
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
	maxLogsFlag := flag.Int("maxLogs", 10000, "Maximum number of logs to store in memory")
	flag.Parse()

	// Initialize LogManager
	config := DefaultLogManagerConfig()
	config.MaxIndexValueLength = *maxIndexValueLengthFlag
	config.MaxLogs = *maxLogsFlag
	config.DropIndexKeysCallback = broadcastDroppedIndexKeysMessage
	logManager = NewLogManager(config)

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

		var raw JsonObject
		if err := json.Unmarshal([]byte(line), &raw); err == nil {
			logManager.AddLogEntry(raw)
		}
		// else: not JSON, just echo
	}
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

		wsClientsMu.Lock()
		for _, client := range wsClients {
			logsToSend := logManager.FilterLogs(result.Logs, client.searchPayload)
			if len(logsToSend) > 0 {
				wsSend(client, JsonObject{
					"type":    "add_logs",
					"payload": logsToSend,
				})
			}
			if client.indexCounts == nil {
				sendUpdateIndexMessage(client, logManager.GetIndexCounts())
			} else {
				logManager.increaseClientIndexCounts(client.indexCounts, client.searchPayload.Filters, result.Logs)
				sendUpdateIndexMessage(client, client.indexCounts)
			}
		}
		wsClientsMu.Unlock()
	}
}

func sendUpdateIndexMessage(client *Client, indexCounts IndexCounts) {
	wsSend(client, JsonObject{
		"type":    "update_index",
		"payload": indexCounts,
	})
}

func broadcastDroppedIndexKeysMessage(droppedKeys []string) {
	wsBroadcastMsg(JsonObject{
		"type":    "drop_index",
		"payload": droppedKeys,
	})
}

// getStatusMessage creates a status message with system information
func getStatusMessage() JsonObject {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return JsonObject{
		"type": "set_status",
		"payload": JsonObject{
			"allocatedMemory": m.Alloc,
			"logsStored":      logManager.GetLogsCount(),
		},
	}
}

// wsSend sends a message to a specific WebSocket client
func wsSend(client *Client, msg any) error {
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
	client := &Client{conn: conn}
	wsClientsMu.Lock()
	wsClients[conn] = client
	wsClientsMu.Unlock()


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
			searchLogsResult := logManager.SearchLogs(req.Payload, 1000)
			client.indexCounts = searchLogsResult.IndexCounts
			wsSend(client, JsonObject{
				"type": "set_logs",
				"payload": JsonObject{
					"logs":        searchLogsResult.Logs,
					"indexCounts": searchLogsResult.IndexCounts,
				},
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
