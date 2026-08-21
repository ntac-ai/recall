package daemon

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	config, err := ParseConfig(nil, io.Discard)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if config.Port != DefaultPort {
		t.Errorf("default port = %d, want %d", config.Port, DefaultPort)
	}
	if want := filepath.Join(home, ".config", "recall"); config.DataDir != want {
		t.Errorf("default data dir = %q, want %q", config.DataDir, want)
	}

	config, err = ParseConfig([]string{"--port=3000", "--data-dir=var/recall"}, io.Discard)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if config.Port != 3000 || !filepath.IsAbs(config.DataDir) {
		t.Errorf("custom config = %#v", config)
	}
}

func TestParseConfigRejectsInvalidArguments(t *testing.T) {
	tests := [][]string{
		{"--port=0"},
		{"--port=65536"},
		{"--data-dir="},
		{"positional"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			var output bytes.Buffer
			if _, err := ParseConfig(args, &output); err == nil {
				t.Fatalf("ParseConfig(%q) error = nil", args)
			}
		})
	}
}

func TestEnsureWritableDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "recall")
	if err := ensureWritableDirectory(path); err != nil {
		t.Fatalf("ensureWritableDirectory() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		t.Fatalf("%q was not created as a directory", path)
	}

	filePath := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(filePath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureWritableDirectory(filePath); err == nil {
		t.Fatal("ensureWritableDirectory() with file error = nil")
	}
}

func TestRunChecksPortBeforeCreatingDataDirectory(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "must-not-exist")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	listen := func(_, _ string) (net.Listener, error) {
		return nil, errors.New("address already in use")
	}
	err := run(context.Background(), Config{Port: 22550, DataDir: dataDir}, logger, listen)
	if err == nil || !strings.Contains(err.Error(), "bind to port") {
		t.Fatalf("Run() error = %v, want bind error", err)
	}
	if _, err := os.Stat(dataDir); !os.IsNotExist(err) {
		t.Fatalf("data directory was created before bind check; Stat error = %v", err)
	}
}
