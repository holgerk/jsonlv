package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWatcherAddDeduplicates(t *testing.T) {
	p := filepath.Join(t.TempDir(), "test.log")
	b := newBroker()
	w := NewWatcher(b)

	w.Add(p)
	w.Add(p)

	w.mu.Lock()
	n := len(w.tailed)
	w.mu.Unlock()
	assert.Equal(t, 1, n)
}

func TestWatcherReopenSortedOrdersByTimestamp(t *testing.T) {
	dir := t.TempDir()

	fileA := filepath.Join(dir, "a.log")
	require.NoError(t, os.WriteFile(fileA, []byte(
		`{"time":"2024-01-15T10:00:00Z","message":"A1"}`+"\n"+
			`{"time":"2024-01-15T10:02:00Z","message":"A2"}`+"\n",
	), 0o644))

	fileB := filepath.Join(dir, "b.log")
	require.NoError(t, os.WriteFile(fileB, []byte(
		`{"time":"2024-01-15T09:59:00Z","message":"B1"}`+"\n"+
			`{"time":"2024-01-15T10:01:00Z","message":"B2"}`+"\n",
	), 0o644))

	b := newBroker()
	w := NewWatcher(b)
	w.ReopenSorted([]string{fileA, fileB})

	hist, _ := b.subscribe()
	require.Len(t, hist, 4)

	msgs := make([]string, 4)
	for i, msg := range hist {
		var obj map[string]string
		require.NoError(t, json.Unmarshal([]byte(msg.D), &obj))
		msgs[i] = obj["message"]
	}
	assert.Equal(t, []string{"B1", "A1", "B2", "A2"}, msgs)
}

func TestWatcherReopenSortedMarksFilesAsTailed(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.log")
	fileB := filepath.Join(dir, "b.log")
	require.NoError(t, os.WriteFile(fileA, []byte(""), 0o644))
	require.NoError(t, os.WriteFile(fileB, []byte(""), 0o644))

	b := newBroker()
	w := NewWatcher(b)
	w.ReopenSorted([]string{fileA, fileB})

	// Give follow goroutines a moment to start, then verify dedup
	time.Sleep(20 * time.Millisecond)
	w.Add(fileA) // should be a no-op

	w.mu.Lock()
	n := len(w.tailed)
	w.mu.Unlock()
	assert.Equal(t, 2, n)
}
