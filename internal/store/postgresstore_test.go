package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPostgresStoreImportConfigFromFileRejectsInvalidYAML(t *testing.T) {
	store := &PostgresStore{}
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configPath, []byte("port: [\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	imported, err := store.importConfigFromFile(context.Background(), configPath)
	if err == nil {
		t.Fatalf("expected importConfigFromFile() to fail for invalid yaml")
	}
	if imported {
		t.Fatalf("expected imported=false for invalid yaml")
	}
	if !strings.Contains(err.Error(), "validate local config for migration") {
		t.Fatalf("unexpected error: %v", err)
	}
}
