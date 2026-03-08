package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/denggeng/symphony-go/internal/orchestrator"
)

type Options struct {
	Logger     *slog.Logger
	Host       string
	Port       int
	Controller *orchestrator.Controller
}

type Server struct {
	logger     *slog.Logger
	httpServer *http.Server
	controller *orchestrator.Controller
	addr       string
}

func New(opts Options) *Server {
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	addr := fmt.Sprintf("%s:%d", opts.Host, opts.Port)
	server := &Server{logger: logger, controller: opts.Controller, addr: addr}
	mux := http.NewServeMux()
	mux.HandleFunc("/", server.handleIndex)
	mux.HandleFunc("/healthz", server.handleHealthz)
	mux.HandleFunc("/api/v1/state", server.handleState)
	mux.HandleFunc("/api/v1/refresh", server.handleRefresh)
	mux.HandleFunc("/api/v1/issues/", server.handleIssue)
	mux.HandleFunc("/api/v1/webhooks/jira", server.handleJiraWebhook)
	server.httpServer = &http.Server{Addr: addr, Handler: mux}
	return server
}

func (server *Server) Addr() string { return server.addr }
func (server *Server) Start() error {
	server.logger.Info("http server listening", slog.String("addr", server.addr))
	return server.httpServer.ListenAndServe()
}
func (server *Server) Shutdown(ctx context.Context) error { return server.httpServer.Shutdown(ctx) }

func (server *Server) handleIndex(writer http.ResponseWriter, _ *http.Request) {
	writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = writer.Write([]byte("symphony-go\n\nGET /healthz\nGET /api/v1/state\nPOST /api/v1/refresh\nGET /api/v1/issues/{identifier}\nPOST /api/v1/webhooks/jira\n"))
}

func (server *Server) handleHealthz(writer http.ResponseWriter, _ *http.Request) {
	server.writeJSON(writer, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) handleState(writer http.ResponseWriter, _ *http.Request) {
	if server.controller == nil {
		server.writeError(writer, http.StatusServiceUnavailable, "controller unavailable")
		return
	}
	server.writeJSON(writer, http.StatusOK, server.controller.Snapshot())
}

func (server *Server) handleRefresh(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		server.writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	server.writeJSON(writer, http.StatusOK, server.controller.RequestRefresh())
}

func (server *Server) handleIssue(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		server.writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identifier := strings.TrimPrefix(request.URL.Path, "/api/v1/issues/")
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		server.writeError(writer, http.StatusBadRequest, "missing issue identifier")
		return
	}
	snapshot, ok := server.controller.IssueSnapshot(identifier)
	if !ok {
		server.writeError(writer, http.StatusNotFound, "issue not found in running set")
		return
	}
	server.writeJSON(writer, http.StatusOK, snapshot)
}

func (server *Server) handleJiraWebhook(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		server.writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	secret := request.URL.Query().Get("secret")
	if secret == "" {
		secret = request.Header.Get("X-Symphony-Webhook-Secret")
	}
	response, err := server.controller.HandleJiraWebhook(secret)
	if err != nil {
		server.writeError(writer, http.StatusUnauthorized, err.Error())
		return
	}
	server.writeJSON(writer, http.StatusOK, response)
}

func (server *Server) writeError(writer http.ResponseWriter, status int, message string) {
	server.writeJSON(writer, status, map[string]string{"error": message})
}

func (server *Server) writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		server.logger.Error("encode response", slog.Any("error", err))
	}
}
