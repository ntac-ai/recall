package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestSnapshot(t *testing.T) {
	dataDir, skillsDir := t.TempDir(), t.TempDir()
	first := "Keep requests small.\n\n```go\ntype Request struct {\n\tName string\n}\n```"
	second := "Validate input before saving."
	// Write in reverse order to verify ordering by timestamps, not insertion.
	writeMemoryFixture(t, dataDir, "FieldSheet", second, 2000)
	writeMemoryFixture(t, dataDir, "FieldSheet", first, 1000)
	writeMemoryFixture(t, dataDir, "FieldSheet", first, 3000)
	writeMemoryFixture(t, dataDir, "other", "Do not include this project.", 1000)
	if err := os.WriteFile(filepath.Join(dataDir, "fieldsheet", ".recall-pending.tmp"), []byte("unfinished"), 0o600); err != nil {
		t.Fatal(err)
	}
	path, err := snapshot(t.Context(), dataDir, skillsDir, "fieldsheet", "api-requests")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(skillsDir, "fieldsheet", "SKILL.md"); path != want {
		t.Fatalf("snapshot path = %q, want %q", path, want)
	}
	data := readFile(t, path)
	text := string(data)
	for _, want := range []string{"---\nname: \"fieldsheet\"\ndescription: ", "\n---\n\n# api-requests\n", first, second} {
		if !strings.Contains(text, want) {
			t.Errorf("snapshot missing %q:\n%s", want, text)
		}
	}
	if strings.Index(text, first) > strings.Index(text, second) || strings.Count(text, first) != 1 {
		t.Fatalf("memories are not chronological and deduplicated:\n%s", text)
	}
	if strings.Contains(text, "Do not include") || strings.Contains(text, "unfinished") {
		t.Fatalf("snapshot included unrelated data:\n%s", text)
	}
	if _, err := snapshot(t.Context(), dataDir, skillsDir, "fieldsheet", "api-requests"); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, readFile(t, path)) {
		t.Fatal("repeated snapshot changed without new memories")
	}
	writeMemoryFixture(t, dataDir, "fieldsheet", "New instruction.", 4000)
	if _, err := snapshot(t.Context(), dataDir, skillsDir, "fieldsheet", "updated-title"); err != nil {
		t.Fatal(err)
	}
	updated := string(readFile(t, path))
	if !strings.Contains(updated, first) || !strings.Contains(updated, "New instruction.") || !strings.Contains(updated, "# updated-title") {
		t.Fatalf("snapshot update lost or omitted memories:\n%s", updated)
	}
}

func TestSnapshotRejectsCorruptMemories(t *testing.T) {
	for _, tt := range []struct {
		name   string
		mutate func(*memory)
	}{
		{"hash mismatch", func(m *memory) { m.Text = "tampered" }},
		{"wrong project", func(m *memory) { m.Project = "different" }},
		{"invalid id", func(m *memory) { m.ID = "invalid" }},
		{"invalid timestamp", func(m *memory) { m.Created = 0 }},
		{"missing agent", func(m *memory) { m.Agent = "" }},
		{"missing model", func(m *memory) { m.Model = "" }},
		{"short hash", func(m *memory) { m.SHA256 = "abc" }},
		{"filename mismatch", func(m *memory) { m.Created++ }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dataDir, skillsDir := t.TempDir(), t.TempDir()
			memoryPath := writeMemoryFixture(t, dataDir, "project", "original", 1000)
			path, err := snapshot(t.Context(), dataDir, skillsDir, "project", "title")
			if err != nil {
				t.Fatal(err)
			}
			original := readFile(t, path)
			m := readMemory(t, memoryPath)
			tt.mutate(&m)
			data, err := json.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(memoryPath, data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := snapshot(t.Context(), dataDir, skillsDir, "project", "new title"); err == nil {
				t.Fatal("accepted a corrupt memory")
			}
			if !bytes.Equal(original, readFile(t, path)) {
				t.Fatal("failed snapshot modified the previous skill")
			}
		})
	}
}

func TestSnapshotRejectsInvalidJSON(t *testing.T) {
	dataDir := t.TempDir()
	path := writeMemoryFixture(t, dataDir, "project", "text", 1000)
	for _, data := range []string{"{", "null", "{}", "{}{}"} {
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := snapshot(t.Context(), dataDir, t.TempDir(), "project", "title"); err == nil {
			t.Fatalf("accepted invalid JSON: %q", data)
		}
	}
}

func TestEmptySnapshotDoesNotReplaceSkill(t *testing.T) {
	dataDir, skillsDir := t.TempDir(), t.TempDir()
	dir := filepath.Join(skillsDir, "project")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, createEmpty := range []bool{false, true} {
		if createEmpty {
			if err := os.Mkdir(filepath.Join(dataDir, "project"), 0o700); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := snapshot(t.Context(), dataDir, skillsDir, "project", "title"); err == nil {
			t.Fatal("snapshot with no memories succeeded")
		}
		if got := string(readFile(t, path)); got != "keep me" {
			t.Fatalf("empty snapshot replaced skill: %q", got)
		}
	}
}

func TestSnapshotRejectsSymlinks(t *testing.T) {
	dataDir, skillsDir, outside := t.TempDir(), t.TempDir(), t.TempDir()
	path := writeMemoryFixture(t, dataDir, "project", "original", 1000)
	if err := os.Symlink(outside, filepath.Join(skillsDir, "project")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := snapshot(t.Context(), dataDir, skillsDir, "project", "title"); err == nil {
		t.Fatal("wrote through skill directory symlink outside root")
	}
	if _, err := os.Stat(filepath.Join(outside, "SKILL.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("snapshot wrote outside root: %v", err)
	}
	if err := os.Symlink(path, filepath.Join(dataDir, "project", "linked.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := loadMemories(t.Context(), dataDir, "project"); err == nil {
		t.Fatal("accepted symlinked memory")
	}
}

func TestCanceledSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	writeMemoryFixture(t, dataDir, "project", "text", 1000)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	skillsDir := filepath.Join(t.TempDir(), "skills")
	if _, err := snapshot(ctx, dataDir, skillsDir, "project", "title"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if _, err := os.Stat(skillsDir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("canceled snapshot created files: %v", err)
	}
}

func writeMemoryFixture(t *testing.T, dataDir, project, text string, created int64) string {
	t.Helper()
	name, err := normalizeName(project)
	if err != nil {
		t.Fatal(err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	m := memory{ID: id.String(), Created: created, Project: project, Agent: "test", Model: "test", Text: text, SHA256: hashMemory(text)}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(dataDir, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, m.filename())
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
