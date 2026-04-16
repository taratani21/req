package runner

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRun_GET(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/users/42" {
			t.Errorf("path = %q, want /users/42", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"id": "42", "name": "John"}`))
	}))
	defer server.Close()

	resp, err := Run(&Request{
		Method:  "GET",
		URL:     server.URL + "/users/42",
		Headers: map[string]string{"Accept": "application/json"},
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"id": "42", "name": "John"}` {
		t.Errorf("body = %q", string(body))
	}
}

func TestRun_POST(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) == "" {
			t.Error("expected non-empty body")
		}
		w.WriteHeader(201)
		w.Write([]byte(`{"id": "99"}`))
	}))
	defer server.Close()

	resp, err := Run(&Request{
		Method:  "POST",
		URL:     server.URL + "/users",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    `{"name": "Jane"}`,
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Errorf("status = %d, want 201", resp.StatusCode)
	}
}

func TestRun_QueryParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("verbose") != "true" {
			t.Errorf("query verbose = %q, want true", r.URL.Query().Get("verbose"))
		}
		if r.URL.Query().Get("page") != "2" {
			t.Errorf("query page = %q, want 2", r.URL.Query().Get("page"))
		}
		w.WriteHeader(200)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	resp, err := Run(&Request{
		Method:  "GET",
		URL:     server.URL + "/items",
		Query:   map[string]string{"verbose": "true", "page": "2"},
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

func TestRun_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(200)
	}))
	defer server.Close()

	_, err := Run(&Request{
		Method:  "GET",
		URL:     server.URL + "/slow",
		Timeout: 100 * time.Millisecond,
	})
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestRun_4xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error": "not found"}`))
	}))
	defer server.Close()

	resp, err := Run(&Request{
		Method:  "GET",
		URL:     server.URL + "/missing",
		Timeout: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v (4xx should not be a connection error)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
