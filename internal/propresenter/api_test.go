package propresenter

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTriggerPresentation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/presentations/open" {
			t.Errorf("expected /api/v1/presentations/open, got %s", r.URL.Path)
		}

		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("failed to parse request: %v", err)
		}
		if path, ok := req["path"]; !ok || path != "/path/to/Message.pro" {
			t.Errorf("expected path=/path/to/Message.pro, got %v", path)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	err := client.TriggerPresentation("/path/to/Message.pro")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTriggerPresentationWithPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader != "Bearer secret-password" {
			t.Errorf("expected Bearer token, got %q", authHeader)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "secret-password")
	err := client.TriggerPresentation("/path/to/Message.pro")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTriggerPresentationAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	err := client.TriggerPresentation("/path/to/Message.pro")
	if err == nil {
		t.Fatal("expected error for API failure")
	}
}
