package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNormalizeName(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"FieldSheet", "fieldsheet"},
		{"  My   Project!  ", "my-project"},
		{"café 東京 app", "caf-app"},
		{"../Outside\\another", "outside-another"},
		{"api---requests", "api-requests"},
		{"CON", "con-recall"},
		{"LPT9.", "lpt9-recall"},
		{"123", "123"},
		{strings.Repeat("a", 64), strings.Repeat("a", 64)},
		{"", ""}, {"---", ""}, {"東京", ""}, {strings.Repeat("a", 65), ""},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got, err := normalizeName(tt.input)
			if (err != nil) != (tt.want == "") || got != tt.want {
				t.Fatalf("normalizeName(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
			}
			if err == nil {
				if again, err := normalizeName(got); err != nil || again != got {
					t.Fatalf("normalization is not idempotent: %q, %v", again, err)
				}
			}
		})
	}
}

func TestMemoryUUIDValidation(t *testing.T) {
	for _, tt := range []struct {
		name, id string
		wantErr  bool
	}{
		{"existing v7", "017f22e2-79b0-7cc3-98c4-dc0c0c07398f", false},
		{"empty", "", true},
		{"version 4", "017f22e2-79b0-4cc3-98c4-dc0c0c07398f", true},
		{"wrong variant", "017f22e2-79b0-7cc3-78c4-dc0c0c07398f", true},
		{"invalid hex", "017f22e2-79b0-7cc3-98c4-dc0c0c07398z", true},
		{"unhyphenated", "017f22e279b07cc398c4dc0c0c07398f", true},
		{"URN", "urn:uuid:017f22e2-79b0-7cc3-98c4-dc0c0c07398f", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := memory{ID: tt.id, Created: 1645557742000000, Project: "project", Agent: "agent", Model: "model", Text: "hello", SHA256: hashMemory("hello")}
			if err := m.validate("project", m.filename()); (err != nil) != tt.wantErr {
				t.Fatalf("validate UUID %q = %v; want error %v", tt.id, err, tt.wantErr)
			}
		})
	}
}

func TestSaveMemory(t *testing.T) {
	dir := t.TempDir()
	text := "  Keep indentation.\n\n```go\n\treturn nil\n```\n"
	before := time.Now().UnixMicro()
	path, err := saveMemory(t.Context(), dir, "Field Sheet", text, "Codex", "test-model")
	if err != nil {
		t.Fatal(err)
	}
	m := readMemory(t, path)
	id, err := uuid.Parse(m.ID)
	if m.Created < before || m.Created > time.Now().UnixMicro() || err != nil || id.Version() != 7 || id.Variant() != uuid.RFC4122 {
		t.Fatalf("invalid identity: %+v", m)
	}
	if m.Project != "Field Sheet" || m.Text != text || m.Agent != "Codex" || m.Model != "test-model" {
		t.Fatalf("memory did not round trip: %+v", m)
	}
	if m.SHA256 != hashMemory(text) || filepath.Base(path) != m.filename() || filepath.Base(filepath.Dir(path)) != "field-sheet" {
		t.Fatalf("incorrect hash or path: %s, %s", m.SHA256, path)
	}
	if got := hashMemory("hello"); got != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("SHA256 = %s", got)
	}
	data := readFile(t, path)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 7 {
		t.Fatalf("unexpected JSON fields: %s", data)
	}
	if runtime.GOOS != "windows" {
		for path, want := range map[string]fs.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != want {
				t.Fatalf("permissions for %s: %v, %v; want %o", path, info, err, want)
			}
		}
	}
}

func TestSaveRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct{ project, text, agent, model string }{
		{"..", "text", "agent", "model"},
		{"project", " \n", "agent", "model"},
		{"project", "text", "", "model"},
		{"project", "text", "agent", ""},
		{"project", string([]byte{0xff}), "agent", "model"},
	} {
		t.Run(tt.project+tt.text+tt.agent+tt.model, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "memories")
			if _, err := saveMemory(t.Context(), dir, tt.project, tt.text, tt.agent, tt.model); err == nil {
				t.Fatal("expected validation error")
			}
			if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("validation created files: %v", err)
			}
		})
	}
}

func TestConcurrentSaves(t *testing.T) {
	dir := t.TempDir()
	type result struct {
		path string
		err  error
	}
	results := make(chan result, 64)
	var wg sync.WaitGroup
	for range cap(results) {
		wg.Go(func() {
			path, err := saveMemory(t.Context(), dir, "project", "same memory", "agent", "model")
			results <- result{path, err}
		})
	}
	wg.Wait()
	close(results)
	seenPaths, seenIDs := make(map[string]bool), make(map[string]bool)
	for result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		m := readMemory(t, result.path)
		if seenPaths[result.path] || seenIDs[m.ID] {
			t.Fatalf("save overwrote an existing memory: %s", result.path)
		}
		seenPaths[result.path], seenIDs[m.ID] = true, true
	}
	entries, err := os.ReadDir(filepath.Join(dir, "project"))
	if err != nil || len(entries) != cap(results) {
		t.Fatalf("unexpected files or leftover temporary files: %d, %v", len(entries), err)
	}
}

func TestCanceledSave(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	dir := filepath.Join(t.TempDir(), "memories")
	if _, err := saveMemory(ctx, dir, "project", "text", "agent", "model"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("canceled save created files: %v", err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readMemory(t *testing.T, path string) memory {
	t.Helper()
	var m memory
	if err := json.Unmarshal(readFile(t, path), &m); err != nil {
		t.Fatal(err)
	}
	return m
}
