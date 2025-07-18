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

// Index structure: map[propertyKey][propertyValue][]uuid
var (
	index     = make(map[string]map[string][]string)
	blacklist = make(map[string]bool)
	maxIndexValues = 10
	maxLogs = 10000
)

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
			// TODO: Notify websocket clients with drop_index
		}
	}
}

func removeFromIndex(uuid string, flat map[string]interface{}) {
	for k, v := range flat {
		valStr := toString(v)
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
	logStore := make(map[string]LogEntry)
	logOrder := []string{} // maintain insertion order for maxLogs
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
			logOrder = append(logOrder, u)
			updateIndex(u, flat)

			// Enforce maxLogs
			if len(logOrder) > maxLogs {
				oldest := logOrder[0]
				logOrder = logOrder[1:]
				if entry, ok := logStore[oldest]; ok {
					removeFromIndex(oldest, entry.Flat)
					delete(logStore, oldest)
					// TODO: Notify websocket clients with update_index
				}
			}
			// TODO: Notify websocket clients with add_logs/update_index
		}
		// else: not JSON, just echo
	}
	// TODO: Start web server and websocket logic
} 