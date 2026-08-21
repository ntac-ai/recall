// Package database owns Recall's SQLite connection, schema migrations, and
// persistence operations.
package database

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

const filename = "recall.sqlite3"

// ErrNotFound is returned when the requested project or memory does not exist.
var ErrNotFound = errors.New("not found")

//go:embed migrations/*.sql
var migrationFiles embed.FS

// DBPath returns the conventional database path within dataDir.
func DBPath(dataDir string) string {
	return filepath.Join(dataDir, filename)
}

// Open opens (or creates) a SQLite database and applies all embedded migrations.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	_, statErr := os.Stat(path)
	newFile := errors.Is(statErr, os.ErrNotExist)
	if statErr != nil && !newFile {
		return nil, fmt.Errorf("inspect database file: %w", statErr)
	}

	dsn := sqliteDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	closeOnError := func(err error) (*sql.DB, error) {
		_ = db.Close()
		return nil, err
	}

	// A small pool allows concurrent readers while SQLite's busy timeout and WAL
	// mode serialize brief write transactions.
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	if err := db.PingContext(ctx); err != nil {
		return closeOnError(fmt.Errorf("connect to database: %w", err))
	}
	if newFile {
		if err := os.Chmod(path, 0o600); err != nil {
			return closeOnError(fmt.Errorf("secure database file: %w", err))
		}
	}

	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return closeOnError(fmt.Errorf("load migrations: %w", err))
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations)
	if err != nil {
		return closeOnError(fmt.Errorf("initialize migrations: %w", err))
	}
	if _, err := provider.Up(ctx); err != nil {
		return closeOnError(fmt.Errorf("apply migrations: %w", err))
	}

	return db, nil
}

func sqliteDSN(path string) string {
	query := make(url.Values)
	query.Add("_pragma", "busy_timeout(5000)")
	query.Add("_pragma", "foreign_keys(1)")
	query.Add("_pragma", "journal_mode(WAL)")

	return (&url.URL{
		Scheme:   "file",
		Path:     path,
		RawQuery: query.Encode(),
	}).String()
}

// Store persists and retrieves Recall's domain objects.
type Store struct {
	db  *sql.DB
	now func() time.Time
}

// NewStore constructs a Store backed by db.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db, now: time.Now}
}

// Project is a project returned by the API.
type Project struct {
	ID        int64   `json:"id"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
	Name      string  `json:"name"`
	Summary   *string `json:"summary"`
	Path      string  `json:"path"`
}

// NewProject contains the caller-supplied fields used to create a project.
type NewProject struct {
	Name    string
	Summary *string
	Path    string
}

// Memory is a memory returned by the API.
type Memory struct {
	ID        int64   `json:"id"`
	CreatedAt int64   `json:"created_at"`
	UpdatedAt int64   `json:"updated_at"`
	ProjectID int64   `json:"project_id"`
	Name      string  `json:"name"`
	Memory    string  `json:"memory"`
	Agent     *string `json:"agent"`
	Model     *string `json:"model"`
}

// NewMemory contains the caller-supplied fields used to create a memory.
type NewMemory struct {
	Name   string
	Memory string
	Agent  *string
	Model  *string
}

// ListProjects returns every project ordered by its identifier.
func (s *Store) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, created_at, updated_at, name, summary, path
		FROM project
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]Project, 0)
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return nil, fmt.Errorf("read project: %w", err)
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	return projects, nil
}

// GetProject returns one project by identifier.
func (s *Store) GetProject(ctx context.Context, id int64) (Project, error) {
	project, err := scanProject(s.db.QueryRowContext(ctx, `
		SELECT id, created_at, updated_at, name, summary, path
		FROM project
		WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, fmt.Errorf("get project: %w", err)
	}
	return project, nil
}

// CreateProject inserts and returns a project.
func (s *Store) CreateProject(ctx context.Context, input NewProject) (Project, error) {
	now := s.now().UTC().Unix()
	project, err := scanProject(s.db.QueryRowContext(ctx, `
		INSERT INTO project (created_at, updated_at, name, summary, path)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, created_at, updated_at, name, summary, path`,
		now, now, input.Name, input.Summary, input.Path))
	if err != nil {
		return Project{}, fmt.Errorf("create project: %w", err)
	}
	return project, nil
}

// CreateMemory inserts and returns a memory when projectID exists.
func (s *Store) CreateMemory(ctx context.Context, projectID int64, input NewMemory) (Memory, error) {
	now := s.now().UTC().Unix()
	memory, err := scanMemory(s.db.QueryRowContext(ctx, `
		INSERT INTO memory (created_at, updated_at, project_id, name, memory, agent, model)
		SELECT ?, ?, id, ?, ?, ?, ?
		FROM project
		WHERE id = ?
		RETURNING id, created_at, updated_at, project_id, name, memory, agent, model`,
		now, now, input.Name, input.Memory, input.Agent, input.Model, projectID))
	if errors.Is(err, sql.ErrNoRows) {
		return Memory{}, ErrNotFound
	}
	if err != nil {
		return Memory{}, fmt.Errorf("create memory: %w", err)
	}
	return memory, nil
}

// GetMemory returns a memory only when it belongs to projectID.
func (s *Store) GetMemory(ctx context.Context, projectID, memoryID int64) (Memory, error) {
	memory, err := scanMemory(s.db.QueryRowContext(ctx, `
		SELECT id, created_at, updated_at, project_id, name, memory, agent, model
		FROM memory
		WHERE project_id = ? AND id = ?`, projectID, memoryID))
	if errors.Is(err, sql.ErrNoRows) {
		return Memory{}, ErrNotFound
	}
	if err != nil {
		return Memory{}, fmt.Errorf("get memory: %w", err)
	}
	return memory, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProject(row scanner) (Project, error) {
	var project Project
	var summary sql.NullString
	err := row.Scan(
		&project.ID,
		&project.CreatedAt,
		&project.UpdatedAt,
		&project.Name,
		&summary,
		&project.Path,
	)
	project.Summary = optionalString(summary)
	return project, err
}

func scanMemory(row scanner) (Memory, error) {
	var memory Memory
	var agent, model sql.NullString
	err := row.Scan(
		&memory.ID,
		&memory.CreatedAt,
		&memory.UpdatedAt,
		&memory.ProjectID,
		&memory.Name,
		&memory.Memory,
		&agent,
		&model,
	)
	memory.Agent = optionalString(agent)
	memory.Model = optionalString(model)
	return memory, err
}

func optionalString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
