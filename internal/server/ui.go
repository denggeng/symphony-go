package server

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"time"
)

type pageData struct {
	Title           string
	PageKind        string
	IssueIdentifier string
}

var dashboardTemplate = template.Must(template.New("dashboard").Parse(`<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{{.Title}}</title>
    <style>
      :root {
        color-scheme: dark;
        --bg: #0b1020;
        --panel: #121936;
        --panel-border: #2a355f;
        --text: #e8eeff;
        --muted: #97a5d6;
        --accent: #6ea8fe;
        --accent-2: #4dd4ac;
        --danger: #ff6b6b;
        --warning: #f7c948;
        --shadow: 0 18px 40px rgba(0, 0, 0, 0.28);
      }
      * { box-sizing: border-box; }
      body {
        margin: 0;
        font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        background: radial-gradient(circle at top, #16224a 0%, var(--bg) 42%);
        color: var(--text);
      }
      a { color: var(--accent); text-decoration: none; }
      a:hover { text-decoration: underline; }
      .container {
        max-width: 1240px;
        margin: 0 auto;
        padding: 24px;
      }
      .topbar {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        gap: 16px;
        margin-bottom: 24px;
      }
      .headline {
        margin: 0;
        font-size: 32px;
        line-height: 1.15;
      }
      .subtle {
        color: var(--muted);
        margin-top: 8px;
        line-height: 1.5;
      }
      .actions {
        display: flex;
        gap: 12px;
        align-items: center;
        flex-wrap: wrap;
      }
      .button {
        appearance: none;
        border: 0;
        cursor: pointer;
        border-radius: 12px;
        padding: 12px 16px;
        font-weight: 600;
        color: #081224;
        background: linear-gradient(180deg, #8dc1ff, #5d98f5);
        box-shadow: var(--shadow);
      }
      .button.secondary {
        color: var(--text);
        background: linear-gradient(180deg, #1d2850, #182243);
        border: 1px solid var(--panel-border);
      }
      .button:disabled {
        opacity: 0.6;
        cursor: progress;
      }
      .status-pill {
        border-radius: 999px;
        padding: 8px 12px;
        font-size: 13px;
        font-weight: 600;
        background: rgba(110, 168, 254, 0.12);
        color: var(--accent);
        border: 1px solid rgba(110, 168, 254, 0.25);
      }
      .grid {
        display: grid;
        gap: 16px;
      }
      .cards {
        grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
        margin-bottom: 20px;
      }
      .card, .panel {
        background: rgba(18, 25, 54, 0.9);
        border: 1px solid var(--panel-border);
        border-radius: 18px;
        padding: 18px;
        box-shadow: var(--shadow);
        backdrop-filter: blur(12px);
      }
      .card h2, .panel h2 {
        margin: 0 0 10px;
        font-size: 16px;
      }
      .metric {
        font-size: 28px;
        font-weight: 800;
        margin: 4px 0;
      }
      .muted { color: var(--muted); }
      .error-banner {
        display: none;
        margin-bottom: 20px;
        border: 1px solid rgba(255, 107, 107, 0.4);
        background: rgba(255, 107, 107, 0.12);
        color: #ffd8d8;
        border-radius: 16px;
        padding: 16px;
      }
      .panel-grid {
        grid-template-columns: 1.1fr 0.9fr;
        margin-bottom: 20px;
      }
      .kv {
        display: grid;
        grid-template-columns: 180px 1fr;
        gap: 10px 14px;
        font-size: 14px;
      }
      .kv dt { color: var(--muted); }
      .kv dd { margin: 0; word-break: break-word; }
      .table-wrap { overflow-x: auto; }
      table {
        width: 100%;
        border-collapse: collapse;
        font-size: 14px;
      }
      th, td {
        padding: 12px 10px;
        text-align: left;
        border-bottom: 1px solid rgba(151, 165, 214, 0.16);
        vertical-align: top;
      }
      th {
        color: var(--muted);
        font-weight: 600;
        white-space: nowrap;
      }
      td code {
        font-size: 12px;
        color: #bdd1ff;
      }
      .empty {
        color: var(--muted);
        border: 1px dashed rgba(151, 165, 214, 0.25);
        border-radius: 14px;
        padding: 18px;
      }
      .tag {
        display: inline-flex;
        padding: 5px 10px;
        border-radius: 999px;
        font-size: 12px;
        font-weight: 700;
      }
      .tag.ok { background: rgba(77, 212, 172, 0.16); color: #90f0d0; }
      .tag.warn { background: rgba(247, 201, 72, 0.18); color: #ffe08a; }
      .tag.err { background: rgba(255, 107, 107, 0.16); color: #ffc2c2; }
      .split { display: grid; gap: 20px; grid-template-columns: 1fr; }
      .issue-view { display: none; }
      .dashboard-view { display: block; }
      body[data-page-kind="issue"] .issue-view { display: block; }
      body[data-page-kind="issue"] .dashboard-view { display: none; }
      .issue-title { margin: 0 0 8px; font-size: 28px; }
      .issue-meta {
        display: flex;
        flex-wrap: wrap;
        gap: 10px;
        margin-bottom: 18px;
      }
      .mono {
        font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace;
      }
      .footer {
        margin-top: 24px;
        color: var(--muted);
        font-size: 13px;
      }
      @media (max-width: 860px) {
        .topbar { flex-direction: column; }
        .panel-grid { grid-template-columns: 1fr; }
        .kv { grid-template-columns: 1fr; }
      }
    </style>
  </head>
  <body data-page-kind="{{.PageKind}}" data-issue-identifier="{{.IssueIdentifier}}">
    <div class="container">
      <header class="topbar">
        <div>
          <h1 class="headline">symphony-go dashboard</h1>
          <div class="subtle">
            Self-hosted Jira + Codex orchestration runtime.
            <span class="dashboard-view">Live snapshot of polling, running issues, and retries.</span>
            <span class="issue-view">Focused view for one running issue.</span>
          </div>
        </div>
        <div class="actions">
          <a class="button secondary" href="/">Overview</a>
          <button id="refresh-button" class="button">Refresh now</button>
          <div id="action-status" class="status-pill">Connecting live updates…</div>
        </div>
      </header>

      <section id="error-banner" class="error-banner"></section>

      <section class="grid cards">
        <article class="card">
          <h2>Running</h2>
          <div id="metric-running" class="metric">0</div>
          <div class="muted">Current issue runs</div>
        </article>
        <article class="card">
          <h2>Retry Queue</h2>
          <div id="metric-retrying" class="metric">0</div>
          <div class="muted">Scheduled retries</div>
        </article>
        <article class="card">
          <h2>Tracker</h2>
          <div id="metric-tracker" class="metric">—</div>
          <div id="metric-project" class="muted">Project —</div>
        </article>
        <article class="card">
          <h2>Poll Interval</h2>
          <div id="metric-poll-interval" class="metric">—</div>
          <div id="metric-next-poll" class="muted">Next poll —</div>
        </article>
      </section>

      <section class="grid panel-grid dashboard-view">
        <article class="panel">
          <h2>Runtime</h2>
          <dl class="kv">
            <dt>Service</dt><dd id="runtime-service">—</dd>
            <dt>Uptime</dt><dd id="runtime-uptime">—</dd>
            <dt>Workflow</dt><dd id="runtime-workflow">—</dd>
            <dt>Prompt Length</dt><dd id="runtime-prompt-length">—</dd>
            <dt>Polling</dt><dd id="runtime-polling">—</dd>
            <dt>Last Poll</dt><dd id="runtime-last-poll">—</dd>
            <dt>Next Poll</dt><dd id="runtime-next-poll">—</dd>
          </dl>
        </article>
        <article class="panel">
          <h2>Config Summary</h2>
          <dl class="kv">
            <dt>Workspace Root</dt><dd id="config-workspace">—</dd>
            <dt>Tracker JQL</dt><dd id="config-jql">—</dd>
            <dt>Active States</dt><dd id="config-active-states">—</dd>
            <dt>Terminal States</dt><dd id="config-terminal-states">—</dd>
            <dt>Max Agents</dt><dd id="config-max-agents">—</dd>
            <dt>Agent Max Turns</dt><dd id="config-max-turns">—</dd>
            <dt>Codex Command</dt><dd id="config-codex-command">—</dd>
          </dl>
        </article>
      </section>

      <section class="panel dashboard-view">
        <h2>Running Issues</h2>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Issue</th>
                <th>State</th>
                <th>Turns</th>
                <th>Runtime</th>
                <th>Session</th>
                <th>Last Event</th>
                <th>Workspace</th>
              </tr>
            </thead>
            <tbody id="running-body"></tbody>
          </table>
        </div>
        <div id="running-empty" class="empty">No issues are currently running.</div>
      </section>

      <section class="panel dashboard-view">
        <h2>Retry Queue</h2>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Issue</th>
                <th>Attempt</th>
                <th>Due In</th>
                <th>Type</th>
                <th>Error</th>
              </tr>
            </thead>
            <tbody id="retry-body"></tbody>
          </table>
        </div>
        <div id="retry-empty" class="empty">Retry queue is empty.</div>
      </section>

      <section class="panel issue-view">
        <h2 class="issue-title" id="issue-heading">Issue</h2>
        <div class="issue-meta">
          <span id="issue-state" class="tag ok">Unknown</span>
          <span id="issue-turns" class="tag warn">Turns: 0</span>
          <span id="issue-runtime" class="tag warn">Runtime: 0s</span>
        </div>
        <div id="issue-missing" class="empty" style="display:none;">This issue is not in the running set right now.</div>
        <div id="issue-content">
          <dl class="kv">
            <dt>Issue ID</dt><dd id="issue-id">—</dd>
            <dt>Session ID</dt><dd id="issue-session">—</dd>
            <dt>Codex PID</dt><dd id="issue-pid">—</dd>
            <dt>Retry Attempt</dt><dd id="issue-retry">—</dd>
            <dt>Last Event</dt><dd id="issue-last-event">—</dd>
            <dt>Last Message</dt><dd id="issue-last-message">—</dd>
            <dt>Last Timestamp</dt><dd id="issue-last-timestamp">—</dd>
            <dt>Workspace</dt><dd class="mono" id="issue-workspace">—</dd>
            <dt>Usage</dt><dd id="issue-usage">—</dd>
          </dl>
        </div>
      </section>

      <div class="footer">
        API: <a href="/api/v1/state">/api/v1/state</a> ·
        live stream: <a href="/events">/events</a> ·
        health: <a href="/healthz">/healthz</a>
      </div>
    </div>

    <script>
      const pageKind = document.body.dataset.pageKind || 'dashboard';
      const issueIdentifier = document.body.dataset.issueIdentifier || '';
      const refreshButton = document.getElementById('refresh-button');
      const actionStatus = document.getElementById('action-status');
      const errorBanner = document.getElementById('error-banner');

      function formatTime(value) {
        if (!value) return '—';
        const parsed = new Date(value);
        if (Number.isNaN(parsed.getTime())) return value;
        return parsed.toLocaleString();
      }

      function formatMs(value) {
        if (value === null || value === undefined) return '—';
        if (value < 1000) return value + ' ms';
        const seconds = Math.round((value / 1000) * 10) / 10;
        return seconds + ' s';
      }

      function statusClass(text) {
        const value = String(text || '').toLowerCase();
        if (value.includes('fail') || value.includes('error') || value.includes('cancel')) return 'err';
        if (value.includes('wait') || value.includes('retry') || value.includes('progress')) return 'warn';
        return 'ok';
      }

      function setText(id, value) {
        const node = document.getElementById(id);
        if (node) node.textContent = value ?? '—';
      }

      function escapeHTML(value) {
        return String(value ?? '—')
          .replace(/&/g, '&amp;')
          .replace(/</g, '&lt;')
          .replace(/>/g, '&gt;')
          .replace(/"/g, '&quot;')
          .replace(/'/g, '&#39;');
      }

      function showError(message) {
        if (!message) {
          errorBanner.style.display = 'none';
          errorBanner.textContent = '';
          return;
        }
        errorBanner.style.display = 'block';
        errorBanner.textContent = message;
      }

      function renderDashboard(snapshot) {
        const running = snapshot.running || [];
        const retrying = snapshot.retrying || [];
        const config = snapshot.config || {};
        const polling = snapshot.polling || {};
        const service = snapshot.service || {};
        const workflow = snapshot.workflow || {};

        setText('metric-running', String(running.length));
        setText('metric-retrying', String(retrying.length));
        setText('metric-tracker', (config.tracker && config.tracker.kind) || '—');
        setText('metric-project', (config.tracker && config.tracker.project_key) ? 'Project ' + config.tracker.project_key : 'Project —');
        setText('metric-poll-interval', formatMs(polling.poll_interval_ms));
        setText('metric-next-poll', 'Next poll ' + formatTime(polling.next_poll_at));

        setText('runtime-service', (service.name || 'symphony-go') + ' (' + (service.version || 'dev') + ')');
        setText('runtime-uptime', service.uptime || '—');
        setText('runtime-workflow', workflow.path || '—');
        setText('runtime-prompt-length', workflow.prompt_length != null ? workflow.prompt_length + ' chars' : '—');
        setText('runtime-polling', polling.checking ? 'Checking now…' : 'Idle');
        setText('runtime-last-poll', formatTime(polling.last_poll_at));
        setText('runtime-next-poll', formatTime(polling.next_poll_at));

        setText('config-workspace', config.workspace ? config.workspace.root : '—');
        setText('config-jql', config.tracker ? (config.tracker.jql || '—') : '—');
        setText('config-active-states', config.tracker ? (config.tracker.active_states || []).join(', ') : '—');
        setText('config-terminal-states', config.tracker ? (config.tracker.terminal_states || []).join(', ') : '—');
        setText('config-max-agents', config.orchestrator ? String(config.orchestrator.max_concurrent_agents ?? '—') : '—');
        setText('config-max-turns', config.agent ? String(config.agent.max_turns ?? '—') : '—');
        setText('config-codex-command', config.codex ? (config.codex.command || '—') : '—');

        showError(polling.last_error || '');

        const runningBody = document.getElementById('running-body');
        const runningEmpty = document.getElementById('running-empty');
        runningBody.innerHTML = '';
        if (!running.length) {
          runningEmpty.style.display = 'block';
        } else {
          runningEmpty.style.display = 'none';
          running.forEach(item => {
            const row = document.createElement('tr');
            const issueLink = '/issues/' + encodeURIComponent(item.identifier);
            row.innerHTML = '' +
              '<td><a href="' + issueLink + '">' + escapeHTML(item.identifier) + '</a><div class="muted mono">' + escapeHTML(item.issue_id) + '</div></td>' +
              '<td><span class="tag ' + statusClass(item.state) + '">' + escapeHTML(item.state || 'Unknown') + '</span></td>' +
              '<td>' + escapeHTML(item.turns) + '</td>' +
              '<td>' + escapeHTML(item.runtime_seconds) + 's</td>' +
              '<td class="mono">' + escapeHTML(item.session_id || '—') + '</td>' +
              '<td>' + escapeHTML(item.last_event || '—') + '<div class="muted">' + escapeHTML(item.last_message || '') + '</div></td>' +
              '<td><code>' + escapeHTML(item.workspace_path || '—') + '</code></td>';
            runningBody.appendChild(row);
          });
        }

        const retryBody = document.getElementById('retry-body');
        const retryEmpty = document.getElementById('retry-empty');
        retryBody.innerHTML = '';
        if (!retrying.length) {
          retryEmpty.style.display = 'block';
        } else {
          retryEmpty.style.display = 'none';
          retrying.forEach(item => {
            const row = document.createElement('tr');
            row.innerHTML = '' +
              '<td>' + escapeHTML(item.identifier) + '<div class="muted mono">' + escapeHTML(item.issue_id) + '</div></td>' +
              '<td>' + escapeHTML(item.attempt) + '</td>' +
              '<td>' + escapeHTML(formatMs(item.due_in_ms)) + '</td>' +
              '<td><span class="tag ' + (item.continuation ? 'warn' : 'err') + '">' + escapeHTML(item.continuation ? 'Continuation' : 'Failure') + '</span></td>' +
              '<td>' + escapeHTML(item.error || '—') + '</td>';
            retryBody.appendChild(row);
          });
        }

        renderIssue(snapshot);
      }

      function renderIssue(snapshot) {
        if (pageKind !== 'issue') return;
        const running = snapshot.running || [];
        const issue = running.find(item => item.identifier === issueIdentifier || item.issue_id === issueIdentifier);
        setText('issue-heading', issueIdentifier || 'Issue');
        const missing = document.getElementById('issue-missing');
        const content = document.getElementById('issue-content');
        if (!issue) {
          missing.style.display = 'block';
          content.style.display = 'none';
          document.getElementById('issue-state').className = 'tag err';
          setText('issue-state', 'Not running');
          setText('issue-turns', 'Turns: 0');
          setText('issue-runtime', 'Runtime: 0s');
          return;
        }
        missing.style.display = 'none';
        content.style.display = 'block';
        const stateNode = document.getElementById('issue-state');
        stateNode.className = 'tag ' + statusClass(issue.state);
        setText('issue-state', issue.state || 'Unknown');
        setText('issue-turns', 'Turns: ' + issue.turns);
        setText('issue-runtime', 'Runtime: ' + issue.runtime_seconds + 's');
        setText('issue-heading', issue.identifier);
        setText('issue-id', issue.issue_id || '—');
        setText('issue-session', issue.session_id || '—');
        setText('issue-pid', issue.codex_app_server_pid || '—');
        setText('issue-retry', String(issue.retry_attempt ?? 0));
        setText('issue-last-event', issue.last_event || '—');
        setText('issue-last-message', issue.last_message || '—');
        setText('issue-last-timestamp', formatTime(issue.last_timestamp));
        setText('issue-workspace', issue.workspace_path || '—');
        const usage = issue.usage || {};
        setText('issue-usage', 'input=' + (usage.input_tokens || 0) + ', output=' + (usage.output_tokens || 0) + ', total=' + (usage.total_tokens || 0));
      }

      async function loadSnapshot() {
        const response = await fetch('/api/v1/state');
        if (!response.ok) throw new Error('Failed to load snapshot');
        return response.json();
      }

      async function refreshNow() {
        refreshButton.disabled = true;
        actionStatus.textContent = 'Requesting refresh…';
        try {
          const response = await fetch('/api/v1/refresh', { method: 'POST' });
          const payload = await response.json();
          actionStatus.textContent = payload.coalesced ? 'Refresh already queued' : 'Refresh requested';
        } catch (error) {
          actionStatus.textContent = 'Refresh failed';
        } finally {
          refreshButton.disabled = false;
        }
      }

      refreshButton.addEventListener('click', refreshNow);

      (async function init() {
        try {
          const snapshot = await loadSnapshot();
          renderDashboard(snapshot);
          actionStatus.textContent = 'Live updates connected';
        } catch (error) {
          actionStatus.textContent = 'Initial load failed';
          showError(String(error));
        }

        const events = new EventSource('/events');
        events.onmessage = event => {
          try {
            renderDashboard(JSON.parse(event.data));
            actionStatus.textContent = 'Live updates connected';
          } catch (_error) {
            actionStatus.textContent = 'Live update parse failed';
          }
        };
        events.onerror = () => {
          actionStatus.textContent = 'Live updates disconnected';
        };

        setInterval(async () => {
          if (document.hidden) return;
          try {
            const snapshot = await loadSnapshot();
            renderDashboard(snapshot);
          } catch (_error) {
          }
        }, 15000);
      })();
    </script>
  </body>
</html>`))

func (server *Server) renderPage(writer http.ResponseWriter, status int, data pageData) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)
	if err := dashboardTemplate.Execute(writer, data); err != nil {
		server.logger.Error("render html page", "error", err)
	}
}

func (server *Server) handleDashboardPage(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		server.writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if request.URL.Path != "/" {
		http.NotFound(writer, request)
		return
	}
	server.renderPage(writer, http.StatusOK, pageData{Title: "symphony-go dashboard", PageKind: "dashboard"})
}

func (server *Server) handleIssuePage(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		server.writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identifier := strings.TrimSpace(strings.TrimPrefix(request.URL.Path, "/issues/"))
	if identifier == "" {
		http.NotFound(writer, request)
		return
	}
	server.renderPage(writer, http.StatusOK, pageData{Title: "Issue · symphony-go", PageKind: "issue", IssueIdentifier: identifier})
}

func (server *Server) handleEvents(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		server.writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if server.controller == nil {
		server.writeError(writer, http.StatusServiceUnavailable, "controller unavailable")
		return
	}
	flusher, ok := writer.(http.Flusher)
	if !ok {
		server.writeError(writer, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")

	pushSnapshot := func() bool {
		payload, err := json.Marshal(server.controller.Snapshot())
		if err != nil {
			server.logger.Error("encode event snapshot", "error", err)
			return false
		}
		if _, err := writer.Write([]byte("data: ")); err != nil {
			return false
		}
		if _, err := writer.Write(payload); err != nil {
			return false
		}
		if _, err := writer.Write([]byte("\n\n")); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if !pushSnapshot() {
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case <-ticker.C:
			if !pushSnapshot() {
				return
			}
		}
	}
}
