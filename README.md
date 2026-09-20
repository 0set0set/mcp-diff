# WIP ⚠️ mcp-diff

`mcp-diff` is a semantic diff for MCP client configuration. It shows which MCP
servers a repository grants to its agents, how clients launch or reach them,
and which static access settings changed.

It does not decide whether a change is safe.

## Supported clients

V1 reads the JSON formats used by:

- Claude Code project configuration in `.mcp.json`
- Cursor project configuration in `.cursor/mcp.json`
- VS Code and GitHub Copilot workspace configuration in `.vscode/mcp.json`
- Gemini CLI configuration in `settings.json`

Claude Code, Cursor, and Gemini CLI use an `mcpServers` root object. VS Code
uses a `servers` root object. `--format auto` detects VS Code by its root key
and distinguishes the overlapping `mcpServers` formats from client-specific
fields and interpolation syntax. Use an explicit `--format` when a minimal
configuration is ambiguous.

Codex TOML configuration is deferred to V1.1.

## What it compares

For each server, the CLI compares:

- server additions and removals
- transport, command, arguments, URL, working directory, and environment file
- environment variable and header names
- header helper commands
- timeouts, trust, always-load, and sandbox flags when the client stores them
- OAuth scopes
- Gemini `includeTools` and `excludeTools` filters

The output is deterministic. Server names, fields, and set-like values are
sorted before formatting.

## What static analysis cannot determine

MCP clients discover actual tool names, descriptions, and schemas at runtime
by connecting to the server and calling `tools/list`. Normal client
configuration does not contain that server interface.

`mcp-diff` therefore does not claim that a server added or removed a tool. A
change such as `includeTools: + merge_pull_request` means only that the Gemini
configuration now permits that tool name.

## Build

Go 1.27.1 or newer is required. Go does not publish an LTS channel, so the
project pins the current stable Go release.

```sh
make build
```

The binary is written to `bin/mcp-diff`. Set `VERSION` to embed a release
version:

```sh
make build VERSION=1.0.0
bin/mcp-diff --version
```

Build the digest-pinned, multi-stage container image with:

```sh
make docker-build
```

The runtime image is `scratch` and runs as user `65532`.

## Usage

```sh
mcp-diff [--json] [--format auto|claude|cursor|vscode|gemini] BEFORE AFTER
```

Flags can appear before or after the input paths. `-` reads one side from
standard input:

```sh
git show origin/main:.cursor/mcp.json |
  bin/mcp-diff --format cursor - .cursor/mcp.json
```

Only one input can be `-`.

Example:

```sh
bin/mcp-diff \
  --format gemini \
  examples/gemini-tool-filter-trust/before.json \
  examples/gemini-tool-filter-trust/after.json
```

Output:

```text
MCP configuration diff (gemini)
--------------------------------

~ github
    includeTools: + create_release
    includeTools: + merge_pull_request
    trust: false -> true

--------------------------------
0 servers added, 0 removed, 1 changed
```

No changes produce:

```text
No MCP configuration changes detected.
```

Run the container against files in the current directory:

```sh
docker run --rm \
  --volume "$PWD:/work:ro" \
  mcp-diff:local \
  --format cursor \
  /work/.cursor/mcp.before.json /work/.cursor/mcp.json
```

## Redaction

The CLI never prints environment or header values. It reports only the key and
whether the value is a literal, environment reference, or input reference.
For references, the variable or input name is included so a change from a
staging reference to a production reference remains visible.

For URL-shaped server URLs and arguments:

- passwords in URL user information become `***`
- query strings are removed

This protects common DSN and bearer-token patterns in terminal, JSON, and
future CI output. Because query values are removed before comparison, changes
that exist only in a URL query string are intentionally not reported.

## JSON output

`--json` emits schema version 1:

```json
{
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
```

Arrays are encoded as `[]`, never `null`. Server status and change operations
are `added`, `removed`, or `changed`.

## Exit codes

- `0`: no configuration changes
- `1`: configuration changes detected
- `2`: usage, file, JSON, schema, format, or output error

Exit code `1` follows `diff(1)` semantics. CI can use it as a change gate, or
handle it explicitly when the report is informational.

## Examples

[`examples/`](examples/README.md) contains client-specific review scenarios,
two fixtures based on public repositories, and exact text and JSON golden
outputs. `make test` runs every example through the CLI byte for byte.

## Non-goals

V1 does not provide:

- MCP server execution or network connections
- actual runtime tool, resource, or prompt discovery
- security or vulnerability scanning
- risk scores or severity classifications
- policy enforcement
- LLM calls, databases, backends, SaaS, or dashboards
- Git revision discovery or a GitHub Action

## Development

The project uses only the Go standard library.

```sh
make check
```

`make check` verifies formatting, runs `go vet`, and executes all tests.

## License

MIT. See [LICENSE](LICENSE).
