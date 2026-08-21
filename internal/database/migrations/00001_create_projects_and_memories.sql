-- +goose Up
CREATE TABLE project (
    id         INTEGER PRIMARY KEY,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    name       TEXT NOT NULL,
    summary    TEXT,
    path       TEXT NOT NULL
);

CREATE TABLE memory (
    id         INTEGER PRIMARY KEY,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    project_id INTEGER NOT NULL,
    name       TEXT NOT NULL,
    memory     TEXT NOT NULL,
    agent      TEXT,
    model      TEXT,
    FOREIGN KEY (project_id) REFERENCES project (id) ON DELETE CASCADE
);

CREATE INDEX memory_project_id_idx ON memory (project_id);

-- +goose Down
DROP TABLE memory;
DROP TABLE project;
