# `req` — CLI HTTP & WebSocket Client Design Spec

## Overview

`req` is a terminal-native CLI tool for running HTTP and WebSocket requests defined in TOML files. Requests are stored as plain files on disk, committed alongside the codebase, and run from the terminal. There is no GUI, no cloud sync, and no account required.

The tool is designed to integrate naturally with a Neovim + git workflow, including git worktrees.

---

## Goals

- Run saved HTTP and WebSocket requests from the terminal with a single command
- Store requests as human-readable TOML files that live in the repo
- Support environment files for variable interpolation (kept out of git via `.git/info/exclude`)
- Support interactive WebSocket sessions (connect, send, receive in the terminal)
- Output raw response bodies to stdout so output is pipeable to `jq` and other tools
- Support request chaining — run requests in sequence, passing extracted response values forward
- Single static binary, no runtime dependencies

## Non-Goals (v1)

- Collection runners / batch execution
- Test assertions
- Auth flows beyond static header values (no OAuth)
- TUI or GUI of any kind

---

## Language & Dependencies

**Language:** Go

**Key dependencies:**

| Package | Purpose |
|---|---|
| `github.com/spf13/cobra` | CLI structure and subcommands |
| `github.com/BurntSushi/toml` | TOML parsing |
| `github.com/gorilla/websocket` | WebSocket client |
| `net/http` (stdlib) | HTTP client |

---

## Directory Layout

Requests live wherever the user wants — typically a `.requests/` directory in the project root. Environment files are excluded from git via `.git/info/exclude`.

```
.requests/
  envs/
    local.toml
    staging.toml
  auth/
    login.toml
  users/
    get-profile.toml
    update-profile.toml
  ws/
    subscribe-events.toml
  flows/
    create-and-fetch-user.chain.toml
```

---

## File Formats

### Request File (`*.toml`)

#### HTTP Request

```toml
name = "Get user profile"
type = "http"
method = "GET"
url = "{{base_url}}/users/{{user_id}}"

[headers]
Authorization = "Bearer {{token}}"
Content-Type = "application/json"

[query]
include_inactive = "false"

# body is optional, used for POST/PUT/PATCH
# Content-Type is always set in [headers], not here
[body]
data = '''
{
  "name": "John Doe"
}
'''
```

`Content-Type` is always set in `[headers]`. The `[body]` section contains only `data`.

#### WebSocket Request

```toml
name = "Subscribe to events"
type = "websocket"
url = "wss://{{base_url}}/events"

[headers]
Authorization = "Bearer {{token}}"

# messages are sent in order on connect (optional)
# if no messages are defined, drops into interactive mode
[[messages]]
payload = '{"type": "subscribe", "channel": "updates"}'
await_response = true

[[messages]]
payload = '{"type": "ping"}'
await_response = false
```

If `[[messages]]` is omitted, the tool drops directly into interactive mode after connecting.

### Chain File (`*.chain.toml`)

A chain file defines a sequence of requests to run in order, with value extraction between steps.

```toml
name = "Create and fetch user"

[[steps]]
request = "auth/login.toml"
[steps.extract]
token = "access_token"

[[steps]]
request = "users/create-user.toml"
[steps.extract]
user_id = "data.id"

[[steps]]
request = "users/get-profile.toml"
```

Each step references an existing request file. The `[steps.extract]` table maps variable names to dot-paths into the JSON response body. Extracted values merge into the variable pool and are available to all subsequent steps.

Dot-path examples:
- `access_token` — top-level key
- `data.id` — nested key
- `data.users.0.name` — array index access

### Environment File (`envs/<name>.toml`)

```toml
base_url = "api.example.com"
token = "dev-token-abc123"
user_id = "42"
```

Environment files should be added to `.git/info/exclude` to prevent committing secrets:

```bash
echo ".requests/envs/" >> .git/info/exclude
```

---

## CLI Interface

### Top-level

```
req <command> [flags]
```

### Commands

#### `req run <file> [flags]`

Runs a single HTTP request file.

```
req run .requests/users/get-profile.toml
req run .requests/users/get-profile.toml --env staging
req run .requests/users/get-profile.toml --env staging --var user_id=99
```

**Flags:**

| Flag | Description |
|---|---|
| `--env <name>` | Load environment from `.requests/envs/<name>.toml` |
| `--var <key=value>` | Override or inject a single variable (repeatable) |
| `--verbose` | Print request details (method, url, headers) to stderr before executing |
| `--timeout <duration>` | Request timeout, e.g. `30s`, `5m` (default: `30s`) |

**Output behavior:**

- Response body is written to **stdout** (pipeable to `jq`, `grep`, etc.)
- Status code, headers, and request details are written to **stderr** when `--verbose` is set
- Exit code `0` on HTTP 2xx; exit code `1` on HTTP 4xx/5xx or connection error

#### `req ws <file> [flags]`

Connects a WebSocket request and enters interactive mode.

```
req ws .requests/ws/subscribe-events.toml
req ws .requests/ws/subscribe-events.toml --env local
```

**Behavior:**

1. Connects to the WebSocket URL with the specified headers
2. If `[[messages]]` are defined in the file, sends them in order
3. When `await_response = true` on a message, waits for one incoming message before sending the next (uses the same `--timeout` duration)
4. Drops into interactive mode:
   - Incoming messages are printed to stdout, one JSON blob per line
   - User types messages and presses Enter to send
   - `Ctrl+C` closes the connection cleanly
5. All received messages go to stdout (pipeable if not interactive)

**Flags:**

| Flag | Description |
|---|---|
| `--env <name>` | Load environment from `.requests/envs/<name>.toml` |
| `--var <key=value>` | Override a variable |
| `--no-interactive` | Send defined messages only, then disconnect (useful for scripting) |
| `--timeout <duration>` | Connection and await_response timeout (default: `30s`) |

#### `req chain <file> [flags]`

Runs a chain file — executes requests in sequence, extracting values between steps.

```
req chain .requests/flows/create-and-fetch-user.chain.toml
req chain .requests/flows/create-and-fetch-user.chain.toml --env staging
```

**Behavior:**

1. Parses the chain file and validates all referenced request files exist
2. Executes each step in order, resolving variables before each request
3. After each step, extracts values from the JSON response body using dot-paths
4. Extracted values are added to the variable pool for subsequent steps
5. Only the last step's response body is written to stdout
6. With `--verbose`, all steps' request/response details are printed to stderr

**Flags:**

| Flag | Description |
|---|---|
| `--env <name>` | Load environment from `.requests/envs/<name>.toml` |
| `--var <key=value>` | Override or inject a single variable (repeatable) |
| `--verbose` | Print all steps' request details and intermediate responses to stderr |
| `--timeout <duration>` | Per-request timeout (default: `30s`) |

**Exit code:** `0` if all steps return HTTP 2xx; `1` if any step fails (stops execution at the first failure).

#### `req init`

Scaffolds a `.requests/` directory in the current working directory with example files.

```
req init
```

Creates:
```
.requests/
  envs/
    local.toml       # example env file
  example-http.toml
  example-ws.toml
```

Also prints a reminder to add `.requests/envs/` to `.git/info/exclude`.

---

## Variable Interpolation

Variables use `{{variable_name}}` syntax throughout request files (url, headers, body, query params, WebSocket payloads).

**Resolution order (highest to lowest priority):**

1. `--var` flags passed at runtime
2. Extracted values from previous chain steps (when running `req chain`)
3. Environment file loaded via `--env`
4. Error if variable is still unresolved

**Strict resolution:** All `{{variables}}` present in the request file must resolve. There is no lazy evaluation — if a variable appears anywhere in the file, it must have a value. This prevents subtle bugs where a variable silently fails to resolve.

Unresolved variables at execution time produce a clear error:

```
error: unresolved variable "token" in header "Authorization"
hint: set it with --var token=<value> or define it in your env file
```

---

## HTTP Execution

- Use Go's `net/http` stdlib client
- Follow redirects by default
- Respect `--timeout` for the full request lifecycle
- Write response body to stdout

---

## WebSocket Execution

- Use `gorilla/websocket` for the WebSocket client
- Perform the HTTP upgrade handshake with interpolated headers
- After connecting, if `[[messages]]` exist, send them in sequence
  - If `await_response = true`, wait for one incoming message before sending the next (bounded by `--timeout`)
- Then enter interactive mode (unless `--no-interactive`)
- Interactive mode:
  - Use a goroutine to print incoming messages to stdout continuously
  - Read lines from stdin and send as text frames on Enter
  - Handle `Ctrl+C` with a clean close handshake

---

## Error Handling

All errors go to **stderr**. Stdout is reserved for response data only so pipes are never polluted.

| Scenario | Exit Code |
|---|---|
| Successful HTTP 2xx | `0` |
| HTTP 4xx or 5xx | `1` |
| Connection error / timeout | `1` |
| Unresolved variable | `1` |
| Malformed TOML file | `1` |
| WebSocket closed cleanly | `0` |
| WebSocket closed with error | `1` |

---

## Build & Distribution

- Single static binary via `go build`
- No CGO dependencies
- Target platforms: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`
- Makefile with `build`, `install` (copies to `/usr/local/bin`), and `test` targets

---

## Project Structure

```
req/
  main.go
  cmd/
    root.go         # cobra root command, global flags
    run.go          # `req run` command
    ws.go           # `req ws` command
    chain.go        # `req chain` command
    init.go         # `req init` command
  internal/
    loader/
      toml.go       # request file and env file parsing
    interpolate/
      vars.go       # {{variable}} substitution
    extract/
      dotpath.go    # dot-path value extraction from JSON responses
    runner/
      client.go     # HTTP request execution
    ws/
      client.go     # WebSocket connection and interactive loop
  testdata/
    requests/       # fixture .toml files for tests
  Makefile
  go.mod
  go.sum
  README.md
```

---

## Out of Scope / Future Iterations

These are explicitly deferred to avoid scope creep in v1:

- Collection runner (run all files in a directory)
- Response assertions / test output
- OAuth / dynamic auth token flows
- Request chaining (use output of one request as input to another)
- `--format` flag for pretty-printed output (use `jq` for now)
- Shell completions
- Config file (`~/.config/req/config.toml`)
- `--no-follow-redirects` flag
- Per-message `await_timeout` overrides for WebSocket
