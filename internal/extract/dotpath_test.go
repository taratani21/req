package extract

import (
	"testing"
)

func TestExtract_TopLevel(t *testing.T) {
	json := []byte(`{"access_token": "abc123", "expires_in": 3600}`)
	val, err := DotPath(json, "access_token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "abc123" {
		t.Errorf("val = %q, want %q", val, "abc123")
	}
}

func TestExtract_Nested(t *testing.T) {
	json := []byte(`{"data": {"user": {"id": "42", "name": "John"}}}`)
	val, err := DotPath(json, "data.user.id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "42" {
		t.Errorf("val = %q, want %q", val, "42")
	}
}

func TestExtract_ArrayIndex(t *testing.T) {
	json := []byte(`{"data": {"users": [{"name": "Alice"}, {"name": "Bob"}]}}`)
	val, err := DotPath(json, "data.users.0.name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "Alice" {
		t.Errorf("val = %q, want %q", val, "Alice")
	}
}

func TestExtract_NumericValue(t *testing.T) {
	json := []byte(`{"count": 42}`)
	val, err := DotPath(json, "count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "42" {
		t.Errorf("val = %q, want %q", val, "42")
	}
}

func TestExtract_BooleanValue(t *testing.T) {
	json := []byte(`{"active": true}`)
	val, err := DotPath(json, "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "true" {
		t.Errorf("val = %q, want %q", val, "true")
	}
}

func TestExtract_MissingKey(t *testing.T) {
	json := []byte(`{"data": {"id": "42"}}`)
	_, err := DotPath(json, "data.name")
	if err == nil {
		t.Error("expected error for missing key")
	}
}

func TestExtract_InvalidJSON(t *testing.T) {
	json := []byte(`not json`)
	_, err := DotPath(json, "key")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestExtractAll(t *testing.T) {
	json := []byte(`{"access_token": "abc123", "data": {"id": "42"}}`)
	mapping := map[string]string{
		"token":   "access_token",
		"user_id": "data.id",
	}
	result, err := All(json, mapping)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["token"] != "abc123" {
		t.Errorf("token = %q, want %q", result["token"], "abc123")
	}
	if result["user_id"] != "42" {
		t.Errorf("user_id = %q, want %q", result["user_id"], "42")
	}
}
