# req

Terminal-native HTTP & WebSocket client. Requests are TOML files on disk, run from the command line.

No GUI. No cloud sync. No account. Just files and a binary.

## Install

### From source

```bash
go install github.com/taratani21/req@latest
```

### Build locally

```bash
git clone https://github.com/taratani21/req.git
cd req
make install    # builds and copies to /usr/local/bin
```

## Quick Start

### 1. Scaffold a project

```bash
req init
```

This creates a `.requests/` directory:

```
.requests/
  envs/
    local.toml        # environment variables
  example-http.toml
  example-ws.toml
```

### 2. Write a request

```toml
# .requests/get-users.toml
name = "List users"
type = "http"
method = "GET"
url = "{{base_url}}/api/users"

[headers]
Authorization = "Bearer {{token}}"
Accept = "application/json"

[query]
page = "1"
```

### 3. Set up your environment

```toml
# .requests/envs/local.toml
base_url = "http://localhost:8080"
token = "dev-token-abc123"
```

### 4. Run it

```bash
req run .requests/get-users.toml --env local
```

Response body goes to stdout. Pipe it wherever you want:

```bash
req run .requests/get-users.toml --env local | jq '.data[0].name'
```

## Environment Files

Environment files hold variables that change between environments (local, staging, prod). They live in `.requests/envs/` and are loaded with `--env <name>`.

```toml
# .requests/envs/staging.toml
base_url = "https://staging.api.example.com"
token = "staging-token-xyz"
user_id = "42"
```

### Keep secrets out of git

Environment files typically contain tokens and other secrets. Exclude them from git:

```bash
echo ".requests/envs/" >> .git/info/exclude
```

This is local to your clone. The request templates themselves (which contain `{{variables}}`, not actual secrets) can be committed and shared.

## Variable Interpolation

Variables use `{{name}}` syntax and work everywhere: URLs, headers, query params, request bodies, and WebSocket payloads.

```toml
url = "{{base_url}}/users/{{user_id}}"

[headers]
Authorization = "Bearer {{token}}"
```

### Resolution order

Variables resolve in this order (highest priority first):

1. `--var` flags on the command line
2. Extracted values from previous chain steps
3. Environment file loaded via `--env`

```bash
# --var always wins
req run .requests/get-user.toml --env local --var user_id=99
```

### Strict resolution

All variables in a request file must resolve. If any variable is missing, `req` stops with a clear error:

```
error: unresolved variable "token" in header "Authorization"
hint: set it with --var token=<value> or define it in your env file
```

## Commands

### `req run <file>`

Run an HTTP request.

```bash
req run .requests/users/get-profile.toml
req run .requests/users/get-profile.toml --env staging
req run .requests/users/get-profile.toml --env staging --var user_id=99
req run .requests/users/get-profile.toml --verbose
req run .requests/users/get-profile.toml --timeout 5s
```

| Flag | Description |
|---|---|
| `--env <name>` | Load `.requests/envs/<name>.toml` |
| `--var key=value` | Set or override a variable (repeatable) |
| `--verbose` | Print request/response details to stderr |
| `--timeout <duration>` | Request timeout (default: `30s`) |

**Output:**
- Response body &rarr; **stdout** (pipeable)
- Verbose details &rarr; **stderr**
- Exit `0` on 2xx, exit `1` on 4xx/5xx or connection error

### `req ws <file>`

Connect to a WebSocket endpoint.

```bash
req ws .requests/ws/events.toml --env local
req ws .requests/ws/events.toml --no-interactive
```

After connecting, if the request file defines `[[messages]]`, they are sent in order. Then `req` drops into interactive mode: incoming messages print to stdout, and you type messages to send.

| Flag | Description |
|---|---|
| `--env <name>` | Load environment file |
| `--var key=value` | Set or override a variable |
| `--no-interactive` | Send defined messages only, then disconnect |
| `--verbose` | Print connection details to stderr |
| `--timeout <duration>` | Connection/await timeout (default: `30s`) |

Press `Ctrl+C` to disconnect cleanly.

### `req chain <file>`

Run a sequence of HTTP requests, extracting values from responses to use in later steps.

```bash
req chain .requests/flows/create-and-fetch.chain.toml --env staging
req chain .requests/flows/create-and-fetch.chain.toml --verbose
```

| Flag | Description |
|---|---|
| `--env <name>` | Load environment file |
| `--var key=value` | Set or override a variable |
| `--verbose` | Print all steps' details to stderr |
| `--timeout <duration>` | Per-request timeout (default: `30s`) |

**Output:**
- Only the **last step's** response goes to stdout
- With `--verbose`, all intermediate responses go to stderr
- Stops at the first non-2xx response (exit `1`)

### `req init`

Create a `.requests/` directory with example files.

```bash
req init
```

## File Formats

### HTTP Request

```toml
name = "Create user"
type = "http"
method = "POST"
url = "{{base_url}}/users"

[headers]
Content-Type = "application/json"
Authorization = "Bearer {{token}}"

[query]
verbose = "true"

[body]
data = '''
{
  "name": "John Doe",
  "email": "john@example.com"
}
'''
```

- `type` must be `"http"`
- `method`: any HTTP method (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`)
- `[headers]`: key-value pairs, set `Content-Type` here
- `[query]`: appended to the URL as query parameters
- `[body]`: optional, `data` holds the raw request body

### WebSocket Request

```toml
name = "Subscribe to events"
type = "websocket"
url = "wss://{{base_url}}/events"

[headers]
Authorization = "Bearer {{token}}"

[query]
token = "{{token}}"

[[messages]]
payload = '{"type": "subscribe", "channel": "updates"}'
await_response = true

[[messages]]
payload = '{"type": "ping"}'
await_response = false
```

- `type` must be `"websocket"`
- `[query]`: optional, appended to the URL as query parameters on the upgrade request
- `[[messages]]`: optional, sent in order after connecting
- `await_response`: if `true`, waits for one incoming message before sending the next
- If `[[messages]]` is omitted, drops directly into interactive mode

### Chain File

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

- Each step references a request file (path relative to the chain file)
- `[steps.extract]` maps variable names to dot-paths into the JSON response
- Extracted values are available to all subsequent steps

**Dot-path syntax:**

| Path | Extracts |
|---|---|
| `access_token` | Top-level key |
| `data.id` | Nested key |
| `data.users.0.name` | Array index |

### Environment File

```toml
# .requests/envs/local.toml
base_url = "http://localhost:8080"
token = "dev-token-abc123"
user_id = "42"
```

Flat key-value pairs. All values are strings.

## Examples

### Pipe to jq

```bash
req run .requests/get-users.toml --env local | jq '.data[] | .name'
```

### Override a variable for one run

```bash
req run .requests/get-user.toml --env local --var user_id=99
```

### Chain: login, create, fetch

```toml
# .requests/flows/full-flow.chain.toml
name = "Full user flow"

[[steps]]
request = "../auth/login.toml"
[steps.extract]
token = "access_token"

[[steps]]
request = "../users/create-user.toml"
[steps.extract]
user_id = "data.id"

[[steps]]
request = "../users/get-profile.toml"
```

```bash
req chain .requests/flows/full-flow.chain.toml --env staging | jq
```

### WebSocket: subscribe and listen

```bash
# Send subscription messages, then listen interactively
req ws .requests/ws/subscribe.toml --env local

# Send messages only, no interaction (for scripting)
req ws .requests/ws/subscribe.toml --env local --no-interactive
```

### Use in shell scripts

```bash
#!/bin/bash
set -e

# req exits 1 on non-2xx, so set -e catches failures
TOKEN=$(req run .requests/auth/login.toml --env local | jq -r '.access_token')

req run .requests/users/list.toml --env local --var "token=$TOKEN" | jq '.data'
```

## Directory Layout

There is no required layout. Organize however makes sense for your project. A typical structure:

```
.requests/
  envs/
    local.toml
    staging.toml
  auth/
    login.toml
  users/
    get-profile.toml
    create-user.toml
  ws/
    subscribe-events.toml
  flows/
    create-and-fetch-user.chain.toml
```

## Exit Codes

| Scenario | Exit Code |
|---|---|
| HTTP 2xx | `0` |
| HTTP 4xx/5xx | `1` |
| Connection error / timeout | `1` |
| Unresolved variable | `1` |
| Malformed TOML | `1` |
| WebSocket closed cleanly | `0` |
| WebSocket closed with error | `1` |

The response body is always written to stdout, even on 4xx/5xx. This lets you inspect error responses:

```bash
req run .requests/create-user.toml --env local 2>/dev/null | jq '.errors'
```
