package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/denggeng/symphony-go/internal/workflow"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Tracker      TrackerConfig      `yaml:"tracker" json:"tracker"`
	Local        LocalConfig        `yaml:"local" json:"local,omitempty"`
	Orchestrator OrchestratorConfig `yaml:"orchestrator" json:"orchestrator"`
	Workspace    WorkspaceConfig    `yaml:"workspace" json:"workspace"`
	Hooks        HooksConfig        `yaml:"hooks" json:"hooks"`
	Agent        AgentConfig        `yaml:"agent" json:"agent"`
	Codex        CodexConfig        `yaml:"codex" json:"codex"`
	Server       ServerConfig       `yaml:"server" json:"server"`
}

type TrackerConfig struct {
	Kind           string   `yaml:"kind" json:"kind"`
	BaseURL        string   `yaml:"base_url" json:"base_url,omitempty"`
	AuthMode       string   `yaml:"auth_mode" json:"auth_mode,omitempty"`
	Email          string   `yaml:"email" json:"-"`
	APIToken       string   `yaml:"api_token" json:"-"`
	ProjectKey     string   `yaml:"project_key" json:"project_key,omitempty"`
	JQL            string   `yaml:"jql" json:"jql,omitempty"`
	ActiveStates   []string `yaml:"active_states" json:"active_states,omitempty"`
	TerminalStates []string `yaml:"terminal_states" json:"terminal_states,omitempty"`
	WebhookSecret  string   `yaml:"webhook_secret" json:"-"`
}

// LocalConfig controls local Markdown task discovery and result storage.
type LocalConfig struct {
	InboxDir       string   `yaml:"inbox_dir" json:"inbox_dir,omitempty"`
	StateDir       string   `yaml:"state_dir" json:"state_dir,omitempty"`
	ArchiveDir     string   `yaml:"archive_dir" json:"archive_dir,omitempty"`
	ResultsDir     string   `yaml:"results_dir" json:"results_dir,omitempty"`
	ActiveStates   []string `yaml:"active_states" json:"active_states,omitempty"`
	TerminalStates []string `yaml:"terminal_states" json:"terminal_states,omitempty"`
}

type OrchestratorConfig struct {
	PollIntervalMs      int `yaml:"poll_interval_ms" json:"poll_interval_ms"`
	MaxConcurrentAgents int `yaml:"max_concurrent_agents" json:"max_concurrent_agents"`
	MaxRetryBackoffMs   int `yaml:"max_retry_backoff_ms" json:"max_retry_backoff_ms"`
}

type WorkspaceConfig struct {
	Root     string                  `yaml:"root" json:"root"`
	Seed     WorkspaceSeedConfig     `yaml:"seed" json:"seed,omitempty"`
	SyncBack WorkspaceSyncBackConfig `yaml:"sync_back" json:"sync_back,omitempty"`
}

// WorkspaceSeedConfig overlays a baseline directory into newly created workspaces.
type WorkspaceSeedConfig struct {
	Path     string   `yaml:"path" json:"path,omitempty"`
	Excludes []string `yaml:"excludes" json:"excludes,omitempty"`
}

// WorkspaceSyncBackConfig copies workspace files back to a baseline directory on selected terminal states.
type WorkspaceSyncBackConfig struct {
	Path     string   `yaml:"path" json:"path,omitempty"`
	OnStates []string `yaml:"on_states" json:"on_states,omitempty"`
	Excludes []string `yaml:"excludes" json:"excludes,omitempty"`
}

type HooksConfig struct {
	AfterCreate  string `yaml:"after_create" json:"after_create,omitempty"`
	BeforeRun    string `yaml:"before_run" json:"before_run,omitempty"`
	AfterRun     string `yaml:"after_run" json:"after_run,omitempty"`
	BeforeRemove string `yaml:"before_remove" json:"before_remove,omitempty"`
	TimeoutMs    int    `yaml:"timeout_ms" json:"timeout_ms"`
}

type AgentConfig struct {
	MaxTurns int `yaml:"max_turns" json:"max_turns"`
}

type CodexConfig struct {
	Command           string         `yaml:"command" json:"command"`
	ApprovalPolicy    any            `yaml:"approval_policy" json:"approval_policy,omitempty"`
	ThreadSandbox     string         `yaml:"thread_sandbox" json:"thread_sandbox,omitempty"`
	TurnSandboxPolicy map[string]any `yaml:"turn_sandbox_policy" json:"turn_sandbox_policy,omitempty"`
	ReadTimeoutMs     int            `yaml:"read_timeout_ms" json:"read_timeout_ms"`
	TurnTimeoutMs     int            `yaml:"turn_timeout_ms" json:"turn_timeout_ms"`
	StallTimeoutMs    int            `yaml:"stall_timeout_ms" json:"stall_timeout_ms"`
}

type ServerConfig struct {
	Host     string `yaml:"host" json:"host"`
	Port     int    `yaml:"port" json:"port"`
	Username string `yaml:"username" json:"-"`
	Password string `yaml:"password" json:"-"`
}

type Summary struct {
	Tracker      TrackerSummary      `json:"tracker"`
	Local        *LocalSummary       `json:"local,omitempty"`
	Orchestrator OrchestratorSummary `json:"orchestrator"`
	Workspace    WorkspaceSummary    `json:"workspace"`
	Hooks        HooksSummary        `json:"hooks"`
	Agent        AgentSummary        `json:"agent"`
	Codex        CodexSummary        `json:"codex"`
	Server       ServerSummary       `json:"server"`
}

type TrackerSummary struct {
	Kind           string   `json:"kind,omitempty"`
	BaseURL        string   `json:"base_url,omitempty"`
	AuthMode       string   `json:"auth_mode,omitempty"`
	ProjectKey     string   `json:"project_key,omitempty"`
	JQL            string   `json:"jql,omitempty"`
	ActiveStates   []string `json:"active_states,omitempty"`
	TerminalStates []string `json:"terminal_states,omitempty"`
	HasCredentials bool     `json:"has_credentials"`
	HasWebhookAuth bool     `json:"has_webhook_auth"`
}

// LocalSummary describes the local task directories exposed in runtime state.
type LocalSummary struct {
	InboxDir   string `json:"inbox_dir,omitempty"`
	StateDir   string `json:"state_dir,omitempty"`
	ArchiveDir string `json:"archive_dir,omitempty"`
	ResultsDir string `json:"results_dir,omitempty"`
}

type OrchestratorSummary struct {
	PollIntervalMs      int `json:"poll_interval_ms"`
	MaxConcurrentAgents int `json:"max_concurrent_agents"`
	MaxRetryBackoffMs   int `json:"max_retry_backoff_ms"`
}

type WorkspaceSummary struct {
	Root     string                    `json:"root"`
	Seed     *WorkspaceSeedSummary     `json:"seed,omitempty"`
	SyncBack *WorkspaceSyncBackSummary `json:"sync_back,omitempty"`
}

// WorkspaceSeedSummary describes the configured workspace baseline overlay.
type WorkspaceSeedSummary struct {
	Path     string   `json:"path,omitempty"`
	Excludes []string `json:"excludes,omitempty"`
}

// WorkspaceSyncBackSummary describes the configured workspace sync-back target.
type WorkspaceSyncBackSummary struct {
	Path     string   `json:"path,omitempty"`
	OnStates []string `json:"on_states,omitempty"`
	Excludes []string `json:"excludes,omitempty"`
}

type HooksSummary struct {
	AfterCreate  bool `json:"after_create"`
	BeforeRun    bool `json:"before_run"`
	AfterRun     bool `json:"after_run"`
	BeforeRemove bool `json:"before_remove"`
	TimeoutMs    int  `json:"timeout_ms"`
}

type AgentSummary struct {
	MaxTurns int `json:"max_turns"`
}

type CodexSummary struct {
	Command        string `json:"command"`
	ApprovalPolicy any    `json:"approval_policy,omitempty"`
	ThreadSandbox  string `json:"thread_sandbox,omitempty"`
	HasTurnSandbox bool   `json:"has_turn_sandbox"`
	ReadTimeoutMs  int    `json:"read_timeout_ms"`
	TurnTimeoutMs  int    `json:"turn_timeout_ms"`
	StallTimeoutMs int    `json:"stall_timeout_ms"`
}

type ServerSummary struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	AuthEnabled bool   `json:"auth_enabled"`
}

func FromWorkflow(definition workflow.Definition) (Config, error) {
	var cfg Config

	if len(definition.Config) > 0 {
		payload, err := yaml.Marshal(definition.Config)
		if err != nil {
			return Config{}, fmt.Errorf("encode workflow config: %w", err)
		}

		if err := yaml.Unmarshal(payload, &cfg); err != nil {
			return Config{}, fmt.Errorf("decode workflow config: %w", err)
		}
	}

	applyDefaults(&cfg)
	expandEnvironment(&cfg)

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (cfg Config) Summary() Summary {
	baseURL := cfg.Tracker.BaseURL
	authMode := cfg.Tracker.AuthMode
	projectKey := cfg.Tracker.ProjectKey
	jql := cfg.Tracker.JQL
	hasCredentials := cfg.Tracker.APIToken != ""
	hasWebhookAuth := cfg.Tracker.WebhookSecret != ""
	var localSummary *LocalSummary
	if cfg.Tracker.Kind == "local" {
		baseURL = ""
		authMode = ""
		projectKey = ""
		jql = ""
		hasCredentials = false
		hasWebhookAuth = false
		localSummary = &LocalSummary{
			InboxDir:   cfg.Local.InboxDir,
			StateDir:   cfg.Local.StateDir,
			ArchiveDir: cfg.Local.ArchiveDir,
			ResultsDir: cfg.Local.ResultsDir,
		}
	}
	var seedSummary *WorkspaceSeedSummary
	if strings.TrimSpace(cfg.Workspace.Seed.Path) != "" {
		seedSummary = &WorkspaceSeedSummary{Path: cfg.Workspace.Seed.Path, Excludes: append([]string(nil), cfg.Workspace.Seed.Excludes...)}
	}
	var syncBackSummary *WorkspaceSyncBackSummary
	if strings.TrimSpace(cfg.Workspace.SyncBack.Path) != "" {
		syncBackSummary = &WorkspaceSyncBackSummary{
			Path:     cfg.Workspace.SyncBack.Path,
			OnStates: append([]string(nil), cfg.Workspace.SyncBack.OnStates...),
			Excludes: append([]string(nil), cfg.Workspace.SyncBack.Excludes...),
		}
	}
	return Summary{
		Tracker: TrackerSummary{
			Kind:           cfg.Tracker.Kind,
			BaseURL:        baseURL,
			AuthMode:       authMode,
			ProjectKey:     projectKey,
			JQL:            jql,
			ActiveStates:   append([]string(nil), cfg.ActiveStates()...),
			TerminalStates: append([]string(nil), cfg.TerminalStates()...),
			HasCredentials: hasCredentials,
			HasWebhookAuth: hasWebhookAuth,
		},
		Local: localSummary,
		Orchestrator: OrchestratorSummary{
			PollIntervalMs:      cfg.Orchestrator.PollIntervalMs,
			MaxConcurrentAgents: cfg.Orchestrator.MaxConcurrentAgents,
			MaxRetryBackoffMs:   cfg.Orchestrator.MaxRetryBackoffMs,
		},
		Workspace: WorkspaceSummary{Root: cfg.Workspace.Root, Seed: seedSummary, SyncBack: syncBackSummary},
		Hooks: HooksSummary{
			AfterCreate:  strings.TrimSpace(cfg.Hooks.AfterCreate) != "",
			BeforeRun:    strings.TrimSpace(cfg.Hooks.BeforeRun) != "",
			AfterRun:     strings.TrimSpace(cfg.Hooks.AfterRun) != "",
			BeforeRemove: strings.TrimSpace(cfg.Hooks.BeforeRemove) != "",
			TimeoutMs:    cfg.Hooks.TimeoutMs,
		},
		Agent: AgentSummary{MaxTurns: cfg.Agent.MaxTurns},
		Codex: CodexSummary{
			Command:        cfg.Codex.Command,
			ApprovalPolicy: sanitizedApprovalPolicy(cfg.Codex.ApprovalPolicy),
			ThreadSandbox:  cfg.Codex.ThreadSandbox,
			HasTurnSandbox: len(cfg.Codex.TurnSandboxPolicy) > 0,
			ReadTimeoutMs:  cfg.Codex.ReadTimeoutMs,
			TurnTimeoutMs:  cfg.Codex.TurnTimeoutMs,
			StallTimeoutMs: cfg.Codex.StallTimeoutMs,
		},
		Server: ServerSummary{Host: cfg.Server.Host, Port: cfg.Server.Port, AuthEnabled: cfg.Server.Username != "" && cfg.Server.Password != ""},
	}
}

// ActiveStates returns the active state list for the configured tracker kind.
func (cfg Config) ActiveStates() []string {
	if cfg.Tracker.Kind == "local" {
		return append([]string(nil), cfg.Local.ActiveStates...)
	}
	return append([]string(nil), cfg.Tracker.ActiveStates...)
}

// TerminalStates returns the terminal state list for the configured tracker kind.
func (cfg Config) TerminalStates() []string {
	if cfg.Tracker.Kind == "local" {
		return append([]string(nil), cfg.Local.TerminalStates...)
	}
	return append([]string(nil), cfg.Tracker.TerminalStates...)
}

func (cfg Config) ActiveStateSet() map[string]struct{} {
	return toStateSet(cfg.ActiveStates())
}

func (cfg Config) TerminalStateSet() map[string]struct{} {
	return toStateSet(cfg.TerminalStates())
}

func (cfg Config) IsActiveState(state string) bool {
	_, ok := cfg.ActiveStateSet()[normalizeState(state)]
	return ok
}

func (cfg Config) IsTerminalState(state string) bool {
	_, ok := cfg.TerminalStateSet()[normalizeState(state)]
	return ok
}

// ShouldSyncBackState reports whether workspace sync-back should run for the supplied state.
func (cfg Config) ShouldSyncBackState(state string) bool {
	if strings.TrimSpace(cfg.Workspace.SyncBack.Path) == "" {
		return false
	}
	normalized := normalizeState(state)
	for _, candidate := range cfg.Workspace.SyncBack.OnStates {
		if normalizeState(candidate) == normalized {
			return true
		}
	}
	return false
}

func (cfg Config) EffectiveTurnSandboxPolicy(workspace string) map[string]any {
	policy := cloneMap(cfg.Codex.TurnSandboxPolicy)
	if len(policy) == 0 {
		policy = map[string]any{"type": "workspaceWrite", "root": workspace}
	}
	if _, ok := policy["root"]; !ok && strings.EqualFold(asString(policy["type"]), "workspaceWrite") {
		policy["root"] = workspace
	}
	return policy
}

// DefaultPromptTemplate returns a generic fallback prompt for any tracker kind.
func DefaultPromptTemplate() string {
	return strings.TrimSpace(`You are working on a tracked task.

Identifier: {{ issue.identifier }}
Title: {{ issue.title }}
State: {{ issue.state }}

Body:
{% if issue.description %}
{{ issue.description }}
{% else %}
No description provided.
{% endif %}`)
}

func applyDefaults(cfg *Config) {
	if strings.TrimSpace(cfg.Tracker.Kind) == "" {
		cfg.Tracker.Kind = "jira"
	}
	if cfg.Tracker.Kind == "local" {
		if cfg.Local.ActiveStates == nil {
			cfg.Local.ActiveStates = []string{"To Do", "In Progress"}
		}
		if cfg.Local.TerminalStates == nil {
			cfg.Local.TerminalStates = []string{"Done", "Blocked"}
		}
	} else {
		if cfg.Tracker.ActiveStates == nil {
			cfg.Tracker.ActiveStates = []string{"To Do", "In Progress"}
		}
		if cfg.Tracker.TerminalStates == nil {
			cfg.Tracker.TerminalStates = []string{"Done", "Closed", "Cancelled", "Canceled", "Duplicate"}
		}
		if strings.TrimSpace(cfg.Tracker.AuthMode) == "" {
			cfg.Tracker.AuthMode = "token"
		}
	}
	if cfg.Orchestrator.PollIntervalMs <= 0 {
		cfg.Orchestrator.PollIntervalMs = 30_000
	}
	if cfg.Orchestrator.MaxConcurrentAgents <= 0 {
		cfg.Orchestrator.MaxConcurrentAgents = 4
	}
	if cfg.Orchestrator.MaxRetryBackoffMs <= 0 {
		cfg.Orchestrator.MaxRetryBackoffMs = 300_000
	}
	if strings.TrimSpace(cfg.Workspace.Root) == "" {
		cfg.Workspace.Root = filepath.Join(os.TempDir(), "symphony-workspaces")
	}
	cfg.Workspace.Seed.Excludes = normalizeWorkspaceExcludes(cfg.Workspace.Seed.Excludes)
	cfg.Workspace.SyncBack.Excludes = normalizeWorkspaceExcludes(cfg.Workspace.SyncBack.Excludes)
	if strings.TrimSpace(cfg.Workspace.SyncBack.Path) != "" && len(cfg.Workspace.SyncBack.OnStates) == 0 {
		if cfg.Tracker.Kind == "local" {
			cfg.Workspace.SyncBack.OnStates = []string{"Done"}
		} else {
			cfg.Workspace.SyncBack.OnStates = []string{"Done", "Closed"}
		}
	}
	if strings.TrimSpace(cfg.Local.InboxDir) == "" {
		cfg.Local.InboxDir = filepath.Join(".", "local_tasks", "inbox")
	}
	if strings.TrimSpace(cfg.Local.StateDir) == "" {
		cfg.Local.StateDir = filepath.Join(".", "local_tasks", "state")
	}
	if strings.TrimSpace(cfg.Local.ArchiveDir) == "" {
		cfg.Local.ArchiveDir = filepath.Join(".", "local_tasks", "archive")
	}
	if strings.TrimSpace(cfg.Local.ResultsDir) == "" {
		cfg.Local.ResultsDir = filepath.Join(".", "local_tasks", "results")
	}
	if cfg.Hooks.TimeoutMs <= 0 {
		cfg.Hooks.TimeoutMs = 60_000
	}
	if cfg.Agent.MaxTurns <= 0 {
		cfg.Agent.MaxTurns = 20
	}
	if strings.TrimSpace(cfg.Codex.Command) == "" {
		cfg.Codex.Command = "codex app-server"
	}
	if cfg.Codex.ApprovalPolicy == nil {
		cfg.Codex.ApprovalPolicy = "never"
	}
	if strings.TrimSpace(cfg.Codex.ThreadSandbox) == "" {
		cfg.Codex.ThreadSandbox = "workspace-write"
	}
	if cfg.Codex.ReadTimeoutMs <= 0 {
		cfg.Codex.ReadTimeoutMs = 5_000
	}
	if cfg.Codex.TurnTimeoutMs <= 0 {
		cfg.Codex.TurnTimeoutMs = 3_600_000
	}
	if cfg.Codex.StallTimeoutMs <= 0 {
		cfg.Codex.StallTimeoutMs = 300_000
	}
	if strings.TrimSpace(cfg.Server.Host) == "" {
		cfg.Server.Host = "127.0.0.1"
	}
	if cfg.Server.Port <= 0 {
		cfg.Server.Port = 8080
	}
}

func expandEnvironment(cfg *Config) {
	cfg.Tracker.BaseURL = expandString(cfg.Tracker.BaseURL)
	cfg.Tracker.Email = expandString(cfg.Tracker.Email)
	cfg.Tracker.APIToken = expandString(cfg.Tracker.APIToken)
	cfg.Tracker.ProjectKey = expandString(cfg.Tracker.ProjectKey)
	cfg.Tracker.JQL = expandString(cfg.Tracker.JQL)
	cfg.Tracker.WebhookSecret = expandString(cfg.Tracker.WebhookSecret)
	cfg.Local.InboxDir = expandPath(cfg.Local.InboxDir)
	cfg.Local.StateDir = expandPath(cfg.Local.StateDir)
	cfg.Local.ArchiveDir = expandPath(cfg.Local.ArchiveDir)
	cfg.Local.ResultsDir = expandPath(cfg.Local.ResultsDir)
	cfg.Workspace.Root = expandPath(cfg.Workspace.Root)
	cfg.Workspace.Seed.Path = expandPath(cfg.Workspace.Seed.Path)
	cfg.Workspace.Seed.Excludes = normalizeWorkspaceExcludes(expandStrings(cfg.Workspace.Seed.Excludes))
	cfg.Workspace.SyncBack.Path = expandPath(cfg.Workspace.SyncBack.Path)
	cfg.Workspace.SyncBack.OnStates = expandStrings(cfg.Workspace.SyncBack.OnStates)
	cfg.Workspace.SyncBack.Excludes = normalizeWorkspaceExcludes(expandStrings(cfg.Workspace.SyncBack.Excludes))
	cfg.Hooks.AfterCreate = expandString(cfg.Hooks.AfterCreate)
	cfg.Hooks.BeforeRun = expandString(cfg.Hooks.BeforeRun)
	cfg.Hooks.AfterRun = expandString(cfg.Hooks.AfterRun)
	cfg.Hooks.BeforeRemove = expandString(cfg.Hooks.BeforeRemove)
	cfg.Codex.Command = expandString(cfg.Codex.Command)
	cfg.Codex.ThreadSandbox = expandString(cfg.Codex.ThreadSandbox)
	cfg.Server.Host = expandString(cfg.Server.Host)
	cfg.Server.Username = expandString(cfg.Server.Username)
	cfg.Server.Password = expandString(cfg.Server.Password)
	cfg.Codex.ApprovalPolicy = expandValue(cfg.Codex.ApprovalPolicy)
	cfg.Codex.TurnSandboxPolicy = expandMap(cfg.Codex.TurnSandboxPolicy)
}

func validate(cfg Config) error {
	if cfg.Tracker.Kind != "jira" && cfg.Tracker.Kind != "local" {
		return fmt.Errorf("tracker.kind must be jira or local")
	}
	if cfg.Orchestrator.PollIntervalMs <= 0 {
		return fmt.Errorf("orchestrator.poll_interval_ms must be greater than 0")
	}
	if cfg.Orchestrator.MaxConcurrentAgents <= 0 {
		return fmt.Errorf("orchestrator.max_concurrent_agents must be greater than 0")
	}
	if cfg.Orchestrator.MaxRetryBackoffMs <= 0 {
		return fmt.Errorf("orchestrator.max_retry_backoff_ms must be greater than 0")
	}
	if strings.TrimSpace(cfg.Workspace.Root) == "" {
		return fmt.Errorf("workspace.root must not be empty")
	}
	if strings.TrimSpace(cfg.Workspace.SyncBack.Path) == "" && len(cfg.Workspace.SyncBack.OnStates) > 0 {
		return fmt.Errorf("workspace.sync_back.path must be set when workspace.sync_back.on_states is configured")
	}
	if strings.TrimSpace(cfg.Workspace.SyncBack.Path) != "" && len(cfg.Workspace.SyncBack.OnStates) == 0 {
		return fmt.Errorf("workspace.sync_back.on_states must not be empty when workspace.sync_back.path is set")
	}
	if cfg.Agent.MaxTurns <= 0 {
		return fmt.Errorf("agent.max_turns must be greater than 0")
	}
	if strings.TrimSpace(cfg.Codex.Command) == "" {
		return fmt.Errorf("codex.command must not be empty")
	}
	if cfg.Codex.ReadTimeoutMs <= 0 || cfg.Codex.TurnTimeoutMs <= 0 || cfg.Codex.StallTimeoutMs <= 0 {
		return fmt.Errorf("codex timeouts must be greater than 0")
	}
	if cfg.Server.Port < 0 || cfg.Server.Port > 65_535 {
		return fmt.Errorf("server.port must be between 0 and 65535")
	}
	if cfg.Tracker.Kind == "local" {
		if strings.TrimSpace(cfg.Local.InboxDir) == "" || strings.TrimSpace(cfg.Local.StateDir) == "" || strings.TrimSpace(cfg.Local.ArchiveDir) == "" || strings.TrimSpace(cfg.Local.ResultsDir) == "" {
			return fmt.Errorf("local task directories must not be empty")
		}
		if len(cfg.Local.ActiveStates) == 0 || len(cfg.Local.TerminalStates) == 0 {
			return fmt.Errorf("local.active_states and local.terminal_states must not be empty")
		}
	}
	usernameSet := strings.TrimSpace(cfg.Server.Username) != ""
	passwordSet := strings.TrimSpace(cfg.Server.Password) != ""
	if usernameSet != passwordSet {
		return fmt.Errorf("server.username and server.password must be set together")
	}
	return nil
}

func toStateSet(states []string) map[string]struct{} {
	result := make(map[string]struct{}, len(states))
	for _, state := range states {
		if normalized := normalizeState(state); normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	return result
}

func normalizeState(state string) string {
	return strings.ToLower(strings.TrimSpace(state))
}

func expandString(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return os.ExpandEnv(trimmed)
}

func expandPath(value string) string {
	expanded := expandString(value)
	if expanded == "" {
		return ""
	}
	if expanded == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	}
	if strings.HasPrefix(expanded, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, strings.TrimPrefix(expanded, "~/"))
		}
	}
	return expanded
}

func expandStrings(values []string) []string {
	if len(values) == 0 {
		return values
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if expanded := expandString(value); expanded != "" {
			result = append(result, expanded)
		}
	}
	return result
}

func expandMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return values
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = expandValue(value)
	}
	return result
}

func expandSlice(values []any) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, expandValue(value))
	}
	return result
}

func expandValue(value any) any {
	switch typed := value.(type) {
	case string:
		return expandString(typed)
	case map[string]any:
		return expandMap(typed)
	case []any:
		return expandSlice(typed)
	default:
		return value
	}
}

var defaultWorkspaceExcludes = []string{".git", "tmp"}

func normalizeWorkspaceExcludes(values []string) []string {
	combined := append(append([]string(nil), defaultWorkspaceExcludes...), values...)
	result := make([]string, 0, len(combined))
	seen := make(map[string]struct{}, len(combined))
	for _, value := range combined {
		normalized := normalizeWorkspaceExclude(value)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func normalizeWorkspaceExclude(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.Trim(trimmed, "/\\")
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return ""
	}
	return filepath.ToSlash(cleaned)
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return map[string]any{}
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return map[string]any{}
	}
	var cloned map[string]any
	if err := json.Unmarshal(payload, &cloned); err != nil {
		return map[string]any{}
	}
	return cloned
}

func asString(value any) string {
	text, _ := value.(string)
	return text
}

func sanitizedApprovalPolicy(value any) any {
	switch typed := value.(type) {
	case string:
		return typed
	case map[string]any:
		return cloneMap(typed)
	default:
		return typed
	}
}
