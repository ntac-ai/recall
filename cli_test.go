package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIHelpAndErrors(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
		err  bool
	}{
		{"root", nil, "recall save", false},
		{"help flag", []string{"--help"}, "recall snapshot", false},
		{"save help", []string{"help", "save"}, "-agent", false},
		{"snapshot help", []string{"snapshot", "--help"}, "-skills-dir", false},
		{"version", []string{"--version"}, "recall ", false},
		{"unknown command", []string{"nope"}, "", true},
		{"unknown help", []string{"help", "nope"}, "", true},
		{"extra help arg", []string{"--help", "extra"}, "", true},
		{"extra version arg", []string{"--version", "extra"}, "", true},
		{"missing save args", []string{"save"}, "", true},
		{"missing snapshot args", []string{"snapshot", "project"}, "", true},
		{"extra arg", []string{"save", "project", "memory", "extra"}, "", true},
		{"unknown flag", []string{"save", "--bogus"}, "", true},
		{"missing flag value", []string{"save", "--agent"}, "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := run(t.Context(), tt.args, strings.NewReader(""), &out)
			if (err != nil) != tt.err {
				t.Fatalf("run = %v; want error %v", err, tt.err)
			}
			if tt.err && out.Len() != 0 {
				t.Fatalf("error leaked onto stdout: %q", out.String())
			}
			if !strings.Contains(out.String(), tt.want) {
				t.Fatalf("output %q does not contain %q", out.String(), tt.want)
			}
		})
	}
}

func TestCLIWorkflowAndConfiguration(t *testing.T) {
	dataDir, skillsDir := t.TempDir(), t.TempDir()
	t.Setenv("RECALL_DATA_DIR", dataDir)
	t.Setenv("RECALL_SKILLS_DIR", skillsDir)
	t.Setenv("RECALL_AGENT", "environment-agent")
	t.Setenv("RECALL_MODEL", "environment-model")
	var out bytes.Buffer
	text := "Multiline memory.\n\nPreserve this trailing newline.\n"
	if err := run(t.Context(), []string{"save", "FieldSheet", "-"}, strings.NewReader(text), &out); err != nil {
		t.Fatal(err)
	}
	m := readMemory(t, strings.TrimSpace(out.String()))
	if m.Text != text || m.Agent != "environment-agent" || m.Model != "environment-model" {
		t.Fatalf("stdin or environment metadata not preserved: %+v", m)
	}
	out.Reset()
	if err := run(t.Context(), []string{"snapshot", "FieldSheet", "api-requests"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	path := strings.TrimSpace(out.String())
	if path != filepath.Join(skillsDir, "fieldsheet", "SKILL.md") || !strings.Contains(string(readFile(t, path)), text) {
		t.Fatalf("incorrect snapshot: %q", path)
	}
	flagDir := t.TempDir()
	out.Reset()
	if err := run(t.Context(), []string{"save", "--data-dir", flagDir, "--agent", "flag-agent", "--model", "flag-model", "project", "text"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	path = strings.TrimSpace(out.String())
	m = readMemory(t, path)
	if filepath.Dir(filepath.Dir(path)) != flagDir || m.Agent != "flag-agent" || m.Model != "flag-model" {
		t.Fatalf("flags did not override environment: %s, %+v", path, m)
	}
	out.Reset()
	flagSkills := t.TempDir()
	if err := run(t.Context(), []string{"snapshot", "--data-dir", flagDir, "--skills-dir", flagSkills, "project", "title"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != filepath.Join(flagSkills, "project", "SKILL.md") {
		t.Fatalf("skill directory flag did not override environment: %s", got)
	}
}

func TestHomePaths(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}
	for _, parts := range [][]string{{".config", "recall"}, {".agents", "skills"}} {
		got, err := homePath(parts...)
		if want := filepath.Join(home, parts[0], parts[1]); err != nil || got != want {
			t.Fatalf("homePath = %q, %v; want %q", got, err, want)
		}
	}
}

func TestCLIDefaultMetadata(t *testing.T) {
	t.Setenv("RECALL_AGENT", "")
	t.Setenv("RECALL_MODEL", "")
	var out bytes.Buffer
	if err := run(t.Context(), []string{"save", "--data-dir", t.TempDir(), "project", "text"}, strings.NewReader(""), &out); err != nil {
		t.Fatal(err)
	}
	m := readMemory(t, strings.TrimSpace(out.String()))
	if m.Agent != "unknown" || m.Model != "unknown" {
		t.Fatalf("metadata was guessed: %+v", m)
	}
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestCLIStdinErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RECALL_DATA_DIR", dir)
	failure := errors.New("input failed")
	err := run(t.Context(), []string{"save", "project", "-"}, failingReader{failure}, io.Discard)
	if !errors.Is(err, failure) {
		t.Fatalf("lost stdin error: %v", err)
	}
	if err := run(t.Context(), []string{"save", "project", "-"}, strings.NewReader(""), io.Discard); err == nil {
		t.Fatal("accepted empty stdin")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed stdin created files: %v, %v", entries, err)
	}
}
