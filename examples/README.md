# MCP client configuration examples

Each directory contains:

- `before.json` and `after.json`: two client configurations
- `expected.txt`: exact terminal output
- `expected.json`: exact output from `--json`

`examples_test.go` discovers every directory and compares both output formats
byte for byte.

## Review scenarios

- `cursor-new-server-version-bump`: a GitHub server package changes version and
  a PostgreSQL server is added. The DSN password is redacted.
- `vscode-stdio-to-http`: a local Docker server becomes a remote HTTP server,
  an input-backed environment variable is removed, and a literal authorization
  header is added.
- `gemini-tool-filter-trust`: two tool names enter `includeTools` and trusted
  execution is enabled.
- `claude-scopes-headers-helper`: an OAuth scope and an executable header
  helper are added.
- `no-changes`: identical Claude Code configurations produce exit code 0.

## Public repository fixtures

The public fixtures preserve configuration content from the named commit. The
before file is an empty client configuration because the source path did not
exist at the parent commit.

### Cursor: ng-select

- Repository: `ng-select/ng-select`
- Path: `.cursor/mcp.json`
- Before: `5ca4edbbfdab78158082cd90b345f86e57b080d3`
- After: `0ea9df0b1c0e9e25aeb94f25e792dc617dae5915`
- Fixture: `cursor-public-ng-select-add-server`

The change adds an Angular CLI stdio server with its original arguments.

### VS Code: HashiCorp Design System

- Repository: `hashicorp/design-system`
- Path: `.vscode/mcp.json`
- Before: `d8e92d709903af9ea2902fdf3fe8eb3cc9de8efe`
- After: `f1aea34434a19f4b8ccdbd641ee59f40264ce175`
- Fixture: `vscode-public-hashicorp-add-server`

The change adds the Ember MCP stdio server pinned to `ember-mcp@0.1.0`.

## Run one example

```sh
make build
bin/mcp-diff \
  --format cursor \
  examples/cursor-new-server-version-bump/before.json \
  examples/cursor-new-server-version-bump/after.json
```

Exit code 1 means the expected configuration change was found.
