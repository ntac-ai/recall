# Recall

Recall automatically builds personalized [agent skill files](https://agentskills.io/home) from the chat history with your agents.

## Install

Install the `recall` CLI in your `$PATH`.

## Usage

### Save a Memory

`recall save <project-name> <memory>`

#### Example

```shell
recall save fieldsheet 'Always use final readonly DTOs when handling API request data.'
```

### Snapshot Memories Into a Skill

`recall snapshot <project-name> <skill-name>`

#### Example

```shell
recall snapshot fieldsheet api-requests
```

## Credits

- [Vic Cherubini](https://github.com/viccherubini), [1:N Labs, LLC dba North Texas Automation Company](https://ntac.ai)

## License

The MIT License
