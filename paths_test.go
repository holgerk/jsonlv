package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetPrefixMap(t *testing.T) {
	t.Helper()
	mappingMu.Lock()
	prefixMap = map[string]string{}
	mappingMu.Unlock()
	t.Cleanup(func() {
		mappingMu.Lock()
		prefixMap = map[string]string{}
		mappingMu.Unlock()
	})
}

func TestResolveLocalPathExistingFile(t *testing.T) {
	resetPrefixMap(t)
	p := filepath.Join(t.TempDir(), "file.php")
	require.NoError(t, os.WriteFile(p, []byte{}, 0o644))
	got, ok := resolveLocalPath(p)
	assert.True(t, ok)
	assert.Equal(t, p, got)
}

func TestResolveLocalPathViaMapping(t *testing.T) {
	resetPrefixMap(t)
	configDirOverride = t.TempDir()
	t.Cleanup(func() { configDirOverride = "" })

	addMapping("/remote/app/src/Foo.php", "/local/app/src/Foo.php")
	got, ok := resolveLocalPath("/remote/app/src/Bar.php")
	assert.True(t, ok)
	assert.Equal(t, "/local/app/src/Bar.php", got)
}

func TestResolveLocalPathUnknownRemote(t *testing.T) {
	resetPrefixMap(t)
	_, ok := resolveLocalPath("/remote/app/src/Missing.php")
	assert.False(t, ok)
}

func TestAddMappingDeducesCommonPrefix(t *testing.T) {
	resetPrefixMap(t)
	configDirOverride = t.TempDir()
	t.Cleanup(func() { configDirOverride = "" })

	// 4 trailing components are identical → prefix is just the first component of each
	addMapping("/remote/project/src/controllers/Foo.php", "/local/project/src/controllers/Foo.php")

	mappingMu.RLock()
	locPrefix, ok := prefixMap["/remote"]
	mappingMu.RUnlock()
	assert.True(t, ok)
	assert.Equal(t, "/local", locPrefix)
}

func TestAddMappingExactWhenNoCommonSuffix(t *testing.T) {
	resetPrefixMap(t)
	configDirOverride = t.TempDir()
	t.Cleanup(func() { configDirOverride = "" })

	addMapping("/remote/foo.php", "/local/bar.php")

	mappingMu.RLock()
	_, ok := prefixMap["/remote/foo.php"]
	mappingMu.RUnlock()
	assert.True(t, ok)
}
