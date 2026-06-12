package ml

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"
	"time"
)

// ── client tests ──────────────────────────────────────────────────────────────

func TestNewClient_DefaultURL(t *testing.T) {
	c := NewClient("")
	if c.BaseURL != DefaultMLServerURL {
		t.Errorf("expected %s, got %s", DefaultMLServerURL, c.BaseURL)
	}
}

func TestNewClient_CustomURL(t *testing.T) {
	c := NewClient("http://localhost:9000")
	if c.BaseURL != "http://localhost:9000" {
		t.Errorf("expected http://localhost:9000, got %s", c.BaseURL)
	}
}

func TestHealth_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.Health(); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestHealth_ServerDown(t *testing.T) {
	c := NewClient("http://localhost:19999")
	c.HTTPClient.Timeout = 1 * time.Second
	if err := c.Health(); err == nil {
		t.Error("expected error when server is down")
	}
}

func TestHealth_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.Health(); err == nil {
		t.Error("expected error on non-200 response")
	}
}

func TestClassify_ReturnsMapping(t *testing.T) {
	expected := map[string]string{
		"/tmp/dog.jpg":  "animals",
		"/tmp/food.jpg": "food",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/classify" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.Classify([]string{"/tmp/dog.jpg", "/tmp/food.jpg"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for path, tag := range expected {
		if result[path] != tag {
			t.Errorf("path %s: expected %s, got %s", path, tag, result[path])
		}
	}
}

func TestClassify_PythonErrorReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"error": "model not loaded"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Classify([]string{"/tmp/img.jpg"})
	if err == nil {
		t.Error("expected error when Python returns error key")
	}
}

func TestClassify_EmptyPaths(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.Classify([]string{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

func TestClassify_ServerDown(t *testing.T) {
	c := NewClient("http://localhost:19999")
	c.HTTPClient.Timeout = 1 * time.Second
	_, err := c.Classify([]string{"/tmp/img.jpg"})
	if err == nil {
		t.Error("expected error when server is down")
	}
}

func TestClassify_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json {{{"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.Classify([]string{"/tmp/img.jpg"})
	if err == nil {
		t.Error("expected error on invalid JSON response")
	}
}

func TestClassify_NotImageTag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"/tmp/doc.pdf": "__not_image__",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	result, err := c.Classify([]string{"/tmp/doc.pdf"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result["/tmp/doc.pdf"] != "__not_image__" {
		t.Errorf("expected __not_image__, got %s", result["/tmp/doc.pdf"])
	}
}

// ── server tests ──────────────────────────────────────────────────────────────

func TestNewServer(t *testing.T) {
	s := NewServer("")
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.cmd != nil {
		t.Error("cmd should be nil before Start()")
	}
}

func TestStop_BeforeStart(t *testing.T) {
	s := NewServer("")
	// stop before Start should not panic or error
	if err := s.Stop(); err != nil {
		t.Errorf("expected no error on Stop before Start, got %v", err)
	}
}

func TestStop_NilProcess(t *testing.T) {
	s := NewServer("")
	s.cmd = &exec.Cmd{} // cmd set but Process is nil
	if err := s.Stop(); err != nil {
		t.Errorf("expected no error when Process is nil, got %v", err)
	}
}

func TestStart_ClassifierNotFound(t *testing.T) {
	s := NewServer("")
	// the real fiona-classifier won't be next to the test binary
	// so Start() should fail trying to exec a nonexistent file
	err := s.Start()
	if err == nil {
		// if somehow it started, clean up
		s.Stop()
		t.Error("expected error when fiona-classifier is not found")
	}
}

func TestStop_KillsProcess(t *testing.T) {
	// start a real long-running process to verify Kill works
	s := NewServer("")
	s.cmd = exec.Command("sleep", "60")
	if err := s.cmd.Start(); err != nil {
		t.Skipf("cannot start sleep process: %v", err)
	}

	if err := s.Stop(); err != nil {
		t.Errorf("expected no error on Stop, got %v", err)
	}

	// verify process is dead
	if s.cmd.ProcessState != nil && !s.cmd.ProcessState.Exited() {
		t.Error("process should have exited after Stop")
	}
}
