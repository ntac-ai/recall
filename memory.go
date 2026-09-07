package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type memory struct {
	ID      string `json:"id"`
	Created int64  `json:"created"`
	Project string `json:"project"`
	Agent   string `json:"agent"`
	Model   string `json:"model"`
	Text    string `json:"memory"`
	SHA256  string `json:"sha256"`
}

func (m memory) filename() string {
	return fmt.Sprintf("%d_%s.json", m.Created, m.SHA256[:8])
}

func hashMemory(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func validateText(label, value string) error {
	if !utf8.ValidString(value) || strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must be nonempty UTF-8 text", label)
	}
	return nil
}

func saveMemory(ctx context.Context, dataDir, project, text, agent, model string) (string, error) {
	name, err := normalizeName(project)
	if err != nil {
		return "", fmt.Errorf("project: %w", err)
	}
	for _, field := range []struct{ label, value string }{
		{"project", project}, {"memory", text}, {"agent", agent}, {"model", model},
	} {
		if err := validateText(field.label, field.value); err != nil {
			return "", err
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	dir, err := openProjectDir(dataDir, name, true)
	if err != nil {
		return "", err
	}
	defer dir.Close()
	m := memory{Project: project, Agent: agent, Model: model, Text: text, SHA256: hashMemory(text)}
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		id, err := uuid.NewV7()
		if err != nil {
			return "", fmt.Errorf("generate memory ID: %w", err)
		}
		m.ID, m.Created = id.String(), time.Now().UnixMicro()
		data, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			return "", fmt.Errorf("encode memory: %w", err)
		}
		err = writeFile(dir, m.filename(), append(data, '\n'), false)
		if errors.Is(err, fs.ErrExist) {
			// Another process saved the same hash prefix in this microsecond.
			// Obtain a fresh timestamp without changing the required filename format.
			continue
		}
		if err != nil {
			return "", fmt.Errorf("save memory: %w", err)
		}
		return filepath.Join(dataDir, name, m.filename()), nil
	}
}

func (m memory) validate(projectName, filename string) error {
	id, err := uuid.Parse(m.ID)
	// Parse also accepts URNs and unhyphenated UUIDs; memory files use the
	// canonical 36-character form and require version 7 with the RFC variant.
	if err != nil || len(m.ID) != 36 || id.Version() != 7 || id.Variant() != uuid.RFC4122 || m.Created <= 0 {
		return fmt.Errorf("invalid UUID v7 or creation timestamp")
	}
	name, err := normalizeName(m.Project)
	if err != nil || name != projectName {
		return fmt.Errorf("memory belongs to a different or invalid project")
	}
	for _, field := range []struct{ label, value string }{
		{"project", m.Project}, {"memory", m.Text}, {"agent", m.Agent}, {"model", m.Model},
	} {
		if err := validateText(field.label, field.value); err != nil {
			return err
		}
	}
	if m.SHA256 != hashMemory(m.Text) {
		return fmt.Errorf("memory SHA256 does not match its text")
	}
	if filename != m.filename() {
		return fmt.Errorf("filename does not match the creation timestamp and SHA256")
	}
	return nil
}
