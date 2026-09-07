# Recall

Recall is a command line tool that automatically builds personalized [agent skill files](https://agentskills.io/home) from the chat history with your agents.

## Project Goals

The goal of this project is to build a simple Go command line application that can be installed manually or through Homebrew. The application, named `recall`, will log memories during chat conversations with AI agents that can eventually be compiled into `SKILL.md` files.

## Usage

See `README.md` to review the basic command line interface.

### Memory Files

Each memory should be saved to a small JSON file with the following format:

```json
{
    "id": "01a07a23-b612-7668-af22-9a9ec4206c68",
    "created": 1788755623219186,
    "project": "FieldSheet",
    "agent": "Codex",
    "model": "gpt-6-astra",
    "memory": "Reminder to always use small DTOs when handling API request data."
    "sha256": "78946eeccf68284daa0c31c3a1b25e0266d58a2a2eef04305b6178ae8aa2def5"
}
```

The files should be located, by default, in `$HOME/.config/recall/<project-name>/1788755623219186_78946ee.json`.

The `<project-name>` directory should be normalized such that uncommon characters (spaces, non-ASCII, etc) are replaced with hyphens.

#### Memory File Layout

- `id`: UUID v7
- `created`: Unix epoch in microseconds
- `agent`: Name of the agent logging the memory
- `model`: Name of the model used during the chat session
- `memory`: Text of the memory iteself
- `sha256`: SHA256 hash of the memory text

The memory file is named with the format: `<created>_<sha256[0:8]>.json`. That is, the `created` timestamp in microseconds and the first 8 characters of the SHA256 hash.

## Snapshots

When a snapshot is made, the agent should take all memory files written for that project to date and create or update a `SKILL.md` file in `$HOME/.agents/skills/<project-name>/SKILL.md`.
