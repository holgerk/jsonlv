package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTempLog(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "test.log")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

func TestLastNLines(t *testing.T) {
	t.Run("empty file", func(t *testing.T) {
		p := writeTempLog(t, "")
		lines, err := lastNLines(p, 10)
		require.NoError(t, err)
		assert.Empty(t, lines)
	})

	t.Run("fewer lines than N returns all", func(t *testing.T) {
		p := writeTempLog(t, "a\nb\nc\n")
		lines, err := lastNLines(p, 10)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, lines)
	})

	t.Run("exactly N lines", func(t *testing.T) {
		p := writeTempLog(t, "a\nb\nc\n")
		lines, err := lastNLines(p, 3)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, lines)
	})

	t.Run("more lines than N returns last N", func(t *testing.T) {
		p := writeTempLog(t, "a\nb\nc\nd\ne\n")
		lines, err := lastNLines(p, 3)
		require.NoError(t, err)
		assert.Equal(t, []string{"c", "d", "e"}, lines)
	})

	t.Run("N=0 returns nothing", func(t *testing.T) {
		p := writeTempLog(t, "a\nb\nc\n")
		lines, err := lastNLines(p, 0)
		require.NoError(t, err)
		assert.Empty(t, lines)
	})

	t.Run("large file beyond chunk boundary", func(t *testing.T) {
		// Write more than 32 KB so lastNLines must read multiple chunks.
		var sb strings.Builder
		for i := range 2000 {
			sb.WriteString(strings.Repeat("x", 20))
			sb.WriteString("\n")
			_ = i
		}
		p := writeTempLog(t, sb.String())
		lines, err := lastNLines(p, 10)
		require.NoError(t, err)
		assert.Len(t, lines, 10)
		for _, l := range lines {
			assert.Equal(t, strings.Repeat("x", 20), l)
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		_, err := lastNLines(filepath.Join(t.TempDir(), "nope.log"), 10)
		assert.Error(t, err)
	})
}
