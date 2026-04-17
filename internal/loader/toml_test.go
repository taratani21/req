package loader

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRequest_SimpleGet(t *testing.T) {
	req, err := LoadRequest("../../testdata/requests/simple-get.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Name != "Simple GET" {
		t.Errorf("name = %q, want %q", req.Name, "Simple GET")
	}
	if req.Type != "http" {
		t.Errorf("type = %q, want %q", req.Type, "http")
	}
	if req.Method != "GET" {
		t.Errorf("method = %q, want %q", req.Method, "GET")
	}
	if req.URL != "https://api.example.com/health" {
		t.Errorf("url = %q, want %q", req.URL, "https://api.example.com/health")
	}
}

func TestLoadRequest_PostWithBody(t *testing.T) {
	req, err := LoadRequest("../../testdata/requests/post-with-body.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Method != "POST" {
		t.Errorf("method = %q, want %q", req.Method, "POST")
	}
	if req.Headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type = %q, want %q", req.Headers["Content-Type"], "application/json")
	}
	if req.Headers["Authorization"] != "Bearer abc123" {
		t.Errorf("Authorization = %q, want %q", req.Headers["Authorization"], "Bearer abc123")
	}
	if req.Query["verbose"] != "true" {
		t.Errorf("query verbose = %q, want %q", req.Query["verbose"], "true")
	}
	if req.Body.Data == "" {
		t.Error("body data should not be empty")
	}
}

func TestLoadRequest_WithVariables(t *testing.T) {
	req, err := LoadRequest("../../testdata/requests/with-vars.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.URL != "{{base_url}}/users/{{user_id}}" {
		t.Errorf("url = %q, want %q", req.URL, "{{base_url}}/users/{{user_id}}")
	}
	if req.Headers["Authorization"] != "Bearer {{token}}" {
		t.Errorf("Authorization = %q, want %q", req.Headers["Authorization"], "Bearer {{token}}")
	}
}

func TestLoadRequest_WebSocket(t *testing.T) {
	req, err := LoadRequest("../../testdata/requests/ws-simple.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Type != "websocket" {
		t.Errorf("type = %q, want %q", req.Type, "websocket")
	}
	if req.URL != "wss://ws.example.com/events" {
		t.Errorf("url = %q, want %q", req.URL, "wss://ws.example.com/events")
	}
}

func TestLoadRequest_WebSocketWithMessages(t *testing.T) {
	req, err := LoadRequest("../../testdata/requests/ws-with-messages.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(req.Messages) != 2 {
		t.Fatalf("messages count = %d, want 2", len(req.Messages))
	}
	if req.Messages[0].AwaitResponse != true {
		t.Error("first message should have await_response = true")
	}
	if req.Messages[1].AwaitResponse != false {
		t.Error("second message should have await_response = false")
	}
}

func TestLoadRequest_FileNotFound(t *testing.T) {
	_, err := LoadRequest("../../testdata/requests/nonexistent.toml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadEnv(t *testing.T) {
	env, err := LoadEnv("../../testdata/envs/test.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env["base_url"] != "https://api.example.com" {
		t.Errorf("base_url = %q, want %q", env["base_url"], "https://api.example.com")
	}
	if env["token"] != "test-token-123" {
		t.Errorf("token = %q, want %q", env["token"], "test-token-123")
	}
	if env["user_id"] != "42" {
		t.Errorf("user_id = %q, want %q", env["user_id"], "42")
	}
}

func TestLoadEnv_FileNotFound(t *testing.T) {
	_, err := LoadEnv("../../testdata/envs/nonexistent.toml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadChain(t *testing.T) {
	chain, err := LoadChain("../../testdata/requests/chain-simple.chain.toml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chain.Name != "Login and get profile" {
		t.Errorf("name = %q, want %q", chain.Name, "Login and get profile")
	}
	if len(chain.Steps) != 2 {
		t.Fatalf("steps count = %d, want 2", len(chain.Steps))
	}
	if chain.Steps[0].Request != "auth/login.toml" {
		t.Errorf("step 0 request = %q, want %q", chain.Steps[0].Request, "auth/login.toml")
	}
	if chain.Steps[0].Extract["token"] != "access_token" {
		t.Errorf("step 0 extract token = %q, want %q", chain.Steps[0].Extract["token"], "access_token")
	}
	if chain.Steps[1].Request != "users/get-profile.toml" {
		t.Errorf("step 1 request = %q, want %q", chain.Steps[1].Request, "users/get-profile.toml")
	}
	if chain.Steps[1].Extract != nil && len(chain.Steps[1].Extract) != 0 {
		t.Errorf("step 1 should have no extract, got %v", chain.Steps[1].Extract)
	}
}

func TestLoadChain_FileNotFound(t *testing.T) {
	_, err := LoadChain("../../testdata/requests/nonexistent.chain.toml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

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
