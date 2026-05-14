package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAddRecent(t *testing.T) {
	t.Run("adds to empty list", func(t *testing.T) {
		configDirOverride = t.TempDir()
		t.Cleanup(func() { configDirOverride = "" })

		got := addRecent([]string{"/a.log"})
		assert.Equal(t, []string{"/a.log"}, got)
	})

	t.Run("multiple paths prepended in argument order", func(t *testing.T) {
		configDirOverride = t.TempDir()
		t.Cleanup(func() { configDirOverride = "" })

		got := addRecent([]string{"/a.log", "/b.log", "/c.log"})
		assert.Equal(t, []string{"/a.log", "/b.log", "/c.log"}, got)
	})

	t.Run("deduplicates and moves existing entry to front", func(t *testing.T) {
		configDirOverride = t.TempDir()
		t.Cleanup(func() { configDirOverride = "" })

		addRecent([]string{"/a.log", "/b.log", "/c.log"})
		got := addRecent([]string{"/b.log"})
		assert.Equal(t, []string{"/b.log", "/a.log", "/c.log"}, got)
	})

	t.Run("caps at maxRecent", func(t *testing.T) {
		configDirOverride = t.TempDir()
		t.Cleanup(func() { configDirOverride = "" })

		paths := make([]string, maxRecent+5)
		for i := range paths {
			paths[i] = fmt.Sprintf("/file%d.log", i)
		}
		got := addRecent(paths)
		assert.Len(t, got, maxRecent)
	})
}
