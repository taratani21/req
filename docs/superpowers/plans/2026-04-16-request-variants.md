# Request Variants Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-file `[variants.<name>]` preset concept so users can pick between small enumerated parameterizations of the same request without duplicating TOML files or re-passing `--var` every run.

**Architecture:** Extend the existing loader to parse a `variants` table on `Request`. Thread variant vars through `ResolveVars` as a new fourth precedence source between `--var` (highest) and extracted chain values. Add a persistent `--variant <name>` flag on the root command. Each cmd (`run`, `ws`, `chain`) looks up the variant in the file it just loaded and passes its vars to `ResolveVars`; unknown variant names error in `run`/`ws` and silently skip in `chain` (matches how `--env` flows globally).

**Tech Stack:** Go, github.com/spf13/cobra, github.com/BurntSushi/toml, gorilla/websocket. Testing with stdlib `testing` + httptest.

Spec: `docs/superpowers/specs/2026-04-16-request-variants-design.md`.

---

## File Structure

**Modify:**
- `internal/loader/toml.go` — add `Variants` field to `Request` struct.
- `internal/loader/toml_test.go` — parsing test.
- `internal/interpolate/vars.go` — extend `ResolveVars` signature with `variantVars`.
- `internal/interpolate/vars_test.go` — update existing precedence test, add variant-precedence test.
- `cmd/root.go` — add `--variant` persistent flag.
- `cmd/run.go` — look up variant, error on miss, feed to `ResolveVars`.
- `cmd/ws.go` — same.
- `cmd/chain.go` — apply variant to each step file; silent skip when that step's file has no `[variants.<name>]`.
- `integration_test.go` — end-to-end tests covering run/ws/chain + error case + `--var` override.
- `README.md` — document the new section and flag.

**Create:**
- `testdata/requests/with-variants.toml` — fixture for loader test.

---

## Task 1: Add `Variants` field to `Request` and parse it

**Files:**
- Create: `testdata/requests/with-variants.toml`
- Modify: `internal/loader/toml.go`
- Modify: `internal/loader/toml_test.go`

- [ ] **Step 1: Write fixture file**

Create `testdata/requests/with-variants.toml`:

```toml
name = "Variant demo"
type = "websocket"
url = "wss://example.com/chan"

[query]
role = "{{role}}"

[variants.admin]
role = "admin"

[variants.viewer]
role = "viewer"
```

- [ ] **Step 2: Write the failing test**

Append to `internal/loader/toml_test.go`:

```go
func TestLoadRequest_WithVariants(t *testing.T) {
	req, err := LoadRequest("../../testdata/requests/with-variants.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(req.Variants) != 2 {
		t.Fatalf("variants count = %d, want 2", len(req.Variants))
	}
	if req.Variants["admin"]["role"] != "admin" {
		t.Errorf("variants.admin.role = %q, want %q", req.Variants["admin"]["role"], "admin")
	}
	if req.Variants["viewer"]["role"] != "viewer" {
		t.Errorf("variants.viewer.role = %q, want %q", req.Variants["viewer"]["role"], "viewer")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/loader/ -run TestLoadRequest_WithVariants -v`
Expected: FAIL with `req.Variants` undefined.

- [ ] **Step 4: Add `Variants` field to `Request`**

In `internal/loader/toml.go`, modify the `Request` struct:

```go
type Request struct {
	Name     string                       `toml:"name"`
	Type     string                       `toml:"type"`
	Method   string                       `toml:"method"`
	URL      string                       `toml:"url"`
	Headers  map[string]string            `toml:"headers"`
	Query    map[string]string            `toml:"query"`
	Body     Body                         `toml:"body"`
	Messages []Message                    `toml:"messages"`
	Variants map[string]map[string]string `toml:"variants"`
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/loader/ -v`
Expected: PASS, including the new test and all existing ones.

- [ ] **Step 6: Commit**

```bash
git add testdata/requests/with-variants.toml internal/loader/toml.go internal/loader/toml_test.go
git commit -m "feat(loader): parse [variants.*] blocks on requests"
```

---

## Task 2: Extend `ResolveVars` to accept variant vars

**Files:**
- Modify: `internal/interpolate/vars.go:45-59` (the `ResolveVars` function)
- Modify: `internal/interpolate/vars_test.go:75-94` (the existing priority test) + add new variant test
- Modify: `cmd/run.go:55` (signature update only, pass `nil`)
- Modify: `cmd/ws.go:58` (signature update only, pass `nil`)
- Modify: `cmd/chain.go:75` (signature update only, pass `nil`)

**Note:** This task is a pure refactor — the public signature changes but nothing behavioral. Variant wiring in the cmds happens in later tasks.

- [ ] **Step 1: Update the existing priority test to the new signature (still a failing test first)**

Replace the body of `TestResolveVars_Priority` in `internal/interpolate/vars_test.go` with:

```go
func TestResolveVars_Priority(t *testing.T) {
	cliVars := map[string]string{"token": "cli-token", "id": "cli-id"}
	variantVars := map[string]string{"token": "variant-token", "region": "variant-region"}
	extracted := map[string]string{"token": "extracted-token", "name": "extracted-name", "region": "extracted-region"}
	envVars := map[string]string{"token": "env-token", "name": "env-name", "base": "env-base", "region": "env-region"}

	resolved := ResolveVars(cliVars, variantVars, extracted, envVars)

	if resolved["token"] != "cli-token" {
		t.Errorf("token = %q, want %q (cli wins over variant/extracted/env)", resolved["token"], "cli-token")
	}
	if resolved["region"] != "variant-region" {
		t.Errorf("region = %q, want %q (variant wins over extracted/env)", resolved["region"], "variant-region")
	}
	if resolved["name"] != "extracted-name" {
		t.Errorf("name = %q, want %q (extracted wins over env)", resolved["name"], "extracted-name")
	}
	if resolved["base"] != "env-base" {
		t.Errorf("base = %q, want %q (env fallback)", resolved["base"], "env-base")
	}
	if resolved["id"] != "cli-id" {
		t.Errorf("id = %q, want %q", resolved["id"], "cli-id")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/interpolate/ -run TestResolveVars_Priority -v`
Expected: FAIL with "too many arguments" / signature mismatch.

- [ ] **Step 3: Update `ResolveVars` signature**

In `internal/interpolate/vars.go`, replace `ResolveVars`:

```go
func ResolveVars(cliVars, variantVars, extracted, envVars map[string]string) map[string]string {
	resolved := make(map[string]string)

	for k, v := range envVars {
		resolved[k] = v
	}
	for k, v := range extracted {
		resolved[k] = v
	}
	for k, v := range variantVars {
		resolved[k] = v
	}
	for k, v := range cliVars {
		resolved[k] = v
	}

	return resolved
}
```

- [ ] **Step 4: Update all three cmd callers to compile against the new signature**

In `cmd/run.go`, find the line:
```go
resolved := interpolate.ResolveVars(cliVars, nil, envVars)
```
and change it to:
```go
resolved := interpolate.ResolveVars(cliVars, nil, nil, envVars)
```

In `cmd/ws.go`, find the line:
```go
resolved := interpolate.ResolveVars(cliVars, nil, envVars)
```
and change it to:
```go
resolved := interpolate.ResolveVars(cliVars, nil, nil, envVars)
```

In `cmd/chain.go`, find the line (inside the step loop):
```go
resolved := interpolate.ResolveVars(cliVars, extracted, envVars)
```
and change it to:
```go
resolved := interpolate.ResolveVars(cliVars, nil, extracted, envVars)
```

- [ ] **Step 5: Run the full test suite — behavior unchanged**

Run: `go test ./... -count=1`
Expected: PASS everywhere.

- [ ] **Step 6: Commit**

```bash
git add internal/interpolate/vars.go internal/interpolate/vars_test.go cmd/run.go cmd/ws.go cmd/chain.go
git commit -m "refactor(interpolate): thread variantVars through ResolveVars"
```

---

## Task 3: Add `--variant` persistent flag

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Add the flag variable and registration**

In `cmd/root.go`, add `variantName string` to the `var (...)` block:

```go
var (
	envName     string
	vars        []string
	verbose     bool
	timeout     time.Duration
	variantName string
)
```

And add inside `init()`, alongside the other `PersistentFlags` calls:

```go
rootCmd.PersistentFlags().StringVar(&variantName, "variant", "", "Select a named variant from the request file")
```

- [ ] **Step 2: Verify it builds**

Run: `go build ./...`
Expected: Clean build, no output.

- [ ] **Step 3: Verify the flag is visible**

Run: `go run . run --help 2>&1 | grep variant`
Expected: Line showing `--variant string` with the description.

- [ ] **Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "feat(cmd): add --variant persistent flag"
```

---

## Task 4: Wire `--variant` into `req run` (with unknown-variant error)

**Files:**
- Modify: `cmd/run.go`
- Modify: `integration_test.go`

- [ ] **Step 1: Write failing integration test for the happy path**

Append to `integration_test.go`:

```go
func TestRun_Variant(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "req.toml"), fmt.Sprintf(`
name = "Variant run"
type = "http"
method = "GET"
url = "%s/echo"

[query]
role = "{{role}}"

[variants.admin]
role = "admin"

[variants.viewer]
role = "viewer"
`, server.URL))

	stdout, _, exitCode := runReq("run", filepath.Join(dir, "req.toml"), "--variant", "admin")
	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	query, _ := resp["query"].(map[string]interface{})
	roleVal, _ := query["role"].([]interface{})
	if len(roleVal) == 0 || roleVal[0] != "admin" {
		t.Errorf("role = %v, want [admin]", query["role"])
	}
}
```

- [ ] **Step 2: Write failing integration test for unknown variant**

Append to `integration_test.go`:

```go
func TestRun_UnknownVariant(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "req.toml"), `
name = "Variant run"
type = "http"
method = "GET"
url = "http://example.com"

[variants.admin]
role = "admin"
`)

	_, stderr, exitCode := runReq("run", filepath.Join(dir, "req.toml"), "--variant", "bogus")
	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for unknown variant")
	}
	if !strings.Contains(stderr, "unknown variant") {
		t.Errorf("stderr should mention unknown variant, got: %s", stderr)
	}
	if !strings.Contains(stderr, "admin") {
		t.Errorf("stderr should list available variants, got: %s", stderr)
	}
}
```

- [ ] **Step 3: Write failing integration test for --var overriding --variant**

Append to `integration_test.go`:

```go
func TestRun_VarOverridesVariant(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "req.toml"), fmt.Sprintf(`
name = "Override variant"
type = "http"
method = "GET"
url = "%s/echo"

[query]
role = "{{role}}"

[variants.admin]
role = "admin"
`, server.URL))

	stdout, _, exitCode := runReq("run", filepath.Join(dir, "req.toml"),
		"--variant", "admin", "--var", "role=custom")
	if exitCode != 0 {
		t.Fatalf("exit code = %d", exitCode)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	query, _ := resp["query"].(map[string]interface{})
	roleVal, _ := query["role"].([]interface{})
	if len(roleVal) == 0 || roleVal[0] != "custom" {
		t.Errorf("role = %v, want [custom] (--var should override --variant)", query["role"])
	}
}
```

- [ ] **Step 4: Run the three new tests to verify they fail**

Run: `go test -run 'TestRun_Variant$|TestRun_UnknownVariant|TestRun_VarOverridesVariant' -v -count=1`
Expected: All three FAIL — variant flag is defined but not wired in, so `{{role}}` comes back as unresolved.

- [ ] **Step 5: Wire the variant lookup into `cmd/run.go`**

In `cmd/run.go`, find the block that starts with `// Parse CLI vars` (around line 48) and the line that calls `ResolveVars`. Replace the stretch from `cliVars, err := ...` through the `resolved := ...` line with:

```go
	// Parse CLI vars
	cliVars, err := interpolate.ParseCLIVars(vars)
	if err != nil {
		return err
	}

	// Look up the selected variant, if any
	var variantVars map[string]string
	if variantName != "" {
		v, ok := req.Variants[variantName]
		if !ok {
			available := make([]string, 0, len(req.Variants))
			for name := range req.Variants {
				available = append(available, name)
			}
			sort.Strings(available)
			if len(available) == 0 {
				return fmt.Errorf("unknown variant %q (this request defines no variants)", variantName)
			}
			return fmt.Errorf("unknown variant %q (available: %s)", variantName, strings.Join(available, ", "))
		}
		variantVars = v
	}

	// Resolve variables (cli > variant > extracted > env, no extracted for single run)
	resolved := interpolate.ResolveVars(cliVars, variantVars, nil, envVars)
```

- [ ] **Step 6: Add the new imports to `cmd/run.go`**

In the import block of `cmd/run.go`, add `"sort"` and `"strings"` (alphabetical):

```go
import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/taratani21/req/internal/interpolate"
	"github.com/taratani21/req/internal/loader"
	"github.com/taratani21/req/internal/runner"
)
```

- [ ] **Step 7: Run the three new tests to verify they pass**

Run: `go test -run 'TestRun_Variant$|TestRun_UnknownVariant|TestRun_VarOverridesVariant' -v -count=1`
Expected: All three PASS.

- [ ] **Step 8: Run the full test suite to check for regressions**

Run: `go test ./... -count=1`
Expected: PASS everywhere.

- [ ] **Step 9: Commit**

```bash
git add cmd/run.go integration_test.go
git commit -m "feat(run): select variant vars via --variant flag"
```

---

## Task 5: Wire `--variant` into `req ws`

**Files:**
- Modify: `cmd/ws.go`
- Modify: `integration_test.go`

- [ ] **Step 1: Write failing integration test for ws + variant**

Append to `integration_test.go`:

```go
func TestWS_Variant(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	var mu sync.Mutex
	var seenQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seenQuery = r.URL.RawQuery
		mu.Unlock()
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		conn.Close()
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ws.toml"), fmt.Sprintf(`
name = "WS variant"
type = "websocket"
url = "%s/chan"

[query]
role = "{{role}}"

[variants.admin]
role = "admin"

[variants.viewer]
role = "viewer"
`, wsURL))

	_, stderr, exitCode := runReq("ws", filepath.Join(dir, "ws.toml"), "--no-interactive", "--variant", "viewer")
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr: %s", exitCode, stderr)
	}

	mu.Lock()
	got := seenQuery
	mu.Unlock()
	if !strings.Contains(got, "role=viewer") {
		t.Errorf("expected role=viewer in query, got: %q", got)
	}
}
```

- [ ] **Step 2: Run the new test to verify it fails**

Run: `go test -run TestWS_Variant -v -count=1`
Expected: FAIL — `{{role}}` unresolved or query missing.

- [ ] **Step 3: Wire the variant lookup into `cmd/ws.go`**

In `cmd/ws.go`, find the section starting at `// Parse CLI vars` and ending with the `resolved := ...` line. Replace it with:

```go
	// Parse CLI vars
	cliVars, err := interpolate.ParseCLIVars(vars)
	if err != nil {
		return err
	}

	// Look up the selected variant, if any
	var variantVars map[string]string
	if variantName != "" {
		v, ok := req.Variants[variantName]
		if !ok {
			available := make([]string, 0, len(req.Variants))
			for name := range req.Variants {
				available = append(available, name)
			}
			sort.Strings(available)
			if len(available) == 0 {
				return fmt.Errorf("unknown variant %q (this request defines no variants)", variantName)
			}
			return fmt.Errorf("unknown variant %q (available: %s)", variantName, strings.Join(available, ", "))
		}
		variantVars = v
	}

	// Resolve variables
	resolved := interpolate.ResolveVars(cliVars, variantVars, nil, envVars)
```

- [ ] **Step 4: Add the new imports to `cmd/ws.go`**

In the import block of `cmd/ws.go`, add `"sort"` and `"strings"`:

```go
import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/taratani21/req/internal/interpolate"
	"github.com/taratani21/req/internal/loader"
	"github.com/taratani21/req/internal/ws"
)
```

- [ ] **Step 5: Run the new test to verify it passes**

Run: `go test -run TestWS_Variant -v -count=1`
Expected: PASS.

- [ ] **Step 6: Run the full test suite**

Run: `go test ./... -count=1`
Expected: PASS everywhere.

- [ ] **Step 7: Commit**

```bash
git add cmd/ws.go integration_test.go
git commit -m "feat(ws): select variant vars via --variant flag"
```

---

## Task 6: Wire `--variant` into `req chain` (silent skip on miss)

**Files:**
- Modify: `cmd/chain.go`
- Modify: `integration_test.go`

- [ ] **Step 1: Write failing integration test**

The test creates a two-step chain where only the second step defines the named variant. `--variant staging` should apply only to step 2; step 1 should run without variant vars and still succeed.

Append to `integration_test.go`:

```go
func TestChain_Variant_SilentSkip(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()

	// Step 1: no variants defined — uses base_url from env only
	writeFile(t, filepath.Join(dir, "login.toml"), `
name = "Login"
type = "http"
method = "POST"
url = "{{base_url}}/login"

[headers]
Content-Type = "application/json"

[body]
data = '{"username": "test"}'
`)

	// Step 2: defines [variants.staging] setting user_id=42
	writeFile(t, filepath.Join(dir, "get-user.toml"), `
name = "Get user"
type = "http"
method = "GET"
url = "{{base_url}}/users/{{user_id}}"

[headers]
Authorization = "Bearer {{token}}"

[variants.staging]
user_id = "42"
`)

	writeFile(t, filepath.Join(dir, "envs", "test.toml"), fmt.Sprintf(`
base_url = "%s"
`, server.URL))

	writeFile(t, filepath.Join(dir, "flow.chain.toml"), `
name = "Login + get user"

[[steps]]
request = "login.toml"
[steps.extract]
token = "access_token"

[[steps]]
request = "get-user.toml"
`)

	stdout, stderr, exitCode := runReq("chain", filepath.Join(dir, "flow.chain.toml"),
		"--env", "test", "--variant", "staging")
	if exitCode != 0 {
		t.Fatalf("exit code = %d, stderr: %s", exitCode, stderr)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", err, stdout)
	}
	if resp["id"] != "42" {
		t.Errorf("id = %v, want 42 (variant should supply user_id to step 2)", resp["id"])
	}
}
```

- [ ] **Step 2: Run the new test to verify it fails**

Run: `go test -run TestChain_Variant_SilentSkip -v -count=1`
Expected: FAIL — `{{user_id}}` unresolved in step 2 because chain doesn't yet apply the variant.

- [ ] **Step 3: Wire variant into `cmd/chain.go`**

In `cmd/chain.go`, inside the step loop, find the line:

```go
		// Resolve variables for this step
		resolved := interpolate.ResolveVars(cliVars, nil, extracted, envVars)
```

Replace it with:

```go
		// Look up the selected variant in this step's file (silent skip if absent)
		var variantVars map[string]string
		if variantName != "" {
			if v, ok := req.Variants[variantName]; ok {
				variantVars = v
			}
		}

		// Resolve variables for this step (cli > variant > extracted > env)
		resolved := interpolate.ResolveVars(cliVars, variantVars, extracted, envVars)
```

- [ ] **Step 4: Run the new test to verify it passes**

Run: `go test -run TestChain_Variant_SilentSkip -v -count=1`
Expected: PASS.

- [ ] **Step 5: Run the full test suite**

Run: `go test ./... -count=1`
Expected: PASS everywhere.

- [ ] **Step 6: Commit**

```bash
git add cmd/chain.go integration_test.go
git commit -m "feat(chain): apply --variant per step with silent skip"
```

---

## Task 7: Document variants in README

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add a new section after "Variable Interpolation" and before "Commands"**

Find the existing section header `## Commands` in `README.md`. Insert the following ABOVE it:

```markdown
## Variants

Variants are named presets of variables defined inside a request file. Use them when a request has a small set of parameterizations (e.g. `role = admin` vs `role = viewer`) that you don't want to spell out with `--var` every run and don't want to fork into separate files.

```toml
# chat.toml
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

Select one with `--variant <name>`:

```bash
req ws chat.toml --variant admin --env local
```

Variants slot into the resolution order between `--var` and extracted chain values:

1. `--var` flags
2. `--variant` vars
3. Extracted chain values
4. Env file

`req chain --variant <name>` applies to every step; if a step's request file doesn't define that variant, the step runs without variant vars (no error).
```

- [ ] **Step 2: Add `--variant` to the flag tables for `run`, `ws`, and `chain`**

Find each of the three flag tables in `README.md` (one per command) and add this row:

```
| `--variant <name>` | Select a named `[variants.<name>]` block from the request file |
```

- [ ] **Step 3: Add a `[variants.*]` mention to the File Formats section**

In the "HTTP Request" file format section of `README.md`, after the existing bullet for `[body]`, append:

```
- `[variants.<name>]`: optional named preset of variables (see [Variants](#variants))
```

Add the same bullet at the end of the "WebSocket Request" bullet list.

- [ ] **Step 4: Verify the README renders without broken references**

Run: `grep -n 'variant' README.md | head -20`
Expected: The new section, flag rows, and file-format bullets appear.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs: document request variants"
```

---

## Self-Review (completed by plan author)

- **Spec coverage:**
  - TOML syntax → Task 1 (loader field + fixture + test).
  - CLI flag → Task 3 (persistent flag in root.go).
  - Precedence order → Task 2 (ResolveVars signature + precedence test).
  - `run` behavior + unknown-variant error → Task 4.
  - `ws` behavior → Task 5.
  - `chain` silent-skip behavior → Task 6.
  - Testing section of spec → distributed across Tasks 1, 2, 4, 5, 6.
  - Documentation → Task 7.
- **Placeholder scan:** every code step has concrete code; every command shows expected output; no TBDs.
- **Type consistency:** `variantVars map[string]string` used consistently in all three cmds and in `ResolveVars`. The struct field is `Variants map[string]map[string]string` everywhere it appears.
