package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveConfigPathUsesExplicitPath(t *testing.T) {
	path, err := resolveConfigPath("/tmp/custom.yaml")

	require.NoError(t, err)
	require.Equal(t, "/tmp/custom.yaml", path)
}

func TestResolveConfigPathFindsCmdDirectoryConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.WriteFile("config.yaml", []byte("port: :8080"), 0o644))

	path, err := resolveConfigPath("")

	require.NoError(t, err)
	require.Equal(t, "./config.yaml", path)
}

func TestResolveConfigPathFindsRepoRootConfig(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.MkdirAll("cmd", 0o755))
	require.NoError(t, os.WriteFile(filepath.Join("cmd", "config.yaml"), []byte("port: :8080"), 0o644))

	path, err := resolveConfigPath("")

	require.NoError(t, err)
	require.Equal(t, "./cmd/config.yaml", path)
}
