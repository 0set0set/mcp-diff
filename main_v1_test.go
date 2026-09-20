package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunNoChanges(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `{"mcpServers":{"github":{"command":"npx"}}}`)
	var stdout, stderr bytes.Buffer
	exitCode := run([]string{path, path}, strings.NewReader(""), &stdout, &stderr)

	if exitCode != 0 {
		t.Fatalf("run() exit code = %d, want 0; stderr = %q", exitCode, stderr.String())
	}
	if stdout.String() != "No MCP configuration changes detected.\n" {
		t.Fatalf("run() stdout = %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("run() stderr = %q, want empty", stderr.String())
	}
}

func TestRunChangesAndJSON(t *testing.T) {
	t.Parallel()

	before := writeConfig(t, `{"mcpServers":{"github":{"command":"npx","args":["server@1"]}}}`)
	after := writeConfig(t, `{"mcpServers":{"github":{"command":"npx","args":["server@2"]}}}`)

	var stdout, stderr bytes.Buffer
	exitCode := run(
		[]string{before, after, "--json", "--format", "cursor"},
		strings.NewReader(""),
		&stdout,
		&stderr,
	)
	if exitCode != 1 {
		t.Fatalf("run() exit code = %d, want 1; stderr = %q", exitCode, stderr.String())
	}
	var document struct {
		SchemaVersion int `json:"schema_version"`
		Format        string
		Servers       []struct {
			Name    string
			Changes []struct {
				Path string
			}
		}
		Summary struct {
			Changed int
		}
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("run() produced invalid JSON: %v\n%s", err, stdout.String())
	}
	if document.SchemaVersion != 1 ||
		document.Format != "cursor" ||
		len(document.Servers) != 1 ||
		document.Servers[0].Name != "github" ||
		document.Servers[0].Changes[0].Path != "args[0]" ||
		document.Summary.Changed != 1 {
		t.Fatalf("run() JSON = %#v", document)
	}
}

func TestRunReadsEitherInputFromStdin(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `{"mcpServers":{"x":{"command":"npx","args":["server@2"]}}}`)
	stdinConfig := `{"mcpServers":{"x":{"command":"npx","args":["server@1"]}}}`

	for _, args := range [][]string{{"-", path}, {path, "-"}} {
		var stdout, stderr bytes.Buffer
		exitCode := run(args, strings.NewReader(stdinConfig), &stdout, &stderr)
		if exitCode != 1 {
			t.Fatalf("run(%v) exit code = %d, want 1; stderr = %q", args, exitCode, stderr.String())
		}
	}
}

func TestRunErrors(t *testing.T) {
	t.Parallel()

	valid := writeConfig(t, `{"mcpServers":{}}`)
	cursor := writeConfig(t, `{"mcpServers":{"x":{"env":{"A":"${env:A}"}}}}`)
	vscode := writeConfig(t, `{"servers":{}}`)

	tests := []struct {
		name     string
		args     []string
		stdin    string
		wantText string
	}{
		{name: "no arguments", wantText: "expected two input paths"},
		{name: "unknown option", args: []string{"--verbose"}, wantText: `unknown option "--verbose"`},
		{name: "invalid format", args: []string{"--format", "other", valid, valid}, wantText: `unknown format "other"`},
		{name: "missing format value", args: []string{"--format"}, wantText: "--format requires a value"},
		{name: "both stdin", args: []string{"-", "-"}, wantText: "only one input"},
		{name: "format mismatch", args: []string{cursor, vscode}, wantText: "input formats differ"},
		{name: "invalid JSON", args: []string{"-", valid}, stdin: `{`, wantText: "invalid JSON"},
		{name: "missing file", args: []string{filepath.Join(t.TempDir(), "missing.json"), valid}, wantText: "missing.json"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			exitCode := run(tt.args, strings.NewReader(tt.stdin), &stdout, &stderr)
			if exitCode != 2 {
				t.Fatalf("run() exit code = %d, want 2", exitCode)
			}
			if stdout.Len() != 0 {
				t.Fatalf("run() stdout = %q, want empty", stdout.String())
			}
			if !strings.Contains(stderr.String(), tt.wantText) {
				t.Fatalf("run() stderr = %q, want containing %q", stderr.String(), tt.wantText)
			}
		})
	}
}

func TestRunHelpAndVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		args []string
		want string
	}{
		{args: []string{"--help"}, want: usage},
		{args: []string{"--version"}, want: "mcp-diff dev"},
	}
	for _, tt := range tests {
		var stdout, stderr bytes.Buffer
		exitCode := run(tt.args, strings.NewReader(""), &stdout, &stderr)
		if exitCode != 0 {
			t.Fatalf("run(%v) exit code = %d, want 0", tt.args, exitCode)
		}
		if !strings.Contains(stdout.String(), tt.want) || stderr.Len() != 0 {
			t.Fatalf("run(%v) stdout = %q, stderr = %q", tt.args, stdout.String(), stderr.String())
		}
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}
