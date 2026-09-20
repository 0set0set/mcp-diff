package diff

import (
	"reflect"
	"testing"

	"github.com/0set0set/mcp-diff/internal/config"
)

func TestComputeNoChanges(t *testing.T) {
	t.Parallel()

	server := config.Server{
		Transport: "stdio",
		Command:   "npx",
		Args:      []string{"-y", "server"},
		Env:       map[string]config.Value{},
		Headers:   map[string]config.Value{},
		Include:   []string{"read", "write"},
		Exclude:   []string{},
		Flags:     map[string]string{},
	}
	got := Compute(
		config.Snapshot{"github": server},
		config.Snapshot{"github": server},
		config.FormatCursor,
	)
	want := Result{Format: config.FormatCursor, Servers: []ServerDiff{}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Compute() = %#v, want %#v", got, want)
	}
	if got.HasChanges() {
		t.Fatal("HasChanges() = true, want false")
	}
}

func TestComputeChangedServer(t *testing.T) {
	t.Parallel()

	before := config.Snapshot{
		"github": {
			Transport: "stdio",
			Command:   "npx",
			Args:      []string{"-y", "server@1", "--old"},
			Env: map[string]config.Value{
				"TOKEN": {Kind: config.ValueLiteral},
				"OLD":   {Kind: config.ValueReference},
			},
			Headers: map[string]config.Value{},
			Include: []string{"read"},
			Exclude: []string{"delete"},
			Flags: map[string]string{
				"trust":        "false",
				"oauth.scopes": "read",
			},
		},
	}
	after := config.Snapshot{
		"github": {
			Transport: "http",
			URL:       "https://example.com/mcp",
			Args:      []string{"-y", "server@2"},
			Env: map[string]config.Value{
				"TOKEN": {Kind: config.ValueReference, Ref: "NEW_TOKEN"},
				"NEW":   {Kind: config.ValueInput},
			},
			Headers: map[string]config.Value{
				"Authorization": {Kind: config.ValueLiteral},
			},
			Include: []string{"read", "write"},
			Exclude: []string{},
			Flags: map[string]string{
				"trust":        "true",
				"oauth.scopes": "read write",
			},
		},
	}

	got := Compute(before, after, config.FormatGemini)
	wantChanges := []Change{
		{Path: "args[1]", Op: StatusChanged, Old: "server@1", New: "server@2"},
		{Path: "args[2]", Op: StatusRemoved, Old: "--old"},
		{Path: "command", Op: StatusChanged, Old: "npx", New: "(none)"},
		{Path: "env.NEW", Op: StatusAdded, New: "input reference"},
		{Path: "env.OLD", Op: StatusRemoved, Old: "environment reference"},
		{Path: "env.TOKEN", Op: StatusChanged, Old: "literal value", New: "environment reference (NEW_TOKEN)"},
		{Path: "excludeTools", Op: StatusRemoved, Old: "delete"},
		{Path: "headers.Authorization", Op: StatusAdded, New: "literal value"},
		{Path: "includeTools", Op: StatusAdded, New: "write"},
		{Path: "oauth.scopes", Op: StatusAdded, New: "write"},
		{Path: "transport", Op: StatusChanged, Old: "stdio", New: "http"},
		{Path: "trust", Op: StatusChanged, Old: "false", New: "true"},
		{Path: "url", Op: StatusChanged, Old: "(none)", New: "https://example.com/mcp"},
	}
	if got.Format != config.FormatGemini || got.Added != 0 || got.Removed != 0 || got.Changed != 1 {
		t.Fatalf("Compute() summary = %#v", got)
	}
	if len(got.Servers) != 1 || !reflect.DeepEqual(got.Servers[0].Changes, wantChanges) {
		t.Fatalf("Compute() servers = %#v, want changes %#v", got.Servers, wantChanges)
	}
}

func TestComputeAddedAndRemovedServersAreSorted(t *testing.T) {
	t.Parallel()

	before := config.Snapshot{
		"z-old": {
			Transport: "stdio",
			Command:   "old",
			Args:      []string{},
			Env:       map[string]config.Value{},
			Headers:   map[string]config.Value{},
			Include:   []string{},
			Exclude:   []string{},
			Flags:     map[string]string{},
		},
	}
	after := config.Snapshot{
		"a-new": {
			Transport: "http",
			URL:       "https://example.com/mcp",
			Args:      []string{},
			Env:       map[string]config.Value{"TOKEN": {Kind: config.ValueReference}},
			Headers:   map[string]config.Value{},
			Include:   []string{},
			Exclude:   []string{},
			Flags:     map[string]string{},
		},
	}

	got := Compute(before, after, config.FormatClaude)
	if got.Added != 1 || got.Removed != 1 || got.Changed != 0 {
		t.Fatalf("Compute() summary = %#v", got)
	}
	if len(got.Servers) != 2 ||
		got.Servers[0].Name != "a-new" ||
		got.Servers[0].Status != StatusAdded ||
		got.Servers[1].Name != "z-old" ||
		got.Servers[1].Status != StatusRemoved {
		t.Fatalf("Compute() servers = %#v", got.Servers)
	}
	if len(got.Servers[0].Changes) != 2 {
		t.Fatalf("added server changes = %#v", got.Servers[0].Changes)
	}
}
