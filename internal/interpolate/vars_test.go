package interpolate

import (
	"testing"
)

func TestInterpolate_SimpleReplacement(t *testing.T) {
	vars := map[string]string{
		"base_url": "https://api.example.com",
		"user_id":  "42",
	}
	result, err := Interpolate("{{base_url}}/users/{{user_id}}", vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "https://api.example.com/users/42" {
		t.Errorf("result = %q, want %q", result, "https://api.example.com/users/42")
	}
}

func TestInterpolate_NoVariables(t *testing.T) {
	result, err := Interpolate("https://api.example.com/health", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "https://api.example.com/health" {
		t.Errorf("result = %q, want %q", result, "https://api.example.com/health")
	}
}

func TestInterpolate_UnresolvedVariable(t *testing.T) {
	vars := map[string]string{
		"base_url": "https://api.example.com",
	}
	_, err := Interpolate("{{base_url}}/users/{{user_id}}", vars)
	if err == nil {
		t.Error("expected error for unresolved variable")
	}
}

func TestInterpolate_MultipleOccurrences(t *testing.T) {
	vars := map[string]string{"id": "42"}
	result, err := Interpolate("{{id}}/{{id}}/{{id}}", vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "42/42/42" {
		t.Errorf("result = %q, want %q", result, "42/42/42")
	}
}

func TestInterpolate_EmptyString(t *testing.T) {
	result, err := Interpolate("", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("result = %q, want empty string", result)
	}
}

func TestFindVariables(t *testing.T) {
	vars := FindVariables("{{base_url}}/users/{{user_id}}")
	if len(vars) != 2 {
		t.Fatalf("found %d variables, want 2", len(vars))
	}
	expected := map[string]bool{"base_url": true, "user_id": true}
	for _, v := range vars {
		if !expected[v] {
			t.Errorf("unexpected variable: %q", v)
		}
	}
}

func TestResolveVars_Priority(t *testing.T) {
	cliVars := map[string]string{"token": "cli-token", "id": "cli-id"}
	extracted := map[string]string{"token": "extracted-token", "name": "extracted-name"}
	envVars := map[string]string{"token": "env-token", "name": "env-name", "base": "env-base"}

	resolved := ResolveVars(cliVars, extracted, envVars)

	if resolved["token"] != "cli-token" {
		t.Errorf("token = %q, want %q (cli wins)", resolved["token"], "cli-token")
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
