package main_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all integration tests
	tmpDir, err := os.MkdirTemp("", "req-integration-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "req")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "building binary: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// runReq runs the req binary with the given args and returns stdout, stderr, and exit code.
func runReq(args ...string) (stdout, stderr string, exitCode int) {
	cmd := exec.Command(binaryPath, args...)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return outBuf.String(), errBuf.String(), exitCode
}

// writeFile creates a file at path with the given content, creating parent dirs as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("creating dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// newTestServer creates an httptest server that responds based on the request path.
// GET /echo returns request details as JSON.
// POST /echo returns the request body back.
// GET /users/:id returns a user JSON.
// POST /login returns an access token.
// Any path under /status/<code> returns that status code.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/echo" && r.Method == "GET":
			resp := map[string]interface{}{
				"method":  r.Method,
				"path":    r.URL.Path,
				"query":   r.URL.Query(),
				"headers": flattenHeaders(r.Header),
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)

		case r.URL.Path == "/echo" && r.Method == "POST":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			// Echo back the body
			buf := make([]byte, 4096)
			n, _ := r.Body.Read(buf)
			w.Write(buf[:n])

		case r.URL.Path == "/login" && r.Method == "POST":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"access_token": "test-jwt-token-123", "expires_in": 3600}`))

		case r.URL.Path == "/users/42":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id": "42", "name": "John Doe", "email": "john@example.com"}`))

		case r.URL.Path == "/users" && r.Method == "POST":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			w.Write([]byte(`{"id": "99", "name": "New User"}`))

		case strings.HasPrefix(r.URL.Path, "/status/"):
			code := 500
			fmt.Sscanf(r.URL.Path, "/status/%d", &code)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			w.Write([]byte(fmt.Sprintf(`{"status": %d}`, code)))

		default:
			w.WriteHeader(404)
			w.Write([]byte(`{"error": "not found"}`))
		}
	}))
}

func flattenHeaders(h http.Header) map[string]string {
	result := make(map[string]string)
	for k, v := range h {
		result[k] = v[0]
	}
	return result
}

// --- req run tests ---

func TestRun_BasicGET(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "get.toml"), fmt.Sprintf(`
name = "Basic GET"
type = "http"
method = "GET"
url = "%s/echo"
`, server.URL))

	stdout, _, exitCode := runReq("run", filepath.Join(dir, "get.toml"))

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v\nstdout: %s", err, stdout)
	}
	if resp["method"] != "GET" {
		t.Errorf("method = %v, want GET", resp["method"])
	}
}

func TestRun_WithEnvFile(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "get.toml"), `
name = "GET with env"
type = "http"
method = "GET"
url = "{{base_url}}/users/42"
`)
	writeFile(t, filepath.Join(dir, "envs", "test.toml"), fmt.Sprintf(`
base_url = "%s"
`, server.URL))

	stdout, _, exitCode := runReq("run", filepath.Join(dir, "get.toml"), "--env", "test")

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", err, stdout)
	}
	if resp["name"] != "John Doe" {
		t.Errorf("name = %v, want John Doe", resp["name"])
	}
}

func TestRun_WithVarFlag(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "get.toml"), `
name = "GET with var"
type = "http"
method = "GET"
url = "{{base_url}}/users/42"
`)

	stdout, _, exitCode := runReq("run", filepath.Join(dir, "get.toml"),
		"--var", fmt.Sprintf("base_url=%s", server.URL))

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", err, stdout)
	}
	if resp["id"] != "42" {
		t.Errorf("id = %v, want 42", resp["id"])
	}
}

func TestRun_VarOverridesEnv(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "get.toml"), `
name = "GET var override"
type = "http"
method = "GET"
url = "{{base_url}}/users/{{user_id}}"
`)
	writeFile(t, filepath.Join(dir, "envs", "test.toml"), fmt.Sprintf(`
base_url = "%s"
user_id = "999"
`, server.URL))

	// --var user_id=42 should override env's user_id=999
	stdout, _, exitCode := runReq("run", filepath.Join(dir, "get.toml"),
		"--env", "test", "--var", "user_id=42")

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", err, stdout)
	}
	// Server returns user 42 data, not 999
	if resp["id"] != "42" {
		t.Errorf("id = %v, want 42 (--var should override env)", resp["id"])
	}
}

func TestRun_UnresolvedVariable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "get.toml"), `
name = "Unresolved var"
type = "http"
method = "GET"
url = "{{base_url}}/users/{{user_id}}"

[headers]
Authorization = "Bearer {{token}}"
`)

	_, stderr, exitCode := runReq("run", filepath.Join(dir, "get.toml"))

	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for unresolved variable")
	}
	if !strings.Contains(stderr, "unresolved variable") {
		t.Errorf("stderr should mention unresolved variable, got: %s", stderr)
	}
	if !strings.Contains(stderr, "--var") {
		t.Errorf("stderr should include hint about --var, got: %s", stderr)
	}
}

func TestRun_WrongType(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "ws.toml"), `
name = "WebSocket"
type = "websocket"
url = "ws://localhost/ws"
`)

	_, stderr, exitCode := runReq("run", filepath.Join(dir, "ws.toml"))

	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for wrong type")
	}
	if !strings.Contains(stderr, "websocket") {
		t.Errorf("stderr should mention websocket type, got: %s", stderr)
	}
}

func TestRun_PostWithBody(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "post.toml"), fmt.Sprintf(`
name = "POST with body"
type = "http"
method = "POST"
url = "%s/users"

[headers]
Content-Type = "application/json"

[body]
data = '{"name": "New User"}'
`, server.URL))

	stdout, _, exitCode := runReq("run", filepath.Join(dir, "post.toml"))

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", err, stdout)
	}
	if resp["id"] != "99" {
		t.Errorf("id = %v, want 99", resp["id"])
	}
}

func TestRun_VerboseToStderr(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "get.toml"), fmt.Sprintf(`
name = "Verbose test"
type = "http"
method = "GET"
url = "%s/users/42"
`, server.URL))

	stdout, stderr, exitCode := runReq("run", filepath.Join(dir, "get.toml"), "--verbose")

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	// stdout should be clean JSON (no verbose output mixed in)
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("stdout should be clean JSON: %v\nstdout: %s", err, stdout)
	}

	// stderr should contain verbose details
	if !strings.Contains(stderr, "GET") {
		t.Errorf("stderr should contain method, got: %s", stderr)
	}
	if !strings.Contains(stderr, "HTTP") {
		t.Errorf("stderr should contain HTTP status, got: %s", stderr)
	}
}

func TestRun_4xxExitCode(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "get404.toml"), fmt.Sprintf(`
name = "404 test"
type = "http"
method = "GET"
url = "%s/status/404"
`, server.URL))

	stdout, _, exitCode := runReq("run", filepath.Join(dir, "get404.toml"))

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1 for 4xx", exitCode)
	}

	// Body should still be on stdout
	if !strings.Contains(stdout, "404") {
		t.Errorf("stdout should contain response body, got: %s", stdout)
	}
}

func TestRun_5xxExitCode(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "get500.toml"), fmt.Sprintf(`
name = "500 test"
type = "http"
method = "GET"
url = "%s/status/500"
`, server.URL))

	_, _, exitCode := runReq("run", filepath.Join(dir, "get500.toml"))

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1 for 5xx", exitCode)
	}
}

func TestRun_QueryParams(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "query.toml"), fmt.Sprintf(`
name = "Query params"
type = "http"
method = "GET"
url = "%s/echo"

[query]
foo = "bar"
page = "2"
`, server.URL))

	stdout, _, exitCode := runReq("run", filepath.Join(dir, "query.toml"))

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	query, ok := resp["query"].(map[string]interface{})
	if !ok {
		t.Fatalf("query not a map: %v", resp["query"])
	}
	// httptest returns query values as arrays
	fooVal, ok := query["foo"].([]interface{})
	if !ok || len(fooVal) == 0 || fooVal[0] != "bar" {
		t.Errorf("query foo = %v, want [bar]", query["foo"])
	}
}

func TestRun_FileNotFound(t *testing.T) {
	_, stderr, exitCode := runReq("run", "/nonexistent/path/request.toml")

	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for missing file")
	}
	if !strings.Contains(stderr, "no such file") && !strings.Contains(stderr, "not exist") {
		t.Errorf("stderr should mention file not found, got: %s", stderr)
	}
}

func TestRun_MalformedToml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.toml"), `this is not valid toml [[[`)

	_, stderr, exitCode := runReq("run", filepath.Join(dir, "bad.toml"))

	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for malformed TOML")
	}
	if stderr == "" {
		t.Error("stderr should contain error message")
	}
}

func TestRun_NoArgs(t *testing.T) {
	_, stderr, exitCode := runReq("run")

	if exitCode == 0 {
		t.Fatal("expected non-zero exit code when no file argument provided")
	}
	if !strings.Contains(stderr, "accepts 1 arg") {
		t.Errorf("stderr should mention arg requirement, got: %s", stderr)
	}
}

// --- req chain tests ---

func TestChain_BasicWithExtraction(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()

	// Step 1: login to get a token
	writeFile(t, filepath.Join(dir, "login.toml"), fmt.Sprintf(`
name = "Login"
type = "http"
method = "POST"
url = "%s/login"

[headers]
Content-Type = "application/json"

[body]
data = '{"username": "test"}'
`, server.URL))

	// Step 2: get user profile using extracted token
	writeFile(t, filepath.Join(dir, "get-user.toml"), fmt.Sprintf(`
name = "Get user"
type = "http"
method = "GET"
url = "%s/users/42"

[headers]
Authorization = "Bearer {{token}}"
`, server.URL))

	// Chain file
	writeFile(t, filepath.Join(dir, "flow.chain.toml"), `
name = "Login and get user"

[[steps]]
request = "login.toml"
[steps.extract]
token = "access_token"

[[steps]]
request = "get-user.toml"
`)

	stdout, _, exitCode := runReq("chain", filepath.Join(dir, "flow.chain.toml"))

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	// Should get the last step's response (user profile)
	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", err, stdout)
	}
	if resp["name"] != "John Doe" {
		t.Errorf("name = %v, want John Doe", resp["name"])
	}
}

func TestChain_OnlyLastStepToStdout(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "login.toml"), fmt.Sprintf(`
name = "Login"
type = "http"
method = "POST"
url = "%s/login"

[headers]
Content-Type = "application/json"

[body]
data = '{"username": "test"}'
`, server.URL))

	writeFile(t, filepath.Join(dir, "get-user.toml"), fmt.Sprintf(`
name = "Get user"
type = "http"
method = "GET"
url = "%s/users/42"

[headers]
Authorization = "Bearer {{token}}"
`, server.URL))

	writeFile(t, filepath.Join(dir, "flow.chain.toml"), `
name = "Login and get user"

[[steps]]
request = "login.toml"
[steps.extract]
token = "access_token"

[[steps]]
request = "get-user.toml"
`)

	stdout, _, _ := runReq("chain", filepath.Join(dir, "flow.chain.toml"))

	// stdout should NOT contain the login response (access_token)
	if strings.Contains(stdout, "access_token") {
		t.Errorf("stdout should only contain last step's response, but found login response: %s", stdout)
	}
	// stdout SHOULD contain the user profile
	if !strings.Contains(stdout, "John Doe") {
		t.Errorf("stdout should contain last step's response (user profile), got: %s", stdout)
	}
}

func TestChain_FailsOnNon2xx(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "fail.toml"), fmt.Sprintf(`
name = "Fail"
type = "http"
method = "GET"
url = "%s/status/401"
`, server.URL))

	writeFile(t, filepath.Join(dir, "ok.toml"), fmt.Sprintf(`
name = "OK"
type = "http"
method = "GET"
url = "%s/users/42"
`, server.URL))

	writeFile(t, filepath.Join(dir, "flow.chain.toml"), `
name = "Fail fast"

[[steps]]
request = "fail.toml"

[[steps]]
request = "ok.toml"
`)

	stdout, _, exitCode := runReq("chain", filepath.Join(dir, "flow.chain.toml"))

	if exitCode != 1 {
		t.Fatalf("exit code = %d, want 1 (first step should fail)", exitCode)
	}

	// Should output the failed step's response
	if !strings.Contains(stdout, "401") {
		t.Errorf("stdout should contain failed response, got: %s", stdout)
	}
}

func TestChain_WithEnvFile(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "get.toml"), `
name = "Get user"
type = "http"
method = "GET"
url = "{{base_url}}/users/42"
`)
	writeFile(t, filepath.Join(dir, "envs", "test.toml"), fmt.Sprintf(`
base_url = "%s"
`, server.URL))

	writeFile(t, filepath.Join(dir, "flow.chain.toml"), `
name = "Simple chain"

[[steps]]
request = "get.toml"
`)

	stdout, _, exitCode := runReq("chain", filepath.Join(dir, "flow.chain.toml"), "--env", "test")

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", err, stdout)
	}
	if resp["id"] != "42" {
		t.Errorf("id = %v, want 42", resp["id"])
	}
}

func TestChain_MissingRequestFile(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "flow.chain.toml"), `
name = "Missing ref"

[[steps]]
request = "nonexistent.toml"
`)

	_, stderr, exitCode := runReq("chain", filepath.Join(dir, "flow.chain.toml"))

	if exitCode == 0 {
		t.Fatal("expected non-zero exit code for missing request file")
	}
	if !strings.Contains(stderr, "not found") {
		t.Errorf("stderr should mention file not found, got: %s", stderr)
	}
}

func TestChain_VerboseShowsAllSteps(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "login.toml"), fmt.Sprintf(`
name = "Login"
type = "http"
method = "POST"
url = "%s/login"

[headers]
Content-Type = "application/json"

[body]
data = '{"username": "test"}'
`, server.URL))

	writeFile(t, filepath.Join(dir, "get-user.toml"), fmt.Sprintf(`
name = "Get user"
type = "http"
method = "GET"
url = "%s/users/42"

[headers]
Authorization = "Bearer {{token}}"
`, server.URL))

	writeFile(t, filepath.Join(dir, "flow.chain.toml"), `
name = "Verbose chain"

[[steps]]
request = "login.toml"
[steps.extract]
token = "access_token"

[[steps]]
request = "get-user.toml"
`)

	_, stderr, exitCode := runReq("chain", filepath.Join(dir, "flow.chain.toml"), "--verbose")

	if exitCode != 0 {
		t.Fatalf("exit code = %d, want 0", exitCode)
	}

	if !strings.Contains(stderr, "Step 1") {
		t.Errorf("stderr should contain Step 1, got: %s", stderr)
	}
	if !strings.Contains(stderr, "Step 2") {
		t.Errorf("stderr should contain Step 2, got: %s", stderr)
	}
}

// --- req init tests ---

func TestInit_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()

	cmd := exec.Command(binaryPath, "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("init failed: %v\noutput: %s", err, out)
	}

	// Verify directory structure
	entries := []string{
		filepath.Join(dir, ".requests"),
		filepath.Join(dir, ".requests", "envs"),
		filepath.Join(dir, ".requests", "envs", "local.toml"),
		filepath.Join(dir, ".requests", "example-http.toml"),
		filepath.Join(dir, ".requests", "example-ws.toml"),
	}
	for _, path := range entries {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
		}
	}
}

func TestInit_ErrorIfAlreadyExists(t *testing.T) {
	dir := t.TempDir()

	// Create .requests/ first
	os.MkdirAll(filepath.Join(dir, ".requests"), 0755)

	cmd := exec.Command(binaryPath, "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatal("expected error when .requests/ already exists")
	}
	if !strings.Contains(string(out), "already exists") {
		t.Errorf("output should mention already exists, got: %s", out)
	}
}
