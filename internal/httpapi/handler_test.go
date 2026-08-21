package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/1tomany/recall/internal/database"
)

func TestProjectAndMemoryAPI(t *testing.T) {
	handler := newTestHandler(t)

	response := request(t, handler, http.MethodGet, "/projects", nil, nil)
	assertStatus(t, response, http.StatusOK)
	assertJSONContentType(t, response)
	if body := readBody(t, response); body != "[]\n" {
		t.Fatalf("GET /projects body = %q, want empty JSON array", body)
	}

	response = request(t, handler, http.MethodPost, "/projects", []byte(`{
		"name":" Recall ",
		"summary":"Agent memory service",
		"path":"/work/recall/../recall"
	}`), nil)
	assertStatus(t, response, http.StatusCreated)
	if got := response.Header.Get("Location"); got != "/projects/1" {
		t.Errorf("project Location = %q, want /projects/1", got)
	}
	var project database.Project
	decodeResponse(t, response, &project)
	if project.Name != "Recall" || project.Path != filepath.Clean("/work/recall/../recall") {
		t.Fatalf("created project = %#v", project)
	}

	response = request(t, handler, http.MethodGet, "/projects/1", nil, nil)
	assertStatus(t, response, http.StatusOK)
	decodeResponse(t, response, &project)
	if project.ID != 1 {
		t.Fatalf("GET /projects/1 = %#v", project)
	}

	response = request(t, handler, http.MethodPost, "/projects/1/memories", []byte(`{
		"name":"Database choice",
		"memory":"Recall uses SQLite.",
		"agent":"Codex",
		"model":"GPT-5"
	}`), map[string]string{"Content-Type": "application/json; charset=utf-8"})
	assertStatus(t, response, http.StatusCreated)
	if got := response.Header.Get("Location"); got != "/projects/1/memories/1" {
		t.Errorf("memory Location = %q, want /projects/1/memories/1", got)
	}
	var memory database.Memory
	decodeResponse(t, response, &memory)
	if memory.ProjectID != 1 || memory.Agent == nil || *memory.Agent != "Codex" {
		t.Fatalf("created memory = %#v", memory)
	}

	response = request(t, handler, http.MethodGet, "/projects/1/memories/1", nil, nil)
	assertStatus(t, response, http.StatusOK)
	decodeResponse(t, response, &memory)
	if memory.Memory != "Recall uses SQLite." {
		t.Fatalf("GET memory = %#v", memory)
	}

	response = request(t, handler, http.MethodGet, "/projects/2/memories/1", nil, nil)
	assertStatus(t, response, http.StatusNotFound)
}

func TestContentNegotiationAndRouting(t *testing.T) {
	handler := newTestHandler(t)
	tests := []struct {
		name    string
		method  string
		path    string
		headers map[string]string
		status  int
		allow   string
	}{
		{name: "unsupported accept", method: http.MethodGet, path: "/projects", headers: map[string]string{"Accept": "text/html"}, status: http.StatusNotAcceptable},
		{name: "disabled json accept", method: http.MethodGet, path: "/projects", headers: map[string]string{"Accept": "application/json;q=0"}, status: http.StatusNotAcceptable},
		{name: "json among alternatives", method: http.MethodGet, path: "/projects", headers: map[string]string{"Accept": "text/html, application/json;q=0.5"}, status: http.StatusOK},
		{name: "unsupported content type", method: http.MethodPost, path: "/projects", headers: map[string]string{"Content-Type": "text/plain"}, status: http.StatusUnsupportedMediaType},
		{name: "unknown route", method: http.MethodGet, path: "/unknown", status: http.StatusNotFound},
		{name: "trailing slash", method: http.MethodGet, path: "/projects/", status: http.StatusNotFound},
		{name: "method not allowed", method: http.MethodDelete, path: "/projects", status: http.StatusMethodNotAllowed, allow: "GET, POST"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, handler, test.method, test.path, nil, test.headers)
			assertStatus(t, response, test.status)
			assertJSONContentType(t, response)
			if got := response.Header.Get("Allow"); got != test.allow {
				t.Errorf("Allow = %q, want %q", got, test.allow)
			}
			_ = response.Body.Close()
		})
	}
}

func TestCreateRequestValidation(t *testing.T) {
	handler := newTestHandler(t)
	tests := []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{name: "malformed JSON", path: "/projects", body: `{`, status: http.StatusBadRequest},
		{name: "unknown field", path: "/projects", body: `{"name":"Recall","path":"/work","extra":true}`, status: http.StatusBadRequest},
		{name: "multiple objects", path: "/projects", body: `{"name":"Recall","path":"/work"} {}`, status: http.StatusBadRequest},
		{name: "empty name", path: "/projects", body: `{"name":" ","path":"/work"}`, status: http.StatusUnprocessableEntity},
		{name: "relative path", path: "/projects", body: `{"name":"Recall","path":"work"}`, status: http.StatusUnprocessableEntity},
		{name: "missing memory text", path: "/projects/1/memories", body: `{"name":"Preference","memory":" "}`, status: http.StatusUnprocessableEntity},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := request(t, handler, http.MethodPost, test.path, []byte(test.body), nil)
			assertStatus(t, response, test.status)
			assertJSONContentType(t, response)
			var body map[string]any
			decodeResponse(t, response, &body)
			if body["error"] == "" {
				t.Errorf("response has no error message: %#v", body)
			}
		})
	}
}

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	db, err := database.Open(context.Background(), database.DBPath(t.TempDir()))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewHandler(database.NewStore(db), slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func request(t *testing.T, handler http.Handler, method, path string, body []byte, headers map[string]string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder.Result()
}

func assertStatus(t *testing.T, response *http.Response, want int) {
	t.Helper()
	if response.StatusCode != want {
		body := readBody(t, response)
		t.Fatalf("status = %d, want %d; body = %s", response.StatusCode, want, body)
	}
}

func assertJSONContentType(t *testing.T, response *http.Response) {
	t.Helper()
	if got := response.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
}

func decodeResponse(t *testing.T, response *http.Response, destination any) {
	t.Helper()
	defer response.Body.Close()
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func readBody(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return string(body)
}
