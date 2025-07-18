// turbo-tail: see README.md for full specification
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
)

type LogEntry struct {
	UUID   string                 `json:"uuid"`
	Raw    map[string]interface{} `json:"raw"`
	Flat   map[string]interface{} `json:"flat"`
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

func main() {
	logStore := make(map[string]LogEntry)
	reader := bufio.NewReader(os.Stdin)

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
				Flat: flat,
			}
			// TODO: Update index and notify websocket clients
		}
		// else: not JSON, just echo
	}
	// TODO: Start web server and websocket logic
} 