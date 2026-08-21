package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestOpenAppliesMigrationsAndCreatesPrivateDatabase(t *testing.T) {
	dataDir := filepath.Join(t.TempDir(), "data with spaces")
	if err := os.Mkdir(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := DBPath(dataDir)
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, table := range []string{"project", "memory", "goose_db_version"} {
		var count int
		err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count)
		if err != nil {
			t.Fatalf("look up table %q: %v", table, err)
		}
		if count != 1 {
			t.Errorf("table %q count = %d, want 1", table, count)
		}
	}

	var foreignKeys int
	if err := db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys pragma: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1", foreignKeys)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("database permissions = %o, want 600", got)
		}
	}
}

func TestStoreProjectAndMemoryLifecycle(t *testing.T) {
	db, err := Open(context.Background(), DBPath(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := NewStore(db)
	store.now = func() time.Time { return time.Unix(1_800_000_000, 0) }
	summary := "A daemon for agent memories"
	project, err := store.CreateProject(context.Background(), NewProject{
		Name:    "Recall",
		Summary: &summary,
		Path:    "/work/recall",
	})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if project.ID != 1 || project.CreatedAt != 1_800_000_000 || project.UpdatedAt != 1_800_000_000 {
		t.Fatalf("CreateProject() = %#v", project)
	}
	if project.Summary == nil || *project.Summary != summary {
		t.Errorf("project summary = %#v, want %q", project.Summary, summary)
	}

	projects, err := store.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if len(projects) != 1 || projects[0].ID != project.ID {
		t.Fatalf("ListProjects() = %#v", projects)
	}

	agent := "Codex"
	memory, err := store.CreateMemory(context.Background(), project.ID, NewMemory{
		Name:   "Go formatting",
		Memory: "Run gofmt before committing.",
		Agent:  &agent,
	})
	if err != nil {
		t.Fatalf("CreateMemory() error = %v", err)
	}
	if memory.ProjectID != project.ID || memory.Agent == nil || *memory.Agent != agent || memory.Model != nil {
		t.Fatalf("CreateMemory() = %#v", memory)
	}

	gotMemory, err := store.GetMemory(context.Background(), project.ID, memory.ID)
	if err != nil {
		t.Fatalf("GetMemory() error = %v", err)
	}
	if gotMemory.Memory != memory.Memory {
		t.Errorf("GetMemory().Memory = %q, want %q", gotMemory.Memory, memory.Memory)
	}

	if _, err := store.CreateMemory(context.Background(), 999, NewMemory{Name: "missing", Memory: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("CreateMemory() missing project error = %v, want ErrNotFound", err)
	}
	if _, err := db.Exec(`DELETE FROM project WHERE id = ?`, project.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	if _, err := store.GetMemory(context.Background(), project.ID, memory.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetMemory() after cascade error = %v, want ErrNotFound", err)
	}
}

func TestListProjectsReturnsEmptyArray(t *testing.T) {
	db, err := Open(context.Background(), DBPath(t.TempDir()))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	projects, err := NewStore(db).ListProjects(context.Background())
	if err != nil {
		t.Fatalf("ListProjects() error = %v", err)
	}
	if projects == nil || len(projects) != 0 {
		t.Fatalf("ListProjects() = %#v, want non-nil empty slice", projects)
	}
}
