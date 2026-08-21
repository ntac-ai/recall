# Recall

Recall is a small local service that lets AI agents save project-specific facts,
developer preferences, and other useful memories. The `recalld` daemon exposes a
JSON REST API and persists its data in a single SQLite database.

## Requirements

- Go 1.26 or newer

No additional global tools are required. Recall uses a pure-Go SQLite driver, so
it does not need CGO, a C compiler, or a separately installed SQLite library. It
uses [Goose](https://github.com/pressly/goose) as an embedded migration library;
the Goose CLI is not required to run or develop the service.

## Build and run

Build the `recalld` binary:

```sh
mkdir -p bin
go build -o bin/recalld ./cmd/recalld
```

Start it with the defaults:

```sh
./bin/recalld
```

The default address is port `22550` on all local network interfaces, and the
default data directory is `$HOME/.config/recall`. The database is stored at
`$HOME/.config/recall/recall.sqlite3`.

Both settings can be changed:

```sh
./bin/recalld --port=3000 --data-dir=/absolute/path/to/recall-data
```

At startup, `recalld`:

1. Reserves the requested TCP port.
2. Creates the data directory when needed and verifies that it is writable.
3. Creates `recall.sqlite3` when needed.
4. Applies any pending embedded database migrations.
5. Starts serving HTTP requests.

Send `SIGINT` (usually Control-C) or `SIGTERM` to shut the server down gracefully.

## REST API

The API is unauthenticated and intended for trusted environments. All responses,
including errors, use `Content-Type: application/json`.

Requests may omit `Content-Type`; an omitted value is treated as
`application/json`. Any supplied value must be `application/json` (optional media
type parameters such as `charset=utf-8` are accepted), or the server returns
`415 Unsupported Media Type`.

An omitted `Accept` header is accepted. A supplied `Accept` header must allow
either `application/json` or `*/*`, or the server returns `406 Not Acceptable`.

Timestamps are Unix timestamps in whole seconds. Nullable database fields are
represented as either a JSON string or `null`.

### Create a project

`POST /projects` returns `201 Created`. The project path must be absolute.

```sh
curl --request POST http://localhost:22550/projects \
  --header 'Content-Type: application/json' \
  --data '{
    "name": "Recall",
    "summary": "A local memory service for agents",
    "path": "/Users/example/Developer/recall"
  }'
```

Example response:

```json
{
  "id": 1,
  "created_at": 1787263200,
  "updated_at": 1787263200,
  "name": "Recall",
  "summary": "A local memory service for agents",
  "path": "/Users/example/Developer/recall"
}
```

The response also includes `Location: /projects/1`.

### Read all projects

`GET /projects` returns `200 OK` and a JSON array. An empty database returns `[]`.

```sh
curl http://localhost:22550/projects
```

### Read one project

`GET /projects/{projectId}` returns `200 OK` or `404 Not Found`.

```sh
curl http://localhost:22550/projects/1
```

### Create a memory

`POST /projects/{projectId}/memories` returns `201 Created`, or `404 Not Found`
when the project does not exist. `agent` and `model` are optional.

```sh
curl --request POST http://localhost:22550/projects/1/memories \
  --header 'Content-Type: application/json' \
  --data '{
    "name": "Formatting preference",
    "memory": "Run gofmt on all Go files before committing.",
    "agent": "Codex",
    "model": "GPT-5"
  }'
```

Example response:

```json
{
  "id": 1,
  "created_at": 1787263260,
  "updated_at": 1787263260,
  "project_id": 1,
  "name": "Formatting preference",
  "memory": "Run gofmt on all Go files before committing.",
  "agent": "Codex",
  "model": "GPT-5"
}
```

The response also includes `Location: /projects/1/memories/1`.

### Read one memory

`GET /projects/{projectId}/memories/{memoryId}` returns the memory only when it
belongs to the specified project. It returns `404 Not Found` otherwise.

```sh
curl http://localhost:22550/projects/1/memories/1
```

### Errors

Errors use the requested HTTP status and a stable JSON shape:

```json
{
  "error": "project not found"
}
```

Malformed JSON and unknown request fields return `400 Bad Request`. Semantically
invalid fields, such as an empty name or relative project path, return
`422 Unprocessable Entity`.

## Database schema and migrations

The initial migration creates the singular `project` and `memory` tables. A
memory belongs to one project, and deleting a project cascades to its memories.
An index on `memory.project_id` supports project-scoped memory access.

Migration files live in [`internal/database/migrations`](internal/database/migrations)
and are embedded into the daemon at build time. To add a schema change, add the
next sequential Goose SQL file (for example, `00002_add_example.sql`) with both
`-- +goose Up` and `-- +goose Down` sections. Pending upward migrations run
automatically at startup.

## Development

Format and test the project with standard Go commands:

```sh
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
```

The tests use temporary SQLite databases and do not write to the normal Recall
data directory.
