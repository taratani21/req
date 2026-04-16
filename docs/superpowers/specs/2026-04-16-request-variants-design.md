# Request Variants — Design

## Problem

Some requests have a small enum of parameterizations — for example, a
WebSocket connection whose `role` query param only ever takes two values
(`admin` / `viewer`). Today users have two options, both bad:

1. Duplicate the whole request TOML into `chat-admin.toml` / `chat-viewer.toml`.
2. Pass `--var role=admin` on every invocation.

We want a named, in-file preset concept between "full duplicate file" and
"pass the value every time".

## Solution

Add a `[variants.<name>]` table to request files. Each variant is a flat
`key = value` block whose keys are variables. A new `--variant <name>` CLI
flag selects one variant per run; its vars are merged into the existing
interpolation stack.

### TOML syntax

```toml
name = "Chat"
type = "websocket"
url = "wss://{{base_url}}/chat"

[query]
role = "{{role}}"

[variants.admin]
role = "admin"

[variants.viewer]
role = "viewer"
```

- `[variants.<name>]` — any identifier, user-chosen.
- Value types: strings (same coercion as env files).
- Multiple variants per file allowed.
- Variants set *variables only* — no direct header/query/body overrides
  (parameterize via `{{var}}` and let the variant set the var).

### CLI

Single flag on `run`, `ws`, and `chain`:

```bash
req run chat.toml --variant admin
req ws chat.toml --variant viewer --env local
req chain flow.chain.toml --variant staging
```

- At most one `--variant` per invocation (no stacking in this iteration).
- Omitting `--variant` leaves behavior identical to today.

### Resolution precedence

Variants become a fourth source in `ResolveVars`. Final order, highest wins:

1. `--var` CLI flags
2. `--variant` vars
3. Extracted vars (chain only)
4. Env file vars

`ResolveVars(cliVars, variantVars, extracted, envVars)` — variants layer
between CLI and extracted.

### Per-command behavior

- **`req run` / `req ws`:** if `--variant foo` is passed and
  `[variants.foo]` doesn't exist in the file → exit 1 with
  `unknown variant "foo" (available: admin, viewer)`.
- **`req chain`:** `--variant foo` applies globally. If a step file has no
  `[variants.foo]`, that step silently runs without variant vars (matches
  `--env` semantics — missing optional config doesn't fail). Unresolved
  `{{var}}` in any step still errors with today's strict message.
- **No `--variant` flag:** the variant map is empty; existing behavior
  unchanged, even if the file defines `[variants.*]` blocks.
- **Variant sets a var that no template references:** no warning, no
  error (matches env behavior).

## Component changes

1. `internal/loader/toml.go` — add `Variants map[string]map[string]string
   `toml:"variants"`` to `Request`. Existing files parse unchanged.
2. `internal/interpolate/vars.go` — change `ResolveVars` signature to
   accept `variantVars` as a new parameter; merge in the documented order.
3. `cmd/root.go` — add a persistent `--variant <name>` string flag.
4. `cmd/run.go`, `cmd/ws.go` — look up the named variant; on miss, return
   unknown-variant error; pass variant vars to `ResolveVars`.
5. `cmd/chain.go` — on each step, look up the variant in that step's file.
   Missing → pass empty map (silent skip). Pass variant vars through the
   step's `ResolveVars` call.

## Testing

- **Unit — `ResolveVars`:** table test asserting precedence across all
  four sources (cli/variant/extracted/env).
- **Loader:** round-trip a TOML with multiple `[variants.*]` blocks.
- **Integration (`integration_test.go`):**
  - `req run --variant admin` resolves a variant var into a query param
    the test server sees.
  - `req ws --variant admin --no-interactive` resolves a variant var
    into the WebSocket upgrade query string.
  - `req run --variant bogus` exits 1 with an error mentioning
    "unknown variant" and the available names.
  - `req run --variant admin --var role=other` — `--var` wins
    (precedence proof).
  - `req chain --variant staging` — chain with two steps where only one
    defines `[variants.staging]`; chain runs cleanly and applies the
    variant only in the step that has it.

## Non-goals

- Stackable variants (`--variant a --variant b`).
- Default / auto-selected variants.
- Direct header/query/body overrides from variants.
- Cross-file variant sharing.
- Variants in env files.

Any of these can be layered on later without breaking this design.
