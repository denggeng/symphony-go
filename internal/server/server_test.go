package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/orchestrator"
	"github.com/denggeng/symphony-go/internal/workflow"
)

func newTestServer() *Server {
	return newTestServerWithOptions(Options{Host: "127.0.0.1", Port: 8080})
}

func newTestServerWithOptions(options Options) *Server {
	controller := orchestrator.New(orchestrator.Options{
		Workflow: workflow.Definition{Path: "/tmp/WORKFLOW.md"},
		Config:   config.Config{Orchestrator: config.OrchestratorConfig{PollIntervalMs: 30000}},
	})
	options.Controller = controller
	if options.Host == "" {
		options.Host = "127.0.0.1"
	}
	if options.Port == 0 {
		options.Port = 8080
	}
	return New(options)
}

func TestDashboardPageRequiresAuthWhenConfigured(t *testing.T) {
	t.Parallel()
	server := newTestServerWithOptions(Options{Host: "127.0.0.1", Port: 8080, AuthUsername: "admin", AuthPassword: "secret"})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status code: %d", response.Code)
	}
	if authHeader := response.Header().Get("WWW-Authenticate"); !strings.Contains(authHeader, "Basic") {
		t.Fatalf("expected auth challenge, got %q", authHeader)
	}
}

func TestDashboardPageAcceptsValidBasicAuth(t *testing.T) {
	t.Parallel()
	server := newTestServerWithOptions(Options{Host: "127.0.0.1", Port: 8080, AuthUsername: "admin", AuthPassword: "secret"})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.SetBasicAuth("admin", "secret")
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.Code)
	}
}

func TestHealthzBypassesAuth(t *testing.T) {
	t.Parallel()
	server := newTestServerWithOptions(Options{Host: "127.0.0.1", Port: 8080, AuthUsername: "admin", AuthPassword: "secret"})
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.Code)
	}
}

func TestDashboardPageRendersHTML(t *testing.T) {
	t.Parallel()
	server := newTestServer()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	if !strings.Contains(response.Body.String(), "symphony-go dashboard") {
		t.Fatalf("expected dashboard markup")
	}
	if !strings.Contains(response.Body.String(), "Pending Backlog") {
		t.Fatalf("expected backlog markup")
	}
	if !strings.Contains(response.Body.String(), "language-toggle") {
		t.Fatalf("expected language toggle markup")
	}
}

func TestHistoryPageRendersHTML(t *testing.T) {
	t.Parallel()
	server := newTestServer()
	request := httptest.NewRequest(http.MethodGet, "/history", nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "run history") {
		t.Fatalf("expected history markup")
	}
	if !strings.Contains(response.Body.String(), "language-toggle") {
		t.Fatalf("expected language toggle markup")
	}
}

func TestRunHistoryPageRendersHTML(t *testing.T) {
	t.Parallel()
	server := newTestServer()
	request := httptest.NewRequest(http.MethodGet, "/history/run-123", nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, `data-page-kind="run"`) {
		t.Fatalf("expected run page kind")
	}
	if !strings.Contains(body, `data-run-id="run-123"`) {
		t.Fatalf("expected run id in html")
	}
	if !strings.Contains(body, "language-toggle") {
		t.Fatalf("expected language toggle markup")
	}
}

func TestIssuePageRendersHTML(t *testing.T) {
	t.Parallel()
	server := newTestServer()
	request := httptest.NewRequest(http.MethodGet, "/issues/ABC-123", nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.Code)
	}
	body := response.Body.String()
	if !strings.Contains(body, `data-page-kind="issue"`) {
		t.Fatalf("expected issue page kind, got %q", body)
	}
	if !strings.Contains(body, `data-issue-identifier="ABC-123"`) {
		t.Fatalf("expected issue identifier in html")
	}
	if !strings.Contains(body, "Queue Detail") {
		t.Fatalf("expected issue detail markup")
	}
	if !strings.Contains(body, "Recent Runs for Issue") {
		t.Fatalf("expected issue history markup")
	}
}

func TestUnknownPathReturnsNotFound(t *testing.T) {
	t.Parallel()
	server := newTestServer()
	request := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unexpected status code: %d", response.Code)
	}
}

func TestEventsEndpointStreamsSnapshot(t *testing.T) {
	t.Parallel()
	server := newTestServer()
	httpServer := httptest.NewServer(server.httpServer.Handler)
	defer httpServer.Close()

	response, err := httpServer.Client().Get(httpServer.URL + "/events")
	if err != nil {
		t.Fatalf("stream request failed: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.StatusCode)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.Contains(contentType, "text/event-stream") {
		t.Fatalf("unexpected content type: %q", contentType)
	}

	line, err := bufio.NewReader(response.Body).ReadString('\n')
	if err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if !strings.HasPrefix(line, "data: ") {
		t.Fatalf("unexpected stream line: %q", line)
	}

	var snapshot orchestrator.Snapshot
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
	if err := json.Unmarshal([]byte(payload), &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if snapshot.Service.Name != "symphony-go" {
		t.Fatalf("unexpected service name: %q", snapshot.Service.Name)
	}
}

func TestHistoryEndpointReturnsJSON(t *testing.T) {
	t.Parallel()
	server := newTestServer()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/history", nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("unexpected content type: %q", contentType)
	}
	if !strings.Contains(response.Body.String(), `"runs": []`) {
		t.Fatalf("expected empty history payload")
	}
}

func TestRunHistoryEndpointMissingRunReturnsNotFound(t *testing.T) {
	t.Parallel()
	server := newTestServer()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/history/missing-run", nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unexpected status code: %d", response.Code)
	}
}

func TestStateEndpointStillReturnsJSON(t *testing.T) {
	t.Parallel()
	server := newTestServer()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/state", nil)
	response := httptest.NewRecorder()
	server.httpServer.Handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status code: %d", response.Code)
	}
	if contentType := response.Header().Get("Content-Type"); !strings.Contains(contentType, "application/json") {
		t.Fatalf("unexpected content type: %q", contentType)
	}
}
