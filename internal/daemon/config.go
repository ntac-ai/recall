package daemon

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// DefaultPort is Recall's conventional HTTP port.
const DefaultPort = 22550

// Config contains recalld's runtime configuration.
type Config struct {
	Port    int
	DataDir string
}

// ParseConfig parses recalld's supported command-line flags.
func ParseConfig(args []string, output io.Writer) (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("find home directory: %w", err)
	}

	config := Config{
		Port:    DefaultPort,
		DataDir: filepath.Join(home, ".config", "recall"),
	}
	flags := flag.NewFlagSet("recalld", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.IntVar(&config.Port, "port", config.Port, "HTTP port to listen on")
	flags.StringVar(&config.DataDir, "data-dir", config.DataDir, "directory containing recall.sqlite3")
	if err := flags.Parse(args); err != nil {
		return Config{}, err
	}
	if flags.NArg() != 0 {
		return Config{}, errors.New("recalld does not accept positional arguments")
	}
	if config.Port < 1 || config.Port > 65535 {
		return Config{}, fmt.Errorf("port must be between 1 and 65535, got %d", config.Port)
	}
	if config.DataDir == "" {
		return Config{}, errors.New("data-dir must not be empty")
	}
	config.DataDir, err = filepath.Abs(config.DataDir)
	if err != nil {
		return Config{}, fmt.Errorf("resolve data-dir: %w", err)
	}
	return config, nil
}
