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

func TestWatcherCloseStopsTailingAndDropsEntries(t *testing.T) {
	dir := t.TempDir()
	keep := filepath.Join(dir, "keep.log")
	gone := filepath.Join(dir, "gone.log")
	require.NoError(t, os.WriteFile(keep, []byte("k1\n"), 0o644))
	require.NoError(t, os.WriteFile(gone, []byte("g1\n"), 0o644))
	b := newBroker()
	w := NewWatcher(b)
	w.Add(keep)
	w.Add(gone)
	require.Eventually(t, func() bool { hist, _ := b.subscribe(); return len(hist) == 2 },
		time.Second, 10*time.Millisecond)

	assert.True(t, w.Close(gone))

	assert.Equal(t, []string{keep}, w.Files())
	hist, _ := b.subscribe()
	assert.Equal(t, []logMsg{{S: "keep.log", D: "k1"}}, hist)

	f, err := os.OpenFile(gone, os.O_APPEND|os.O_WRONLY, 0)
	require.NoError(t, err)
	_, err = f.WriteString("g2\n")
	require.NoError(t, err)
	f.Close()
	time.Sleep(300 * time.Millisecond)
	hist, _ = b.subscribe()
	assert.Len(t, hist, 1)
}

func TestWatcherCloseKeepsEntriesOfSharedSource(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a", "app.log")
	c := filepath.Join(dir, "c", "app.log")
	b := newBroker()
	w := NewWatcher(b)
	w.Register(a)
	w.Register(c)
	b.publish("app.log", "x")

	assert.False(t, w.Close(a))

	hist, _ := b.subscribe()
	assert.Len(t, hist, 1)
}
