package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
)

// version is set by GoReleaser. go install builds use module build information.
var version = "dev"

const usage = `Recall saves project memories and compiles them into agent skills.

Usage:
  recall save [options] <project-name> <memory>
  recall snapshot [options] <project-name> <skill-name>
  recall help [command]
  recall --version

Use "recall help save" or "recall help snapshot" for options.
Place options before positional arguments. Use - as the memory to read stdin.
`

func run(ctx context.Context, args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 0 {
		_, err := io.WriteString(stdout, usage)
		return err
	}
	switch args[0] {
	case "help", "-h", "--help":
		if len(args) == 1 {
			_, err := io.WriteString(stdout, usage)
			return err
		}
		if args[0] == "help" && len(args) == 2 && (args[1] == "save" || args[1] == "snapshot") {
			return run(ctx, []string{args[1], "--help"}, stdin, stdout)
		}
		return fmt.Errorf("usage: recall help [save|snapshot]")
	case "--version":
		if len(args) != 1 {
			return fmt.Errorf("usage: recall --version")
		}
		_, err := fmt.Fprintln(stdout, "recall", buildVersion())
		return err
	case "save", "snapshot":
	default:
		return fmt.Errorf("unknown command %q; run 'recall --help'", args[0])
	}

	command := args[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	var help bytes.Buffer
	flags.SetOutput(&help)
	dataDir := flags.String("data-dir", os.Getenv("RECALL_DATA_DIR"), "memory root (RECALL_DATA_DIR; default ~/.config/recall)")
	var agent, model, skillsDir string
	if command == "save" {
		flags.StringVar(&agent, "agent", envOr("RECALL_AGENT", "unknown"), "agent name (RECALL_AGENT)")
		flags.StringVar(&model, "model", envOr("RECALL_MODEL", "unknown"), "model name (RECALL_MODEL)")
	} else {
		flags.StringVar(&skillsDir, "skills-dir", os.Getenv("RECALL_SKILLS_DIR"), "skill root (RECALL_SKILLS_DIR; default ~/.agents/skills)")
	}
	flags.Usage = func() {
		if command == "save" {
			fmt.Fprintln(flags.Output(), "Usage: recall save [options] <project-name> <memory>")
			fmt.Fprintln(flags.Output(), "Use - as the memory to read stdin. Memory text is preserved exactly.")
		} else {
			fmt.Fprintln(flags.Output(), "Usage: recall snapshot [options] <project-name> <skill-name>")
			fmt.Fprintln(flags.Output(), "Compile all project memories into <skills-dir>/<project-name>/SKILL.md.")
		}
		fmt.Fprintln(flags.Output(), "\nOptions (before positional arguments):")
		flags.PrintDefaults()
	}
	// Only main reports errors. Buffer flag's automatic diagnostics to avoid
	// printing the same error twice, and send successful help to stdout.
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, err := io.Copy(stdout, &help)
			return err
		}
		return fmt.Errorf("%w; run 'recall help %s'", err, command)
	}
	if flags.NArg() != 2 {
		return fmt.Errorf("%s requires exactly two arguments; run 'recall help %s'", command, command)
	}
	if *dataDir == "" {
		var err error
		*dataDir, err = homePath(".config", "recall")
		if err != nil {
			return err
		}
	}
	var path string
	var err error
	if command == "save" {
		text := flags.Arg(1)
		if text == "-" {
			data, readErr := io.ReadAll(stdin)
			if err := ctx.Err(); err != nil {
				return err
			}
			if readErr != nil {
				return fmt.Errorf("read memory from stdin: %w", readErr)
			}
			text = string(data)
		}
		path, err = saveMemory(ctx, *dataDir, flags.Arg(0), text, agent, model)
	} else {
		if skillsDir == "" {
			skillsDir, err = homePath(".agents", "skills")
			if err != nil {
				return err
			}
		}
		path, err = snapshot(ctx, *dataDir, skillsDir, flags.Arg(0), flags.Arg(1))
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(stdout, path)
	return err
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func homePath(parts ...string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory (or set directory flags): %w", err)
	}
	return filepath.Join(append([]string{home}, parts...)...), nil
}

func buildVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}
