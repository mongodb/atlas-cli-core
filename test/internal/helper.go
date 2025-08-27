package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TempConfigFolder(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("home", tmpDir)
	t.Setenv("XDG_CONFIG_HOME", tmpDir)
	t.Setenv("AppData", tmpDir)

	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	dir = filepath.Join(dir, "atlascli")

	err = os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	return dir
}
