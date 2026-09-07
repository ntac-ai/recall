package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFilePublication(t *testing.T) {
	path := t.TempDir()
	dir, err := os.OpenRoot(path)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	if err := writeFile(dir, "memory.json", []byte("original"), false); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(dir, "memory.json", []byte("replacement"), false); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("expected collision, got %v", err)
	}
	if got := string(readFile(t, filepath.Join(path, "memory.json"))); got != "original" {
		t.Fatalf("clobbered memory: %q", got)
	}
	if err := writeFile(dir, "memory.json", []byte("replacement"), true); err != nil {
		t.Fatal(err)
	}
	if got := string(readFile(t, filepath.Join(path, "memory.json"))); got != "replacement" {
		t.Fatalf("did not replace snapshot: %q", got)
	}
	if err := dir.Mkdir("blocked", 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(dir, "blocked", []byte("replacement"), true); err == nil {
		t.Fatal("replaced a directory")
	}
	entries, err := os.ReadDir(path)
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary files left after collision or failure: %v, %v", entries, err)
	}
}

func TestProjectSymlinkCannotEscapeRoot(t *testing.T) {
	base, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(base, "project")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := saveMemory(t.Context(), base, "project", "text", "agent", "model"); err == nil {
		t.Fatal("saved through a project symlink outside the data root")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("modified outside directory: %v, %v", entries, err)
	}
}
