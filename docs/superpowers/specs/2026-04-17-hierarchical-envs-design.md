# Hierarchical env resolution — design

## Motivation

Today, `req run`, `req ws`, and `req chain` resolve `--env <name>` to a single
file at `<requestDir>/envs/<name>.toml` (for chains, `<chainDir>/envs/<name>.toml`).
This forces users to either keep all requests in one flat directory, or
duplicate env files into every subdirectory that holds requests. In practice,
projects want shared env files at a root (`.requests/envs/local.toml`) with
requests organized into subdirectories (`.requests/users/`, `.requests/auth/`,
etc.), and today that doesn't work — those subdirectory requests fail because
there's no sibling `envs/` next to them.

This spec makes env resolution walk up the directory tree, finding and merging
every `envs/<name>.toml` on the way, with the nearest file's keys winning.

## Behavior

When a command is invoked with `--env <name>`:

1. Determine the **start directory**: `filepath.Dir(reqFile)` for `run`/`ws`,
   `filepath.Dir(chainFile)` for `chain`.
2. Walk upward from the start directory. At each level, check whether
   `<level>/envs/<name>.toml` exists. If so, record its path.
3. Stop after processing a directory whose base name is `.requests`
   (inclusive), or when there is no parent directory to walk into
   (filesystem root).
4. Load each recorded file in order from **farthest ancestor → nearest**,
   merging into a single `map[string]string`. Later loads overwrite keys from
   earlier loads, so the nearest file wins per-key.
5. If zero files were found during the walk, return an error using the same
   message shape as today's missing-env error (see "Error handling").
6. If any recorded file fails to parse, return an error that includes the
   file path.

The resulting merged map is passed to the existing variable resolver exactly
as today's single-file `envVars` map is. Nothing downstream of the loader
changes.

## Examples

### Shared env at the root

```
.requests/
  envs/
    local.toml        # base_url = "http://localhost:8080", token = "dev"
  users/
    get-profile.toml
```

`req run .requests/users/get-profile.toml --env local` walks:

1. `.requests/users/envs/local.toml` — not found
2. `.requests/envs/local.toml` — found, loaded
3. Stop (processed `.requests`).

Merged env: `{base_url, token}`.

### Override at a subdirectory

```
.requests/
  envs/
    local.toml        # base_url = "http://api.local", token = "global"
  admin/
    envs/
      local.toml      # token = "admin-dev"
    delete-user.toml
```

`req run .requests/admin/delete-user.toml --env local` walks:

1. `.requests/admin/envs/local.toml` — found (`token = admin-dev`)
2. `.requests/envs/local.toml` — found (`base_url, token = global`)
3. Stop.

Load order: root first, then admin.
Merged env: `{base_url = "http://api.local", token = "admin-dev"}` — admin's
`token` overrides root's.

### Request file outside `.requests/`

```
./foo.toml
./envs/local.toml
```

Walk starts at `.`, finds `./envs/local.toml`, continues upward to `..`, `../..`,
etc., looking for more `envs/local.toml` until filesystem root. In practice, if
there is no `.requests/` ancestor and no other `envs/local.toml` ancestors,
only the one next to `foo.toml` is used.

### Chain files

`req chain foo.chain.toml --env local` walks from `filepath.Dir(foo.chain.toml)`.
Individual step request files **do not** trigger additional walks; the env is
resolved once per chain invocation, matching today's behavior.

## Error handling

Today's error when `--env local` is passed but the single expected file is
missing:

```
error: loading env "local": reading env file: open .requests/envs/local.toml: no such file or directory
```

New error when `--env local` is passed and zero files are found in the walk:

```
error: loading env "local": no envs/local.toml found in <startDir> or any ancestor up to .requests/
```

The message names the start directory and the stop condition so the user can
see where the walk looked. If the walk hit filesystem root (no `.requests/`
ancestor), say "or any ancestor" without the `.requests/` suffix.

Parse errors keep today's shape: `parsing env file <path>: <detail>`.

## Implementation

### `internal/loader/toml.go`

Add:

```go
// LoadEnvHierarchical walks from startDir upward, collecting every
// envs/<name>.toml on the way. The walk is inclusive of startDir and stops
// after processing a directory named ".requests", or at filesystem root.
// Files are merged farthest-first so nearer files override farther ones.
// Returns an error if no files are found or any found file fails to parse.
func LoadEnvHierarchical(startDir, name string) (map[string]string, error)
```

Internally, `LoadEnvHierarchical` calls the existing `LoadEnv` once per
found file, merges the results, and returns the combined map.

The existing `LoadEnv(path string)` stays as-is — it's the per-file primitive
and is still useful on its own.

### `cmd/run.go`, `cmd/ws.go`, `cmd/chain.go`

Replace each block of the form:

```go
envPath := filepath.Join(filepath.Dir(reqFile), "envs", envName+".toml")
envVars, err = loader.LoadEnv(envPath)
```

with:

```go
envVars, err = loader.LoadEnvHierarchical(filepath.Dir(reqFile), envName)
```

`cmd/chain.go` passes `chainDir` instead of `filepath.Dir(reqFile)`.

The surrounding `if envName != "" { ... }` guard stays in place — if the user
doesn't pass `--env`, we skip the walk entirely and `envVars` remains nil,
matching today's behavior.

### No changes required

- `internal/interpolate/` — still receives a single merged `envVars` map.
- `internal/runner/`, `internal/ws/`, `internal/extract/` — untouched.
- `cmd/variant.go`, `cmd/root.go`, `cmd/init_cmd.go` — untouched.

## Tests

New tests in `internal/loader/toml_test.go`:

1. **Single file at start dir** — confirms non-hierarchical case still works.
2. **Single file at ancestor** — start dir has no `envs/`, ancestor does.
3. **Nearest-wins merge** — envs at two levels, nearest overrides shared key,
   keys unique to farther file still present.
4. **Three-level merge** — envs at start, mid, and root — verify correct
   override order and final map.
5. **Stops at `.requests/`** — an `envs/<name>.toml` *above* `.requests/` must
   not be loaded.
6. **Stops at filesystem root when no `.requests/`** — walk terminates
   gracefully without error if nothing is found up to root.
7. **No file found → error** — error message matches the new shape.
8. **Parse error propagates** — malformed TOML in a mid-level file surfaces
   with the file path.

Tests use `t.TempDir()` and real filesystem layouts so walk behavior is
exercised end-to-end.

## Documentation

Update `README.md`:

- **Environment Files** section: replace the single-path description with a
  short paragraph explaining the walk + merge, plus a two-level example
  showing override behavior.
- **Resolution order** section: unchanged — the precedence list still has
  "Environment file" as the lowest tier; hierarchical merging happens
  *inside* that tier.

No changes to `docs/superpowers/` other than this spec.

## Out of scope

- Hierarchical resolution for request files themselves (no current user
  need; requests are referenced by explicit path).
- Hierarchical resolution for chain step request files (same reason; the
  chain file lists them by path).
- Env file formats other than TOML.
- Interpolation *inside* env files (not supported today, not changed here).
