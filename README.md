# Recall

Recall automatically builds personalized [agent skill files](https://agentskills.io/home) from the chat history with your agents.

Agents call `recall save` to capture useful instructions during a conversation.
`recall snapshot` compiles those memories into a `SKILL.md` file that agents can
load in future sessions. The basic CLI runs locally, preserves the saved text,
and requires no model API or network connection. It does not monitor chat history;
your agent decides which memories to save and when to create a snapshot.

## Install

Build from a checkout with Go 1.25 or newer:

```shell
go build -trimpath -o bin/recall .
./bin/recall --help
```

Put the resulting `recall` binary in a directory on your `$PATH`. The binary
requires no separate runtime. To install directly from a checkout into your Go
binary directory (`GOBIN`, or `GOPATH/bin` when unset):

```shell
go install .
```

Once a version is published, `go install github.com/ntac-ai/recall@latest` will
also work. Homebrew distribution is planned; a tap has not been configured yet.

## Usage

### Save a Memory

`recall save [options] <project-name> <memory>`

#### Example

```shell
recall save fieldsheet 'Always use final readonly DTOs when handling API request data.'
```

Supply the agent and model explicitly or through environment variables. Recall
records `unknown` for either value when it is not supplied.

```shell
recall save --agent Codex --model gpt-6-astra fieldsheet 'Keep API request DTOs small.'
```

For multiline text, pass `-` as the memory and pipe the text through stdin:

```shell
printf '%s\n' 'Validate input before saving.' | recall save fieldsheet -
```

Memory text is stored exactly as supplied, including whitespace and newlines.
Empty or whitespace-only memories are rejected. A successful save prints the
JSON file path to stdout.

### Snapshot Memories Into a Skill

`recall snapshot [options] <project-name> <skill-name>`

#### Example

```shell
recall snapshot fieldsheet api-requests
```

This writes `~/.agents/skills/fieldsheet/SKILL.md`. As specified in `AGENTS.md`,
there is one generated skill per project: `skill-name` sets the Markdown title
(`api-requests` here), and the frontmatter name matches the project directory
(`fieldsheet`). It does not filter memories by topic.

Snapshots include all saved project memories, oldest first, with exact duplicate
texts included once. Multiline Markdown is preserved. The output has the `name`
and `description` frontmatter required by the
[Agent Skills specification](https://agentskills.io/specification).

Running the command again regenerates the entire skill from the current memory
files; manual edits to that `SKILL.md` are replaced. Memories remain available
afterward. Identical inputs produce identical output. If there are no memories,
or a memory has invalid JSON, metadata, a mismatched checksum, or an incorrect
filename, the command fails and leaves the previous skill untouched. Non-JSON
files, including in-progress saves, are ignored. Saves made after a snapshot
lists the directory are picked up on the next snapshot.

A successful snapshot prints its file path to stdout. Errors go to stderr and
return a nonzero exit status.

### Options and Environment Variables

Place flags **before** positional arguments, following Go's standard flag
conventions. Use `recall help save`, `recall help snapshot`, or `recall --version`.

| Flag | Environment variable | Default | Command |
| --- | --- | --- | --- |
| `--data-dir` | `RECALL_DATA_DIR` | `$HOME/.config/recall` | Both |
| `--skills-dir` | `RECALL_SKILLS_DIR` | `$HOME/.agents/skills` | Snapshot |
| `--agent` | `RECALL_AGENT` | `unknown` | Save |
| `--model` | `RECALL_MODEL` | `unknown` | Save |

Flags override environment variables, which override defaults. The default paths
are the same on macOS and Linux; Windows uses the user's home directory. Quote
custom paths containing spaces. Recall uses the explicit `.config/recall` default
on macOS too, rather than `Library/Application Support`.

### Memory Files

Memories are stored at
`~/.config/recall/<project-name>/<created>_<sha256[0:8]>.json`:

```json
{
  "id": "017f22e2-79b0-7cc3-98c4-dc0c0c07398f",
  "created": 1645557742000000,
  "project": "FieldSheet",
  "agent": "Codex",
  "model": "gpt-6-astra",
  "memory": "hello",
  "sha256": "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
}
```

This example's filename is `1645557742000000_2cf24dba.json`. IDs are UUID v7;
`created` is Unix time in microseconds; `sha256` hashes the exact UTF-8 memory
text. The original project spelling is retained in JSON.

Directory names are lowercased ASCII letters and numbers, with runs of spaces,
punctuation, separators, and non-ASCII characters replaced by a single hyphen.
Leading and trailing hyphens are removed: `Field Sheet` becomes `field-sheet`.
Names with no ASCII letters or numbers, or more than 64 normalized characters,
are rejected. Windows device names get a `-recall` suffix (`CON` becomes
`con-recall`). Names that normalize identically share a project, so choose
distinct normalized names for distinct projects.

New directories use permissions `0700` and files `0600` where supported.
Concurrent saves never replace an existing memory. Files are fully written and
synced before publication; memory storage requires a filesystem supporting hard
links (such as APFS, ext4, or NTFS). Snapshot replacement uses a same-directory
rename, atomic on Unix. Directory traversal and symlink escapes beneath the
configured roots are blocked using Go's `os.Root`.

### Agent Integration

Add instructions like these to the project instructions your agent already reads:

```text
When I give a durable project preference or correction, save a concise,
self-contained memory using:
recall save --agent <your-agent-name> --model <your-model-name> fieldsheet '<memory>'

At the end of the session, compile the saved preferences using:
recall snapshot fieldsheet project-preferences
```

Have the agent quote shell arguments correctly, or send multiline text through
stdin. Skill discovery depends on your agent supporting `~/.agents/skills`;
use `--skills-dir` if it reads a different directory. Snapshots compile recorded
instructions without summarizing, inferring new rules, or resolving conflicts.

## Development

The application uses only the Go standard library and keeps command parsing,
memory storage, and skill generation in separate files within a single package.
Go 1.25 is the minimum because storage uses the extended `os.Root` APIs.

```shell
go mod tidy
go vet ./...
go test -race ./...
CGO_ENABLED=0 go build -trimpath -o bin/recall .
```

Tests use temporary directories. CI runs on Linux, macOS, and Windows, using
both Go 1.25 and the current stable toolchain.

The included [GoReleaser](https://goreleaser.com/) configuration builds runtime-free
archives for macOS, Linux, and Windows on amd64 and arm64, with checksums and
build-time version information. With GoReleaser v2.6 or newer installed, validate
the configuration and create local release artifacts without publishing:

```shell
goreleaser check
goreleaser release --snapshot --clean
```

Artifacts appear in `dist/`. Publishing releases and configuring a Homebrew tap
are separate distribution steps.

## Credits

- [Vic Cherubini](https://github.com/viccherubini), [1:N Labs, LLC dba North Texas Automation Company](https://ntac.ai)

## License

The MIT License
