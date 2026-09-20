package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    Format
		wantErr string
	}{
		{name: "claude", input: `{"mcpServers":{"x":{"type":"http","url":"https://example.com"}}}`, want: FormatClaude},
		{name: "cursor", input: `{"mcpServers":{"x":{"env":{"KEY":"${env:KEY}"}}}}`, want: FormatCursor},
		{name: "gemini", input: `{"mcpServers":{"x":{"includeTools":["read"]}}}`, want: FormatGemini},
		{name: "vscode", input: `{"servers":{}}`, want: FormatVSCode},
		{name: "invalid JSON", input: `{`, wantErr: "invalid JSON"},
		{name: "missing root", input: `{}`, wantErr: "mcpServers or servers"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Detect([]byte(tt.input))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Detect() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Detect() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Detect() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadNormalizesClientFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format Format
		input  string
		want   Server
	}{
		{
			name:   "claude http and reference",
			format: FormatClaude,
			input: `{"mcpServers":{"x":{
				"type":"streamable-http",
				"url":"https://user:secret@example.com/mcp?token=secret",
				"headers":{"Authorization":"Bearer ${TOKEN}"},
				"alwaysLoad":true,
				"oauth":{"scopes":"write read"}
			}}}`,
			want: Server{
				Transport: "http",
				URL:       "https://user:***@example.com/mcp",
				Args:      []string{},
				Env:       map[string]Value{},
				Headers:   map[string]Value{"Authorization": {Kind: ValueReference, Ref: "TOKEN"}},
				Include:   []string{},
				Exclude:   []string{},
				Flags: map[string]string{
					"alwaysLoad":   "true",
					"oauth.scopes": "read write",
				},
			},
		},
		{
			name:   "cursor stdio",
			format: FormatCursor,
			input: `{"mcpServers":{"x":{
				"command":"npx",
				"args":["-y","postgresql://app:secret@db.internal:5432/main?ssl=true"],
				"env":{"TOKEN":"${env:TOKEN}"},
				"envFile":".env"
			}}}`,
			want: Server{
				Transport: "stdio",
				Command:   "npx",
				Args:      []string{"-y", "postgresql://app:***@db.internal:5432/main"},
				Env:       map[string]Value{"TOKEN": {Kind: ValueReference, Ref: "TOKEN"}},
				Headers:   map[string]Value{},
				Include:   []string{},
				Exclude:   []string{},
				Flags:     map[string]string{"envFile": ".env"},
			},
		},
		{
			name:   "vscode input",
			format: FormatVSCode,
			input: `{"servers":{"x":{
				"command":"node",
				"env":{"TOKEN":"${input:token}","ROOT":"${workspaceFolder}","PORT":3000}
			}}}`,
			want: Server{
				Transport: "stdio",
				Command:   "node",
				Args:      []string{},
				Env: map[string]Value{
					"TOKEN": {Kind: ValueInput, Ref: "token"},
					"ROOT":  {Kind: ValueReference, Ref: "workspaceFolder"},
					"PORT":  {Kind: ValueLiteral},
				},
				Headers: map[string]Value{},
				Include: []string{},
				Exclude: []string{},
				Flags:   map[string]string{},
			},
		},
		{
			name:   "gemini tool filters",
			format: FormatGemini,
			input: `{"mcpServers":{"x":{
				"httpUrl":"https://example.com/mcp",
				"env":{"TOKEN":"$TOKEN"},
				"includeTools":["write","read","read"],
				"excludeTools":["delete"],
				"trust":false
			}}}`,
			want: Server{
				Transport: "http",
				URL:       "https://example.com/mcp",
				Args:      []string{},
				Env:       map[string]Value{"TOKEN": {Kind: ValueReference, Ref: "TOKEN"}},
				Headers:   map[string]Value{},
				Include:   []string{"read", "write"},
				Exclude:   []string{"delete"},
				Flags:     map[string]string{"trust": "false"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := Load([]byte(tt.input), tt.format)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if !reflect.DeepEqual(got["x"], tt.want) {
				t.Fatalf("Load()[x] = %#v, want %#v", got["x"], tt.want)
			}
		})
	}
}

func TestLoadTreatsNullCollectionsAsAbsent(t *testing.T) {
	t.Parallel()

	absent, err := Load([]byte(`{"mcpServers":{"x":{}}}`), FormatClaude)
	if err != nil {
		t.Fatalf("Load(absent) error = %v", err)
	}
	nulls, err := Load(
		[]byte(`{"mcpServers":{"x":{"args":null,"env":null,"headers":null,"includeTools":null}}}`),
		FormatClaude,
	)
	if err != nil {
		t.Fatalf("Load(nulls) error = %v", err)
	}
	if !reflect.DeepEqual(absent, nulls) {
		t.Fatalf("absent = %#v, nulls = %#v", absent, nulls)
	}
}

func TestLoadRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		format  Format
		input   string
		wantErr string
	}{
		{name: "invalid JSON", format: FormatClaude, input: `{`, wantErr: "invalid JSON"},
		{name: "null document", format: FormatClaude, input: `null`, wantErr: "JSON object"},
		{name: "wrong root", format: FormatVSCode, input: `{"mcpServers":{}}`, wantErr: "servers is required"},
		{name: "null servers", format: FormatClaude, input: `{"mcpServers":null}`, wantErr: "must be an object"},
		{name: "empty server name", format: FormatClaude, input: `{"mcpServers":{"":{}}}`, wantErr: "server name"},
		{name: "null server", format: FormatClaude, input: `{"mcpServers":{"x":null}}`, wantErr: "must be an object"},
		{name: "args string", format: FormatClaude, input: `{"mcpServers":{"x":{"args":"-y"}}}`, wantErr: "array of strings"},
		{name: "args null item", format: FormatClaude, input: `{"mcpServers":{"x":{"args":[null]}}}`, wantErr: "args[0]"},
		{name: "env array", format: FormatClaude, input: `{"mcpServers":{"x":{"env":[]}}}`, wantErr: "env must be an object"},
		{name: "env object value", format: FormatClaude, input: `{"mcpServers":{"x":{"env":{"A":{}}}}}`, wantErr: "scalar value"},
		{name: "invalid scopes", format: FormatClaude, input: `{"mcpServers":{"x":{"oauth":{"scopes":1}}}}`, wantErr: "scopes"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := Load([]byte(tt.input), tt.format)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestInvalidFixtures(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"invalid-json.txt",
		"invalid-schema-missing-servers.json",
		"invalid-schema-args.json",
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			if _, err := Load(data, FormatClaude); err == nil {
				t.Fatal("Load() error = nil, want an error")
			}
		})
	}
}
