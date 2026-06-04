package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type tailLine struct {
	source string
	line   string
	ts     time.Time
}

func parseLineTime(line string) time.Time {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return time.Time{}
	}
	for _, key := range []string{"datetime", "timestamp", "time", "@timestamp"} {
		raw, ok := obj[key]
		if !ok {
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) == nil {
			for _, layout := range []string{
				time.RFC3339Nano,
				time.RFC3339,
				"2006-01-02 15:04:05.999999999Z07:00",
				"2006-01-02 15:04:05.999999999",
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05.999999999",
				"2006-01-02T15:04:05",
			} {
				if t, err := time.Parse(layout, s); err == nil {
					return t
				}
			}
		}
		var f float64
		if json.Unmarshal(raw, &f) == nil {
			if f > 1e10 { // millisecond epoch (> year 2001 in ms)
				ms := int64(f)
				return time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond))
			}
			sec := int64(f)
			return time.Unix(sec, int64((f-float64(sec))*1e9))
		}
	}
	return time.Time{}
}

// Watcher tracks which files are being tailed and coordinates
// line delivery into the broker.
type Watcher struct {
	b      *broker
	mu     sync.Mutex
	tailed map[string]bool
}

func NewWatcher(b *broker) *Watcher {
	return &Watcher{b: b, tailed: map[string]bool{}}
}

// Register records path as open without starting a tail goroutine.
func (w *Watcher) Register(path string) {
	w.mu.Lock()
	w.tailed[path] = true
	w.mu.Unlock()
}

// Files returns the paths of all currently tracked files.
func (w *Watcher) Files() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	files := make([]string, 0, len(w.tailed))
	for p := range w.tailed {
		files = append(files, p)
	}
	return files
}

// Add begins tailing path if it is not already being watched.
func (w *Watcher) Add(path string) {
	w.mu.Lock()
	already := w.tailed[path]
	if !already {
		w.tailed[path] = true
	}
	w.mu.Unlock()
	if already {
		return
	}
	source := filepath.Base(path)
	go func() {
		tail, err := lastNLines(path, 1000)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			return
		}
		for _, line := range tail {
			if line != "" {
				w.b.publish(source, line)
			}
		}
		followFile(path, source, w.b)
	}()
}

// ReopenSorted reads the last 1000 lines from each path in parallel,
// sorts all lines by timestamp, publishes them as a single batch,
// then begins following each file for new lines.
func (w *Watcher) ReopenSorted(paths []string) {
	w.mu.Lock()
	for _, p := range paths {
		w.tailed[p] = true
	}
	w.mu.Unlock()

	var mu sync.Mutex
	var all []tailLine
	var wg sync.WaitGroup

	for _, path := range paths {
		path := path
		source := filepath.Base(path)
		wg.Add(1)
		go func() {
			defer wg.Done()
			tail, err := lastNLines(path, 1000)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
				return
			}
			local := make([]tailLine, 0, len(tail))
			for _, line := range tail {
				if line != "" {
					local = append(local, tailLine{source, line, parseLineTime(line)})
				}
			}
			mu.Lock()
			all = append(all, local...)
			mu.Unlock()
		}()
	}
	wg.Wait()

	sort.SliceStable(all, func(i, j int) bool {
		ti, tj := all[i].ts, all[j].ts
		if ti.IsZero() != tj.IsZero() {
			return tj.IsZero()
		}
		return ti.Before(tj)
	})

	msgs := make([]logMsg, len(all))
	for i, l := range all {
		msgs[i] = logMsg{S: l.source, D: l.line}
	}
	w.b.publishBatch(msgs)

	for _, path := range paths {
		path := path
		source := filepath.Base(path)
		go followFile(path, source, w.b)
	}
}
