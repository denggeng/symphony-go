package codexappserver

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/denggeng/symphony-go/internal/agent"
	"github.com/denggeng/symphony-go/internal/config"
	"github.com/denggeng/symphony-go/internal/domain"
	"github.com/denggeng/symphony-go/internal/tools"
)

const nonInteractiveAnswer = "This is a non-interactive session. Operator input is unavailable."

type Client struct {
	cfg    config.Config
	logger *slog.Logger
	tools  *tools.Executor
}

type session struct {
	cfg                 config.Config
	logger              *slog.Logger
	tools               *tools.Executor
	cmd                 *exec.Cmd
	stdin               io.WriteCloser
	stdoutCh            chan string
	stderrCh            chan string
	procDone            chan error
	workspace           string
	threadID            string
	approvalPolicy      any
	threadSandbox       string
	turnSandboxPolicy   map[string]any
	autoApproveRequests bool
	sendMu              sync.Mutex
}

func New(cfg config.Config, logger *slog.Logger, toolExecutor *tools.Executor) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{cfg: cfg, logger: logger, tools: toolExecutor}
}

func (client *Client) StartSession(ctx context.Context, workspace string) (agent.Session, error) {
	cmd := exec.CommandContext(ctx, "sh", "-lc", client.cfg.Codex.Command)
	cmd.Dir = workspace

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("codex stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("codex stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("codex stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start codex app-server: %w", err)
	}

	session := &session{
		cfg:                 client.cfg,
		logger:              client.logger,
		tools:               client.tools,
		cmd:                 cmd,
		stdin:               stdin,
		stdoutCh:            make(chan string, 256),
		stderrCh:            make(chan string, 256),
		procDone:            make(chan error, 1),
		workspace:           workspace,
		approvalPolicy:      client.cfg.Codex.ApprovalPolicy,
		threadSandbox:       client.cfg.Codex.ThreadSandbox,
		turnSandboxPolicy:   client.cfg.EffectiveTurnSandboxPolicy(workspace),
		autoApproveRequests: approvalPolicyIsNever(client.cfg.Codex.ApprovalPolicy),
	}

	go session.readStream(stdout, session.stdoutCh)
	go session.readStream(stderr, session.stderrCh)
	go func() {
		session.procDone <- cmd.Wait()
	}()

	if err := session.initialize(ctx); err != nil {
		_ = session.stop(context.Background())
		return nil, err
	}

	return session, nil
}

func (client *Client) RunTurn(ctx context.Context, rawSession agent.Session, issue domain.Issue, prompt string, onEvent agent.EventHandler) (agent.TurnResult, error) {
	session, ok := rawSession.(*session)
	if !ok {
		return agent.TurnResult{}, errors.New("invalid codex session")
	}

	turnID, err := session.startTurn(ctx, issue, prompt)
	if err != nil {
		return agent.TurnResult{}, err
	}

	result := agent.TurnResult{
		SessionID: session.threadID + "-" + turnID,
		ThreadID:  session.threadID,
		TurnID:    turnID,
	}

	session.emit(onEvent, "session_started", result.SessionID, turnID, "turn started", nil)
	if err := session.awaitTurnCompletion(ctx, result, onEvent); err != nil {
		return agent.TurnResult{}, err
	}
	return result, nil
}

func (client *Client) StopSession(ctx context.Context, rawSession agent.Session) error {
	session, ok := rawSession.(*session)
	if !ok {
		return nil
	}
	return session.stop(ctx)
}

func (session *session) initialize(ctx context.Context) error {
	if err := session.send(map[string]any{
		"method": "initialize",
		"id":     1,
		"params": map[string]any{
			"capabilities": map[string]any{"experimentalApi": true},
			"clientInfo": map[string]any{
				"name":    "symphony-go",
				"title":   "symphony-go",
				"version": "0.1.0",
			},
		},
	}); err != nil {
		return err
	}
	if _, err := session.awaitResponse(ctx, 1); err != nil {
		return err
	}
	if err := session.send(map[string]any{"method": "initialized", "params": map[string]any{}}); err != nil {
		return err
	}
	if err := session.send(map[string]any{
		"method": "thread/start",
		"id":     2,
		"params": map[string]any{
			"approvalPolicy": session.approvalPolicy,
			"sandbox":        session.threadSandbox,
			"cwd":            session.workspace,
			"dynamicTools":   session.toolSpecs(),
		},
	}); err != nil {
		return err
	}
	response, err := session.awaitResponse(ctx, 2)
	if err != nil {
		return err
	}
	thread, _ := response["thread"].(map[string]any)
	threadID, _ := thread["id"].(string)
	if threadID == "" {
		return errors.New("codex app-server returned an empty thread id")
	}
	session.threadID = threadID
	return nil
}

func (session *session) startTurn(ctx context.Context, issue domain.Issue, prompt string) (string, error) {
	if err := session.send(map[string]any{
		"method": "turn/start",
		"id":     3,
		"params": map[string]any{
			"threadId": session.threadID,
			"input": []map[string]any{{
				"type": "text",
				"text": prompt,
			}},
			"cwd":            session.workspace,
			"title":          fmt.Sprintf("%s: %s", issue.Identifier, issue.Title),
			"approvalPolicy": session.approvalPolicy,
			"sandboxPolicy":  session.turnSandboxPolicy,
		},
	}); err != nil {
		return "", err
	}
	response, err := session.awaitResponse(ctx, 3)
	if err != nil {
		return "", err
	}
	turn, _ := response["turn"].(map[string]any)
	turnID, _ := turn["id"].(string)
	if turnID == "" {
		return "", errors.New("codex app-server returned an empty turn id")
	}
	return turnID, nil
}

func (session *session) awaitResponse(ctx context.Context, requestID int) (map[string]any, error) {
	timer := time.NewTimer(time.Duration(session.cfg.Codex.ReadTimeoutMs) * time.Millisecond)
	defer timer.Stop()
	stdoutCh := session.stdoutCh
	stderrCh := session.stderrCh

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err := <-session.procDone:
			if err == nil {
				return nil, io.EOF
			}
			return nil, err
		case line, ok := <-stderrCh:
			if !ok {
				stderrCh = nil
				continue
			}
			session.logger.Debug("codex stderr", slog.String("line", line))
		case <-timer.C:
			return nil, errors.New("timed out waiting for codex response")
		case line, ok := <-stdoutCh:
			if !ok {
				return nil, io.EOF
			}
			payload, err := decodeJSONLine(line)
			if err != nil {
				session.logger.Debug("codex non-json line", slog.String("line", line))
				continue
			}
			if rawID, ok := payload["id"]; ok && intFromAny(rawID) == requestID {
				if rawErr, exists := payload["error"]; exists {
					return nil, fmt.Errorf("codex response error: %v", rawErr)
				}
				result, _ := payload["result"].(map[string]any)
				return result, nil
			}
		}
	}
}

func (session *session) awaitTurnCompletion(ctx context.Context, result agent.TurnResult, onEvent agent.EventHandler) error {
	timer := time.NewTimer(time.Duration(session.cfg.Codex.TurnTimeoutMs) * time.Millisecond)
	defer timer.Stop()
	stdoutCh := session.stdoutCh
	stderrCh := session.stderrCh

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-session.procDone:
			if err == nil {
				return nil
			}
			return fmt.Errorf("codex app-server exited: %w", err)
		case line, ok := <-stderrCh:
			if !ok {
				stderrCh = nil
				continue
			}
			session.logger.Debug("codex stderr", slog.String("line", line))
		case <-timer.C:
			return errors.New("codex turn timeout")
		case line, ok := <-stdoutCh:
			if !ok {
				return io.EOF
			}
			payload, err := decodeJSONLine(line)
			if err != nil {
				session.emit(onEvent, "malformed", result.SessionID, result.TurnID, line, nil)
				continue
			}
			method, _ := payload["method"].(string)
			switch method {
			case "turn/completed":
				session.emit(onEvent, "turn_completed", result.SessionID, result.TurnID, "turn completed", payload)
				return nil
			case "turn/failed":
				session.emit(onEvent, "turn_failed", result.SessionID, result.TurnID, "turn failed", payload)
				return fmt.Errorf("codex turn failed: %v", payload["params"])
			case "turn/cancelled":
				session.emit(onEvent, "turn_cancelled", result.SessionID, result.TurnID, "turn cancelled", payload)
				return fmt.Errorf("codex turn cancelled: %v", payload["params"])
			case "item/tool/call":
				if err := session.handleToolCall(ctx, payload, result, onEvent); err != nil {
					return err
				}
			case "item/tool/requestUserInput":
				if err := session.handleToolInputRequest(payload, result, onEvent); err != nil {
					return err
				}
			case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
				if err := session.handleApproval(payload, result, onEvent, "acceptForSession"); err != nil {
					return err
				}
			case "execCommandApproval", "applyPatchApproval":
				if err := session.handleApproval(payload, result, onEvent, "approved_for_session"); err != nil {
					return err
				}
			default:
				if method != "" && needsInput(method, payload) {
					session.emit(onEvent, "turn_input_required", result.SessionID, result.TurnID, method, payload)
					return fmt.Errorf("codex turn requires operator input: %s", method)
				}
				session.emit(onEvent, "notification", result.SessionID, result.TurnID, method, payload)
			}
		}
	}
}

func (session *session) handleToolCall(ctx context.Context, payload map[string]any, result agent.TurnResult, onEvent agent.EventHandler) error {
	id := intFromAny(payload["id"])
	params, _ := payload["params"].(map[string]any)
	toolName := toolCallName(params)
	arguments := params["arguments"]
	toolResult := map[string]any{"success": false, "contentItems": []map[string]any{{"type": "inputText", "text": "No tool executor configured."}}}
	if session.tools != nil {
		toolResult = session.tools.Execute(ctx, toolName, arguments)
	}
	if err := session.send(map[string]any{"id": id, "result": toolResult}); err != nil {
		return err
	}
	eventType := "tool_call_failed"
	if success, _ := toolResult["success"].(bool); success {
		eventType = "tool_call_completed"
	}
	session.emit(onEvent, eventType, result.SessionID, result.TurnID, toolName, payload)
	return nil
}

func (session *session) handleToolInputRequest(payload map[string]any, result agent.TurnResult, onEvent agent.EventHandler) error {
	id := intFromAny(payload["id"])
	params, _ := payload["params"].(map[string]any)
	answers, ok := unavailableAnswers(params)
	if !ok {
		return errors.New("codex requested interactive input without recognizable questions")
	}
	if err := session.send(map[string]any{"id": id, "result": map[string]any{"answers": answers}}); err != nil {
		return err
	}
	session.emit(onEvent, "tool_input_auto_answered", result.SessionID, result.TurnID, nonInteractiveAnswer, payload)
	return nil
}

func (session *session) handleApproval(payload map[string]any, result agent.TurnResult, onEvent agent.EventHandler, decision string) error {
	if !session.autoApproveRequests {
		session.emit(onEvent, "approval_required", result.SessionID, result.TurnID, decision, payload)
		return errors.New("codex requested approval but approval_policy is not never")
	}
	id := intFromAny(payload["id"])
	if err := session.send(map[string]any{"id": id, "result": map[string]any{"decision": decision}}); err != nil {
		return err
	}
	session.emit(onEvent, "approval_auto_approved", result.SessionID, result.TurnID, decision, payload)
	return nil
}

func (session *session) stop(ctx context.Context) error {
	_ = session.stdin.Close()
	if session.cmd.Process == nil {
		return nil
	}
	if err := session.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		return nil
	case <-session.procDone:
		return nil
	}
}

func (session *session) readStream(reader io.Reader, output chan<- string) {
	defer close(output)
	buffered := bufio.NewReaderSize(reader, 1<<20)
	for {
		line, err := buffered.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed != "" {
			output <- trimmed
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				session.logger.Debug("codex stream ended", slog.Any("error", err))
			}
			return
		}
	}
}

func (session *session) send(message map[string]any) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("encode codex message: %w", err)
	}
	session.sendMu.Lock()
	defer session.sendMu.Unlock()
	_, err = io.WriteString(session.stdin, string(payload)+"\n")
	if err != nil {
		return fmt.Errorf("write codex message: %w", err)
	}
	return nil
}

func (session *session) toolSpecs() []map[string]any {
	if session.tools == nil {
		return nil
	}
	return session.tools.ToolSpecs()
}

func (session *session) emit(onEvent agent.EventHandler, eventType string, sessionID string, turnID string, message string, raw map[string]any) {
	if onEvent == nil {
		return
	}
	pid := ""
	if session.cmd != nil && session.cmd.Process != nil {
		pid = fmt.Sprintf("%d", session.cmd.Process.Pid)
	}
	onEvent(agent.Event{
		Timestamp:         time.Now().UTC(),
		Type:              eventType,
		SessionID:         sessionID,
		ThreadID:          session.threadID,
		TurnID:            turnID,
		CodexAppServerPID: pid,
		Message:           message,
		Usage:             usageFromPayload(raw),
		Raw:               raw,
	})
}

func decodeJSONLine(line string) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func toolCallName(params map[string]any) string {
	for _, key := range []string{"tool", "name"} {
		if text, ok := params[key].(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func unavailableAnswers(params map[string]any) (map[string]any, bool) {
	questions, _ := params["questions"].([]any)
	answers := map[string]any{}
	for _, rawQuestion := range questions {
		question, ok := rawQuestion.(map[string]any)
		if !ok {
			return nil, false
		}
		id, _ := question["id"].(string)
		if id == "" {
			return nil, false
		}
		answers[id] = map[string]any{"answers": []string{nonInteractiveAnswer}}
	}
	return answers, len(answers) > 0
}

func approvalPolicyIsNever(policy any) bool {
	text, ok := policy.(string)
	return ok && strings.EqualFold(strings.TrimSpace(text), "never")
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func needsInput(method string, payload map[string]any) bool {
	if strings.HasPrefix(method, "turn/") {
		switch method {
		case "turn/input_required", "turn/needs_input", "turn/need_input", "turn/request_input", "turn/request_response", "turn/provide_input", "turn/approval_required":
			return true
		}
	}
	for _, value := range []any{payload, payload["params"]} {
		if data, ok := value.(map[string]any); ok {
			if data["requiresInput"] == true || data["needsInput"] == true || data["input_required"] == true || data["inputRequired"] == true {
				return true
			}
			if text, _ := data["type"].(string); text == "input_required" || text == "needs_input" {
				return true
			}
		}
	}
	return false
}

func usageFromPayload(payload map[string]any) agent.Usage {
	if payload == nil {
		return agent.Usage{}
	}
	for _, candidate := range []any{payload["usage"], payload["params"]} {
		usageMap, ok := candidate.(map[string]any)
		if !ok {
			continue
		}
		if usageField, ok := usageMap["usage"].(map[string]any); ok {
			usageMap = usageField
		}
		usage := agent.Usage{
			InputTokens:  firstInt(usageMap, "input_tokens", "inputTokens"),
			OutputTokens: firstInt(usageMap, "output_tokens", "outputTokens"),
			TotalTokens:  firstInt(usageMap, "total_tokens", "totalTokens"),
		}
		if usage.TotalTokens != 0 || usage.InputTokens != 0 || usage.OutputTokens != 0 {
			return usage
		}
	}
	return agent.Usage{}
}

func firstInt(values map[string]any, keys ...string) int {
	for _, key := range keys {
		if value := intFromAny(values[key]); value != 0 {
			return value
		}
	}
	return 0
}
