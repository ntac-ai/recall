# Recall

Recall automatically builds personalized [agent skill files](https://agentskills.io/home) from the chat history with your agents.

## Requirements

- Go 1.26+

## Build and run

Build the `recalld` binary:

```console
mkdir -p bin
go build -o bin/recalld ./cmd/recalld
```

Start it with the defaults:

```console
./bin/recalld
```

The default address is port `22550` on all local network interfaces, and the default data directory is `$HOME/.config/recall`.

Both settings can be changed:

```console
./bin/recalld --port=3000 --data-dir=/absolute/path/to/recall-data
```

## Development

Format and test the project with standard Go commands:

```console
gofmt -w ./cmd
go test ./...
go vet ./...
```
