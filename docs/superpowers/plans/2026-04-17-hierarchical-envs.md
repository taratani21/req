# Hierarchical Env Resolution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `--env <name>` walk up the directory tree collecting every `envs/<name>.toml` and merge them with nearest-wins, stopping at `.requests/` (inclusive) or filesystem root.

**Architecture:** Add a new `loader.LoadEnvHierarchical(startDir, name string) (map[string]string, error)` that performs the walk, merge, and error reporting. Swap the three CLI command files (`run`, `ws`, `chain`) to call it instead of the current single-path `LoadEnv`. Existing `LoadEnv` stays as the per-file primitive and is called internally by `LoadEnvHierarchical`.

**Tech Stack:** Go 1.x, standard library (`os`, `path/filepath`, `fmt`), `github.com/BurntSushi/toml` (already in use), Cobra CLI (already in use).

---

## File Structure

Files created or modified by this plan:

- `internal/loader/toml.go` — add `LoadEnvHierarchical`. `LoadEnv` unchanged.
- `internal/loader/toml_test.go` — new tests for hierarchical resolution using `t.TempDir()` filesystem layouts.
- `cmd/run.go` — replace inline env-path construction with `LoadEnvHierarchical` call.
- `cmd/ws.go` — same swap.
- `cmd/chain.go` — same swap, passing `chainDir`.
- `README.md` — update the "Environment Files" section to describe the walk + merge; leave "Resolution order" untouched.

No new packages. No new files other than doc updates.

---

## Task 1: Happy-path single-file load

**Files:**
- Modify: `internal/loader/toml.go`
- Test: `internal/loader/toml_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/loader/toml_test.go`:

```go
func TestLoadEnvHierarchical_SingleFileAtStartDir(t *testing.T) {
	dir := t.TempDir()
	envsDir := filepath.Join(dir, "envs")
	if err := os.MkdirAll(envsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(envsDir, "local.toml"), []byte(`base_url = "http://localhost:8080"`), 0o644); err != nil {
		t.Fatal(err)
	}

	env, err := LoadEnvHierarchical(dir, "local")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["base_url"] != "http://localhost:8080" {
		t.Errorf("base_url = %q, want %q", env["base_url"], "http://localhost:8080")
	}
}
```

Also add these imports to the top of the test file if they are not already present:

```go
import (
	"os"
	"path/filepath"
	"testing"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/loader/ -run TestLoadEnvHierarchical_SingleFileAtStartDir -v`
Expected: FAIL — `undefined: LoadEnvHierarchical`

- [ ] **Step 3: Write minimal implementation**

In `internal/loader/toml.go`, add the `path/filepath` import (update the existing import block) and append:

```go
func LoadEnvHierarchical(startDir, name string) (map[string]string, error) {
	path := filepath.Join(startDir, "envs", name+".toml")
	return LoadEnv(path)
}
```

The updated import block:

```go
import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/loader/ -run TestLoadEnvHierarchical_SingleFileAtStartDir -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/loader/toml.go internal/loader/toml_test.go
git commit -m "feat(loader): add LoadEnvHierarchical skeleton

Loads a single envs/<name>.toml at the start directory."
```

---

## Task 2: Walk up when start dir has no envs/

**Files:**
- Modify: `internal/loader/toml.go`
- Test: `internal/loader/toml_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/loader/toml_test.go`:

```go
func TestLoadEnvHierarchical_FileAtAncestor(t *testing.T) {
	root := t.TempDir()
	envsDir := filepath.Join(root, "envs")
	if err := os.MkdirAll(envsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(envsDir, "local.toml"), []byte(`base_url = "http://root"`), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "users")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	env, err := LoadEnvHierarchical(sub, "local")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["base_url"] != "http://root" {
		t.Errorf("base_url = %q, want %q", env["base_url"], "http://root")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/loader/ -run TestLoadEnvHierarchical_FileAtAncestor -v`
Expected: FAIL — reading env file error (file doesn't exist at `users/envs/local.toml`).

- [ ] **Step 3: Extend the implementation**

In `internal/loader/toml.go`, replace the body of `LoadEnvHierarchical` with:

```go
func LoadEnvHierarchical(startDir, name string) (map[string]string, error) {
	filename := name + ".toml"
	dir := startDir
	for {
		path := filepath.Join(dir, "envs", filename)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return LoadEnv(path)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("no envs/%s.toml found in %s or any ancestor", name, startDir)
		}
		dir = parent
	}
}
```

- [ ] **Step 4: Run tests to verify both tests pass**

Run: `go test ./internal/loader/ -run TestLoadEnvHierarchical -v`
Expected: PASS for both `TestLoadEnvHierarchical_SingleFileAtStartDir` and `TestLoadEnvHierarchical_FileAtAncestor`.

- [ ] **Step 5: Commit**

```bash
git add internal/loader/toml.go internal/loader/toml_test.go
git commit -m "feat(loader): walk up to find envs/<name>.toml"
```

---

## Task 3: Merge two levels with nearest-wins

**Files:**
- Modify: `internal/loader/toml.go`
- Test: `internal/loader/toml_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/loader/toml_test.go`:

```go
func TestLoadEnvHierarchical_NearestWinsMerge(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "envs"), 0o755); err != nil {
		t.Fatal(err)
	}
	rootEnv := `base_url = "http://root"
token = "root-token"
`
	if err := os.WriteFile(filepath.Join(root, "envs", "local.toml"), []byte(rootEnv), 0o644); err != nil {
		t.Fatal(err)
	}

	admin := filepath.Join(root, "admin")
	if err := os.MkdirAll(filepath.Join(admin, "envs"), 0o755); err != nil {
		t.Fatal(err)
	}
	adminEnv := `token = "admin-token"`
	if err := os.WriteFile(filepath.Join(admin, "envs", "local.toml"), []byte(adminEnv), 0o644); err != nil {
		t.Fatal(err)
	}

	env, err := LoadEnvHierarchical(admin, "local")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["base_url"] != "http://root" {
		t.Errorf("base_url = %q, want %q", env["base_url"], "http://root")
	}
	if env["token"] != "admin-token" {
		t.Errorf("token = %q, want %q (nearest should win)", env["token"], "admin-token")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/loader/ -run TestLoadEnvHierarchical_NearestWinsMerge -v`
Expected: FAIL — current impl returns on the first match and never merges, so `base_url` will be missing.

- [ ] **Step 3: Extend the implementation**

In `internal/loader/toml.go`, replace `LoadEnvHierarchical` with:

```go
func LoadEnvHierarchical(startDir, name string) (map[string]string, error) {
	filename := name + ".toml"
	var found []string
	dir := startDir
	for {
		path := filepath.Join(dir, "envs", filename)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			found = append(found, path)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no envs/%s.toml found in %s or any ancestor", name, startDir)
	}
	merged := make(map[string]string)
	for i := len(found) - 1; i >= 0; i-- {
		e, err := LoadEnv(found[i])
		if err != nil {
			return nil, err
		}
		for k, v := range e {
			merged[k] = v
		}
	}
	return merged, nil
}
```

- [ ] **Step 4: Run all hierarchical tests to verify they pass**

Run: `go test ./internal/loader/ -run TestLoadEnvHierarchical -v`
Expected: PASS for all three hierarchical tests.

- [ ] **Step 5: Commit**

```bash
git add internal/loader/toml.go internal/loader/toml_test.go
git commit -m "feat(loader): merge hierarchical envs with nearest-wins"
```

---

## Task 4: Stop at `.requests/` boundary

**Files:**
- Modify: `internal/loader/toml.go`
- Test: `internal/loader/toml_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/loader/toml_test.go`:

```go
func TestLoadEnvHierarchical_StopsAtDotRequests(t *testing.T) {
	tmp := t.TempDir()
	// Layout:
	//   tmp/envs/local.toml        <- MUST NOT be loaded (above .requests/)
	//   tmp/.requests/envs/local.toml
	//   tmp/.requests/users/       <- start dir
	if err := os.MkdirAll(filepath.Join(tmp, "envs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "envs", "local.toml"), []byte(`outside = "leaked"`), 0o644); err != nil {
		t.Fatal(err)
	}

	requestsDir := filepath.Join(tmp, ".requests")
	if err := os.MkdirAll(filepath.Join(requestsDir, "envs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(requestsDir, "envs", "local.toml"), []byte(`base_url = "http://in-project"`), 0o644); err != nil {
		t.Fatal(err)
	}

	users := filepath.Join(requestsDir, "users")
	if err := os.MkdirAll(users, 0o755); err != nil {
		t.Fatal(err)
	}

	env, err := LoadEnvHierarchical(users, "local")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["base_url"] != "http://in-project" {
		t.Errorf("base_url = %q, want %q", env["base_url"], "http://in-project")
	}
	if _, ok := env["outside"]; ok {
		t.Errorf("outside key leaked from above .requests/: got %q", env["outside"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/loader/ -run TestLoadEnvHierarchical_StopsAtDotRequests -v`
Expected: FAIL — `outside` key will be present because the walk currently goes all the way to filesystem root.

- [ ] **Step 3: Extend the implementation**

In `internal/loader/toml.go`, replace `LoadEnvHierarchical` with the `.requests/` boundary version:

```go
func LoadEnvHierarchical(startDir, name string) (map[string]string, error) {
	filename := name + ".toml"
	var found []string
	hitRequestsBoundary := false
	dir := startDir
	for {
		path := filepath.Join(dir, "envs", filename)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			found = append(found, path)
		}
		if filepath.Base(dir) == ".requests" {
			hitRequestsBoundary = true
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if len(found) == 0 {
		if hitRequestsBoundary {
			return nil, fmt.Errorf("no envs/%s.toml found in %s or any ancestor up to .requests/", name, startDir)
		}
		return nil, fmt.Errorf("no envs/%s.toml found in %s or any ancestor", name, startDir)
	}
	merged := make(map[string]string)
	for i := len(found) - 1; i >= 0; i-- {
		e, err := LoadEnv(found[i])
		if err != nil {
			return nil, err
		}
		for k, v := range e {
			merged[k] = v
		}
	}
	return merged, nil
}
```

- [ ] **Step 4: Run all hierarchical tests to verify they pass**

Run: `go test ./internal/loader/ -run TestLoadEnvHierarchical -v`
Expected: PASS for all four hierarchical tests.

- [ ] **Step 5: Commit**

```bash
git add internal/loader/toml.go internal/loader/toml_test.go
git commit -m "feat(loader): stop env walk at .requests/ boundary"
```

---

## Task 5: Three-level merge ordering regression test

**Files:**
- Test: `internal/loader/toml_test.go`

This task adds a regression test to lock in the load order for three nested levels. Implementation should already pass.

- [ ] **Step 1: Write the test**

Append to `internal/loader/toml_test.go`:

```go
func TestLoadEnvHierarchical_ThreeLevelMerge(t *testing.T) {
	tmp := t.TempDir()
	requestsDir := filepath.Join(tmp, ".requests")

	// Level 1 (outermost): .requests/envs/local.toml
	if err := os.MkdirAll(filepath.Join(requestsDir, "envs"), 0o755); err != nil {
		t.Fatal(err)
	}
	rootEnv := `a = "from-root"
b = "from-root"
c = "from-root"
`
	if err := os.WriteFile(filepath.Join(requestsDir, "envs", "local.toml"), []byte(rootEnv), 0o644); err != nil {
		t.Fatal(err)
	}

	// Level 2: .requests/users/envs/local.toml
	users := filepath.Join(requestsDir, "users")
	if err := os.MkdirAll(filepath.Join(users, "envs"), 0o755); err != nil {
		t.Fatal(err)
	}
	midEnv := `b = "from-users"
c = "from-users"
`
	if err := os.WriteFile(filepath.Join(users, "envs", "local.toml"), []byte(midEnv), 0o644); err != nil {
		t.Fatal(err)
	}

	// Level 3 (start dir): .requests/users/admin/envs/local.toml
	admin := filepath.Join(users, "admin")
	if err := os.MkdirAll(filepath.Join(admin, "envs"), 0o755); err != nil {
		t.Fatal(err)
	}
	leafEnv := `c = "from-admin"`
	if err := os.WriteFile(filepath.Join(admin, "envs", "local.toml"), []byte(leafEnv), 0o644); err != nil {
		t.Fatal(err)
	}

	env, err := LoadEnvHierarchical(admin, "local")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["a"] != "from-root" {
		t.Errorf("a = %q, want %q (only defined at root)", env["a"], "from-root")
	}
	if env["b"] != "from-users" {
		t.Errorf("b = %q, want %q (users overrides root)", env["b"], "from-users")
	}
	if env["c"] != "from-admin" {
		t.Errorf("c = %q, want %q (admin overrides users and root)", env["c"], "from-admin")
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./internal/loader/ -run TestLoadEnvHierarchical_ThreeLevelMerge -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/loader/toml_test.go
git commit -m "test(loader): lock in three-level hierarchical env merge order"
```

---

## Task 6: No file found → error

**Files:**
- Test: `internal/loader/toml_test.go`

Implementation already returns the error; this task documents the message shape.

- [ ] **Step 1: Write the test**

Append to `internal/loader/toml_test.go`:

```go
func TestLoadEnvHierarchical_NoFileFound_WithRequestsBoundary(t *testing.T) {
	tmp := t.TempDir()
	requestsDir := filepath.Join(tmp, ".requests")
	users := filepath.Join(requestsDir, "users")
	if err := os.MkdirAll(users, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := LoadEnvHierarchical(users, "missing")
	if err == nil {
		t.Fatal("expected error for missing env, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "no envs/missing.toml found") {
		t.Errorf("error missing prefix: %q", msg)
	}
	if !strings.Contains(msg, ".requests/") {
		t.Errorf("error should mention .requests/ boundary: %q", msg)
	}
}

func TestLoadEnvHierarchical_NoFileFound_NoRequestsBoundary(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := LoadEnvHierarchical(sub, "missing")
	if err == nil {
		t.Fatal("expected error for missing env, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "no envs/missing.toml found") {
		t.Errorf("error missing prefix: %q", msg)
	}
	if strings.Contains(msg, ".requests/") {
		t.Errorf("error should NOT mention .requests/ when boundary not hit: %q", msg)
	}
}
```

Add `"strings"` to the test file's imports if not already present.

- [ ] **Step 2: Run tests to verify they pass**

Run: `go test ./internal/loader/ -run TestLoadEnvHierarchical_NoFileFound -v`
Expected: PASS for both.

- [ ] **Step 3: Commit**

```bash
git add internal/loader/toml_test.go
git commit -m "test(loader): cover hierarchical env not-found error shapes"
```

---

## Task 7: Parse error propagates with path

**Files:**
- Test: `internal/loader/toml_test.go`

- [ ] **Step 1: Write the test**

Append to `internal/loader/toml_test.go`:

```go
func TestLoadEnvHierarchical_ParseErrorIncludesPath(t *testing.T) {
	tmp := t.TempDir()
	envsDir := filepath.Join(tmp, "envs")
	if err := os.MkdirAll(envsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(envsDir, "local.toml")
	if err := os.WriteFile(bad, []byte("this is not = valid = toml\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadEnvHierarchical(tmp, "local")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), bad) {
		t.Errorf("error should name offending path %q, got %q", bad, err.Error())
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test ./internal/loader/ -run TestLoadEnvHierarchical_ParseErrorIncludesPath -v`
Expected: PASS — `LoadEnv` already formats `parsing env file <path>: ...`, and `LoadEnvHierarchical` returns that error unchanged.

- [ ] **Step 3: Run the full loader test suite for sanity**

Run: `go test ./internal/loader/ -v`
Expected: PASS for every test, including the existing non-hierarchical ones.

- [ ] **Step 4: Commit**

```bash
git add internal/loader/toml_test.go
git commit -m "test(loader): verify hierarchical env parse errors include path"
```

---

## Task 8: Wire into `req run`

**Files:**
- Modify: `cmd/run.go:39-46`

- [ ] **Step 1: Read the current env-loading block in `cmd/run.go`**

Locate the block (lines ~39–46):

```go
// Load env file if specified
var envVars map[string]string
if envName != "" {
	envPath := filepath.Join(filepath.Dir(reqFile), "envs", envName+".toml")
	envVars, err = loader.LoadEnv(envPath)
	if err != nil {
		return fmt.Errorf("loading env %q: %w", envName, err)
	}
}
```

- [ ] **Step 2: Replace it with the hierarchical call**

Replace that block with:

```go
// Load env file if specified
var envVars map[string]string
if envName != "" {
	envVars, err = loader.LoadEnvHierarchical(filepath.Dir(reqFile), envName)
	if err != nil {
		return fmt.Errorf("loading env %q: %w", envName, err)
	}
}
```

Leave the `path/filepath` import in place — `filepath.Dir` is still used.

- [ ] **Step 3: Build to verify it compiles**

Run: `go build ./...`
Expected: no output, exit 0.

- [ ] **Step 4: Run the full test suite**

Run: `go test ./... -v`
Expected: PASS for everything.

- [ ] **Step 5: Manual smoke test**

```bash
mkdir -p /tmp/req-smoke/.requests/envs /tmp/req-smoke/.requests/users
cat >/tmp/req-smoke/.requests/envs/local.toml <<'EOF'
base_url = "https://httpbin.org"
EOF
cat >/tmp/req-smoke/.requests/users/get.toml <<'EOF'
name = "smoke"
type = "http"
method = "GET"
url = "{{base_url}}/get"
EOF
go run . run /tmp/req-smoke/.requests/users/get.toml --env local
```

Expected: a JSON response from httpbin.org with exit 0. (Requires network.)

If offline: construct a request pointing to a local server, or accept the build + automated tests as sufficient verification and skip this step.

- [ ] **Step 6: Commit**

```bash
git add cmd/run.go
git commit -m "feat(run): use hierarchical env resolution"
```

---

## Task 9: Wire into `req ws`

**Files:**
- Modify: `cmd/ws.go:43-50`

- [ ] **Step 1: Read the current env-loading block in `cmd/ws.go`**

Locate the block (lines ~43–50):

```go
// Load env file if specified
var envVars map[string]string
if envName != "" {
	envPath := filepath.Join(filepath.Dir(reqFile), "envs", envName+".toml")
	envVars, err = loader.LoadEnv(envPath)
	if err != nil {
		return fmt.Errorf("loading env %q: %w", envName, err)
	}
}
```

- [ ] **Step 2: Replace it with the hierarchical call**

Replace that block with:

```go
// Load env file if specified
var envVars map[string]string
if envName != "" {
	envVars, err = loader.LoadEnvHierarchical(filepath.Dir(reqFile), envName)
	if err != nil {
		return fmt.Errorf("loading env %q: %w", envName, err)
	}
}
```

- [ ] **Step 3: Build to verify it compiles**

Run: `go build ./...`
Expected: no output, exit 0.

- [ ] **Step 4: Run the full test suite**

Run: `go test ./... -v`
Expected: PASS for everything.

- [ ] **Step 5: Commit**

```bash
git add cmd/ws.go
git commit -m "feat(ws): use hierarchical env resolution"
```

---

## Task 10: Wire into `req chain`

**Files:**
- Modify: `cmd/chain.go:36-44`

- [ ] **Step 1: Read the current env-loading block in `cmd/chain.go`**

Locate the block (lines ~36–44):

```go
// Load env file if specified
var envVars map[string]string
if envName != "" {
	envPath := filepath.Join(chainDir, "envs", envName+".toml")
	envVars, err = loader.LoadEnv(envPath)
	if err != nil {
		return fmt.Errorf("loading env %q: %w", envName, err)
	}
}
```

- [ ] **Step 2: Replace it with the hierarchical call**

Replace that block with:

```go
// Load env file if specified
var envVars map[string]string
if envName != "" {
	envVars, err = loader.LoadEnvHierarchical(chainDir, envName)
	if err != nil {
		return fmt.Errorf("loading env %q: %w", envName, err)
	}
}
```

- [ ] **Step 3: Build to verify it compiles**

Run: `go build ./...`
Expected: no output, exit 0.

- [ ] **Step 4: Run the full test suite**

Run: `go test ./... -v`
Expected: PASS for everything, including `integration_test.go`.

- [ ] **Step 5: Commit**

```bash
git add cmd/chain.go
git commit -m "feat(chain): use hierarchical env resolution"
```

---

## Task 11: Update README

**Files:**
- Modify: `README.md` — "Environment Files" section (lines ~78–97)

- [ ] **Step 1: Read the current "Environment Files" section**

Locate the section that begins with `## Environment Files` and ends just before `## Variable Interpolation`.

- [ ] **Step 2: Replace it with the updated section**

Replace the entire `## Environment Files` section with:

```markdown
## Environment Files

Environment files hold variables that change between environments (local, staging, prod). They live in an `envs/` directory and are loaded with `--env <name>`.

```toml
# .requests/envs/staging.toml
base_url = "https://staging.api.example.com"
token = "staging-token-xyz"
user_id = "42"
```

### Hierarchical lookup

`req` walks upward from the request file's directory, collecting every `envs/<name>.toml` it finds. The walk stops after processing a `.requests/` directory (inclusive), or at filesystem root. All found files are merged into one set of variables; nearer files override farther ones per-key.

```
.requests/
  envs/
    local.toml        # base_url, token
  admin/
    envs/
      local.toml      # token (overrides root's token)
    delete-user.toml
```

Running `req run .requests/admin/delete-user.toml --env local` loads both files: `base_url` comes from the root, `token` comes from `admin/` (nearest wins).

If `--env <name>` is passed but no matching file exists anywhere in the walk, `req` exits with an error naming the search path.

### Keep secrets out of git

Environment files typically contain tokens and other secrets. Exclude them from git:

```bash
echo ".requests/envs/" >> .git/info/exclude
```

This is local to your clone. The request templates themselves (which contain `{{variables}}`, not actual secrets) can be committed and shared.
```

- [ ] **Step 3: Verify the Resolution order section is unchanged**

The "### Resolution order" subsection under "## Variable Interpolation" should remain untouched. Read the file and confirm that `--var` > `--variant` > extracted > env order is still listed as-is.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs: describe hierarchical env file lookup"
```

---

## Self-Review

The final commit should leave the repo in this state:

- `LoadEnvHierarchical` exists in `internal/loader/toml.go`, walking up from `startDir`, stopping at `.requests/` or filesystem root, merging files farthest→nearest.
- Seven new tests in `internal/loader/toml_test.go` cover: single file at start dir, file at ancestor, two-level merge, three-level merge, `.requests/` boundary, not-found errors (two shapes), parse error propagation.
- `cmd/run.go`, `cmd/ws.go`, `cmd/chain.go` call `loader.LoadEnvHierarchical` with the same wrap/error message as before.
- `README.md`'s Environment Files section describes the hierarchical walk with an example.
- `go test ./...` passes end-to-end.
