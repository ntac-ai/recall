// Package httpapi implements Recall's JSON REST API.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/1tomany/recall/internal/database"
)

const maxRequestBody = 1 << 20 // 1 MiB

// Store describes the persistence operations needed by the HTTP API.
type Store interface {
	ListProjects(context.Context) ([]database.Project, error)
	GetProject(context.Context, int64) (database.Project, error)
	CreateProject(context.Context, database.NewProject) (database.Project, error)
	CreateMemory(context.Context, int64, database.NewMemory) (database.Memory, error)
	GetMemory(context.Context, int64, int64) (database.Memory, error)
}

// Handler routes and serves Recall API requests.
type Handler struct {
	store  Store
	logger *slog.Logger
}

// NewHandler constructs a Recall API handler.
func NewHandler(store Store, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{store: store, logger: logger}
}

// ServeHTTP enforces JSON content negotiation before routing the request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if !acceptsJSON(r.Header.Values("Accept")) {
		writeError(w, http.StatusNotAcceptable, "the response is only available as application/json")
		return
	}
	if !hasJSONContentType(r.Header.Get("Content-Type")) {
		writeError(w, http.StatusUnsupportedMediaType, "the request Content-Type must be application/json")
		return
	}

	segments := pathSegments(r.URL.Path)
	switch {
	case r.URL.Path == "/projects":
		h.serveProjects(w, r)
	case len(segments) == 2 && segments[0] == "projects":
		h.serveProject(w, r, segments[1])
	case len(segments) == 3 && segments[0] == "projects" && segments[2] == "memories":
		h.serveMemories(w, r, segments[1])
	case len(segments) == 4 && segments[0] == "projects" && segments[2] == "memories":
		h.serveMemory(w, r, segments[1], segments[3])
	default:
		writeError(w, http.StatusNotFound, "resource not found")
	}
}

func (h *Handler) serveProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projects, err := h.store.ListProjects(r.Context())
		if err != nil {
			h.writeInternalError(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, projects)
	case http.MethodPost:
		var request createProjectRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		input, err := request.validate()
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		project, err := h.store.CreateProject(r.Context(), input)
		if err != nil {
			h.writeInternalError(w, r, err)
			return
		}
		w.Header().Set("Location", fmt.Sprintf("/projects/%d", project.ID))
		writeJSON(w, http.StatusCreated, project)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (h *Handler) serveProject(w http.ResponseWriter, r *http.Request, rawProjectID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	projectID, ok := parseID(rawProjectID)
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	project, err := h.store.GetProject(r.Context(), projectID)
	if errors.Is(err, database.ErrNotFound) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		h.writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (h *Handler) serveMemories(w http.ResponseWriter, r *http.Request, rawProjectID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	projectID, ok := parseID(rawProjectID)
	if !ok {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var request createMemoryRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	input, err := request.validate()
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	memory, err := h.store.CreateMemory(r.Context(), projectID, input)
	if errors.Is(err, database.ErrNotFound) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	if err != nil {
		h.writeInternalError(w, r, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/projects/%d/memories/%d", projectID, memory.ID))
	writeJSON(w, http.StatusCreated, memory)
}

func (h *Handler) serveMemory(w http.ResponseWriter, r *http.Request, rawProjectID, rawMemoryID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	projectID, projectOK := parseID(rawProjectID)
	memoryID, memoryOK := parseID(rawMemoryID)
	if !projectOK || !memoryOK {
		writeError(w, http.StatusNotFound, "memory not found")
		return
	}

	memory, err := h.store.GetMemory(r.Context(), projectID, memoryID)
	if errors.Is(err, database.ErrNotFound) {
		writeError(w, http.StatusNotFound, "memory not found")
		return
	}
	if err != nil {
		h.writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, memory)
}

type createProjectRequest struct {
	Name    string  `json:"name"`
	Summary *string `json:"summary"`
	Path    string  `json:"path"`
}

func (r createProjectRequest) validate() (database.NewProject, error) {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return database.NewProject{}, errors.New("name must not be empty")
	}
	if r.Path == "" {
		return database.NewProject{}, errors.New("path must not be empty")
	}
	if !filepath.IsAbs(r.Path) {
		return database.NewProject{}, errors.New("path must be absolute")
	}
	r.Path = filepath.Clean(r.Path)
	return database.NewProject{Name: r.Name, Summary: r.Summary, Path: r.Path}, nil
}

type createMemoryRequest struct {
	Name   string  `json:"name"`
	Memory string  `json:"memory"`
	Agent  *string `json:"agent"`
	Model  *string `json:"model"`
}

func (r createMemoryRequest) validate() (database.NewMemory, error) {
	r.Name = strings.TrimSpace(r.Name)
	if r.Name == "" {
		return database.NewMemory{}, errors.New("name must not be empty")
	}
	if strings.TrimSpace(r.Memory) == "" {
		return database.NewMemory{}, errors.New("memory must not be empty")
	}
	return database.NewMemory{
		Name:   r.Name,
		Memory: r.Memory,
		Agent:  r.Agent,
		Model:  r.Model,
	}, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("invalid JSON body: only one JSON object is allowed")
	}
	return nil
}

func acceptsJSON(values []string) bool {
	if len(values) == 0 {
		return true
	}
	for _, value := range values {
		for _, mediaRange := range strings.Split(value, ",") {
			mediaType, parameters, err := mime.ParseMediaType(strings.TrimSpace(mediaRange))
			if err != nil || (mediaType != "*/*" && mediaType != "application/json") {
				continue
			}
			quality, err := strconv.ParseFloat(parameters["q"], 64)
			if parameters["q"] == "" || (err == nil && quality > 0) {
				return true
			}
		}
	}
	return false
}

func hasJSONContentType(value string) bool {
	if value == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && mediaType == "application/json"
}

func pathSegments(path string) []string {
	if path == "" || path[0] != '/' {
		return nil
	}
	return strings.Split(path[1:], "/")
}

func parseID(value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id > 0
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (h *Handler) writeInternalError(w http.ResponseWriter, r *http.Request, err error) {
	h.logger.ErrorContext(r.Context(), "request failed",
		"method", r.Method,
		"path", r.URL.Path,
		"error", err,
	)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
