package output

import (
	"bytes"
	"testing"

	"github.com/0set0set/mcp-diff/internal/config"
	"github.com/0set0set/mcp-diff/internal/diff"
)

func TestWriteText(t *testing.T) {
	t.Parallel()

	result := diff.Result{
		Format: config.FormatCursor,
		Servers: []diff.ServerDiff{
			{
				Name:      "github",
				Status:    diff.StatusChanged,
				Transport: "http",
				Changes: []diff.Change{
					{Path: "transport", Op: diff.StatusChanged, Old: "stdio", New: "http"},
					{Path: "headers.Authorization", Op: diff.StatusAdded, New: "literal value"},
				},
			},
			{
				Name:      "postgres",
				Status:    diff.StatusAdded,
				Transport: "stdio",
				Changes: []diff.Change{
					{Path: "command", Op: diff.StatusAdded, New: "npx"},
				},
			},
		},
		Added:   1,
		Changed: 1,
	}

	var got bytes.Buffer
	if err := WriteText(&got, result); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}

	const want = `MCP configuration diff (cursor)
--------------------------------

~ github
    transport: stdio -> http
    headers.Authorization: + literal value

+ postgres (stdio)
    command: + npx

--------------------------------
1 server added, 0 removed, 1 changed
`
	if got.String() != want {
		t.Fatalf("WriteText() output:\n%s\nwant:\n%s", got.String(), want)
	}
}

func TestWriteTextNoChanges(t *testing.T) {
	t.Parallel()

	var got bytes.Buffer
	if err := WriteText(&got, diff.Result{Format: config.FormatClaude}); err != nil {
		t.Fatalf("WriteText() error = %v", err)
	}
	const want = "No MCP configuration changes detected.\n"
	if got.String() != want {
		t.Fatalf("WriteText() = %q, want %q", got.String(), want)
	}
}

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	result := diff.Result{
		Format: config.FormatGemini,
		Servers: []diff.ServerDiff{{
			Name:      "github",
			Status:    diff.StatusChanged,
			Transport: "stdio",
			Changes: []diff.Change{{
				Path: "includeTools",
				Op:   diff.StatusAdded,
				New:  "merge_pull_request",
			}},
		}},
		Changed: 1,
	}

	var got bytes.Buffer
	if err := WriteJSON(&got, result); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	const want = `{
  "schema_version": 1,
  "format": "gemini",
  "servers": [
    {
      "name": "github",
      "status": "changed",
      "transport": "stdio",
      "changes": [
        {
          "path": "includeTools",
          "op": "added",
          "new": "merge_pull_request"
        }
      ]
    }
  ],
  "summary": {
    "added": 0,
    "removed": 0,
    "changed": 1
  }
}
`
	if got.String() != want {
		t.Fatalf("WriteJSON() output:\n%s\nwant:\n%s", got.String(), want)
	}
}

func TestWriteJSONNoChangesUsesEmptyArray(t *testing.T) {
	t.Parallel()

	var got bytes.Buffer
	if err := WriteJSON(&got, diff.Result{Format: config.FormatClaude}); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}

	const want = `{
  "schema_version": 1,
  "format": "claude",
  "servers": [],
  "summary": {
    "added": 0,
    "removed": 0,
    "changed": 0
  }
}
`
	if got.String() != want {
		t.Fatalf("WriteJSON() output:\n%s\nwant:\n%s", got.String(), want)
	}
}
