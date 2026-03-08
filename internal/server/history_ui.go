package server

import (
	"html/template"
	"net/http"
	"strings"
)

type historyPageData struct {
	Title    string
	PageKind string
	RunID    string
}

var historyTemplate = template.Must(template.New("history").Parse(`<!DOCTYPE html>
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
      .container { max-width: 1240px; margin: 0 auto; padding: 24px; }
      .topbar {
        display: flex;
        justify-content: space-between;
        align-items: flex-start;
        gap: 16px;
        margin-bottom: 24px;
      }
      .headline { margin: 0; font-size: 32px; line-height: 1.15; }
      .subtle { color: var(--muted); margin-top: 8px; line-height: 1.5; }
      .actions { display: flex; gap: 12px; align-items: center; flex-wrap: wrap; }
      .button {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        border-radius: 12px;
        padding: 12px 16px;
        font-weight: 600;
        color: var(--text);
        background: linear-gradient(180deg, #1d2850, #182243);
        border: 1px solid var(--panel-border);
      }
      .panel {
        background: rgba(18, 25, 54, 0.9);
        border: 1px solid var(--panel-border);
        border-radius: 18px;
        padding: 18px;
        box-shadow: var(--shadow);
        backdrop-filter: blur(12px);
      }
      .panel h2 { margin: 0 0 10px; font-size: 18px; }
      .error-banner {
        display: none;
        margin-bottom: 20px;
        border: 1px solid rgba(255, 107, 107, 0.4);
        background: rgba(255, 107, 107, 0.12);
        color: #ffd8d8;
        border-radius: 16px;
        padding: 16px;
      }
      .table-wrap { overflow-x: auto; }
      table { width: 100%; border-collapse: collapse; font-size: 14px; }
      th, td {
        padding: 12px 10px;
        text-align: left;
        border-bottom: 1px solid rgba(151, 165, 214, 0.16);
        vertical-align: top;
      }
      th { color: var(--muted); font-weight: 600; white-space: nowrap; }
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
      .grid { display: grid; gap: 16px; }
      .meta { grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); margin-bottom: 20px; }
      .card {
        background: rgba(18, 25, 54, 0.9);
        border: 1px solid var(--panel-border);
        border-radius: 18px;
        padding: 18px;
        box-shadow: var(--shadow);
      }
      .metric { font-size: 24px; font-weight: 800; margin: 6px 0; }
      .muted { color: var(--muted); }
      .kv {
        display: grid;
        grid-template-columns: 180px 1fr;
        gap: 10px 14px;
        font-size: 14px;
      }
      .kv dt { color: var(--muted); }
      .kv dd { margin: 0; word-break: break-word; }
      .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", monospace; }
      .history-page { display: block; }
      .run-page { display: none; }
      body[data-page-kind="run"] .history-page { display: none; }
      body[data-page-kind="run"] .run-page { display: block; }
      .footer { margin-top: 24px; color: var(--muted); font-size: 13px; }
      @media (max-width: 860px) {
        .topbar { flex-direction: column; }
        .kv { grid-template-columns: 1fr; }
      }
    </style>
  </head>
  <body data-page-kind="{{.PageKind}}" data-run-id="{{.RunID}}">
    <div class="container">
      <header class="topbar">
        <div>
          <h1 class="headline">run history</h1>
          <div class="subtle">
            Recent completed runs and sanitized event logs for self-hosted operations.
          </div>
        </div>
        <div class="actions">
          <a class="button" href="/">Overview</a>
          <a class="button" href="/history">History</a>
        </div>
      </header>

      <section id="error-banner" class="error-banner"></section>

      <section class="history-page">
        <section class="grid meta">
          <article class="card">
            <h2>Total Runs</h2>
            <div id="metric-total" class="metric">0</div>
            <div class="muted">In-memory retention window</div>
          </article>
          <article class="card">
            <h2>Successful</h2>
            <div id="metric-success" class="metric">0</div>
            <div class="muted">Succeeded without retry continuation</div>
          </article>
          <article class="card">
            <h2>Failed</h2>
            <div id="metric-failed" class="metric">0</div>
            <div class="muted">Failed or stopped runs</div>
          </article>
        </section>

        <section class="panel">
          <h2>Recent Runs</h2>
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Run</th>
                  <th>Status</th>
                  <th>Attempt</th>
                  <th>Finished</th>
                  <th>Runtime</th>
                  <th>Turns</th>
                  <th>Events</th>
                  <th>Workspace</th>
                </tr>
              </thead>
              <tbody id="history-body"></tbody>
            </table>
          </div>
          <div id="history-empty" class="empty">No completed runs recorded yet.</div>
        </section>
      </section>

      <section class="run-page">
        <section class="panel" style="margin-bottom:20px;">
          <h2 id="run-heading">Run detail</h2>
          <dl class="kv">
            <dt>Run ID</dt><dd class="mono" id="run-id">—</dd>
            <dt>Issue</dt><dd id="run-issue">—</dd>
            <dt>Status</dt><dd id="run-status">—</dd>
            <dt>Attempt</dt><dd id="run-attempt">—</dd>
            <dt>Started</dt><dd id="run-started">—</dd>
            <dt>Finished</dt><dd id="run-finished">—</dd>
            <dt>Runtime</dt><dd id="run-runtime">—</dd>
            <dt>Turns</dt><dd id="run-turns">—</dd>
            <dt>Workspace</dt><dd class="mono" id="run-workspace">—</dd>
            <dt>Session</dt><dd class="mono" id="run-session">—</dd>
            <dt>PID</dt><dd id="run-pid">—</dd>
            <dt>Last Event</dt><dd id="run-last-event">—</dd>
            <dt>Last Message</dt><dd id="run-last-message">—</dd>
            <dt>Error</dt><dd id="run-error">—</dd>
            <dt>Usage</dt><dd id="run-usage">—</dd>
          </dl>
        </section>

        <section class="panel">
          <h2>Event Log</h2>
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Type</th>
                  <th>Turn</th>
                  <th>Message</th>
                  <th>Usage</th>
                </tr>
              </thead>
              <tbody id="event-body"></tbody>
            </table>
          </div>
          <div id="event-empty" class="empty">This run has no recorded events.</div>
        </section>
      </section>

      <div class="footer">
        API: <a href="/api/v1/history">/api/v1/history</a> · overview: <a href="/">/</a>
      </div>
    </div>

    <script>
      const pageKind = document.body.dataset.pageKind || 'history';
      const runID = document.body.dataset.runId || '';
      const errorBanner = document.getElementById('error-banner');

      function setText(id, value) {
        const node = document.getElementById(id);
        if (node) node.textContent = value ?? '—';
      }

      function escapeHTML(value) {
        return String(value ?? '—')
          .replace(/&/g, '&amp;')
          .replace(/</g, '&lt;')
          .replace(/>/g, '&gt;')
          .replace(/\"/g, '&quot;')
          .replace(/'/g, '&#39;');
      }

      function formatTime(value) {
        if (!value) return '—';
        const parsed = new Date(value);
        if (Number.isNaN(parsed.getTime())) return value;
        return parsed.toLocaleString();
      }

      function formatDuration(value) {
        if (value === null || value === undefined) return '—';
        return String(value) + 's';
      }

      function usageText(usage) {
        const value = usage || {};
        return 'input=' + (value.input_tokens || 0) + ', output=' + (value.output_tokens || 0) + ', total=' + (value.total_tokens || 0);
      }

      function statusClass(text) {
        const value = String(text || '').toLowerCase();
        if (value.includes('fail') || value.includes('error')) return 'err';
        if (value.includes('continue') || value.includes('stop')) return 'warn';
        return 'ok';
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

      async function loadHistory() {
        const response = await fetch('/api/v1/history');
        if (!response.ok) throw new Error('Failed to load history');
        return response.json();
      }

      async function loadRun() {
        const response = await fetch('/api/v1/history/' + encodeURIComponent(runID));
        if (!response.ok) throw new Error('Failed to load run detail');
        return response.json();
      }

      function renderHistory(payload) {
        const runs = (payload && payload.runs) || [];
        setText('metric-total', String(runs.length));
        setText('metric-success', String(runs.filter(item => item.status === 'succeeded').length));
        setText('metric-failed', String(runs.filter(item => item.status !== 'succeeded').length));

        const body = document.getElementById('history-body');
        const empty = document.getElementById('history-empty');
        body.innerHTML = '';
        if (!runs.length) {
          empty.style.display = 'block';
          return;
        }
        empty.style.display = 'none';
        runs.forEach(item => {
          const row = document.createElement('tr');
          const detailLink = '/history/' + encodeURIComponent(item.run_id);
          const issueLink = item.identifier ? '/issues/' + encodeURIComponent(item.identifier) : '';
          row.innerHTML = '' +
            '<td><a href="' + detailLink + '">' + escapeHTML(item.run_id) + '</a><div class="muted">' + (issueLink ? '<a href="' + issueLink + '">' + escapeHTML(item.identifier) + '</a>' : escapeHTML(item.identifier || '—')) + '</div></td>' +
            '<td><span class="tag ' + statusClass(item.status) + '">' + escapeHTML(item.status) + '</span></td>' +
            '<td>' + escapeHTML(item.retry_attempt) + '</td>' +
            '<td>' + escapeHTML(formatTime(item.finished_at)) + '</td>' +
            '<td>' + escapeHTML(formatDuration(item.runtime_seconds)) + '</td>' +
            '<td>' + escapeHTML(item.turns) + '</td>' +
            '<td>' + escapeHTML(item.event_count) + '</td>' +
            '<td><span class="mono">' + escapeHTML(item.workspace_path || '—') + '</span></td>';
          body.appendChild(row);
        });
      }

      function renderRun(detail) {
        const run = (detail && detail.run) || {};
        const events = (detail && detail.events) || [];
        document.title = (run.identifier || 'Run') + ' · run history';
        setText('run-heading', run.identifier ? run.identifier + ' run detail' : 'Run detail');
        setText('run-id', run.run_id || '—');
        setText('run-issue', (run.identifier || '—') + (run.title ? ' — ' + run.title : ''));
        setText('run-status', run.status || '—');
        const runStatus = document.getElementById('run-status');
        if (runStatus) runStatus.className = 'tag ' + statusClass(run.status);
        setText('run-attempt', String(run.retry_attempt ?? 0));
        setText('run-started', formatTime(run.started_at));
        setText('run-finished', formatTime(run.finished_at));
        setText('run-runtime', formatDuration(run.runtime_seconds));
        setText('run-turns', String(run.turns ?? 0));
        setText('run-workspace', run.workspace_path || '—');
        setText('run-session', run.session_id || '—');
        setText('run-pid', run.codex_app_server_pid || '—');
        setText('run-last-event', run.last_event || '—');
        setText('run-last-message', run.last_message || '—');
        setText('run-error', run.error || '—');
        setText('run-usage', usageText(run.usage));

        const body = document.getElementById('event-body');
        const empty = document.getElementById('event-empty');
        body.innerHTML = '';
        if (!events.length) {
          empty.style.display = 'block';
          return;
        }
        empty.style.display = 'none';
        events.forEach(item => {
          const row = document.createElement('tr');
          row.innerHTML = '' +
            '<td>' + escapeHTML(formatTime(item.timestamp)) + '</td>' +
            '<td><span class="tag ' + statusClass(item.type) + '">' + escapeHTML(item.type) + '</span></td>' +
            '<td class="mono">' + escapeHTML(item.turn_id || '—') + '</td>' +
            '<td>' + escapeHTML(item.message || '—') + '</td>' +
            '<td>' + escapeHTML(usageText(item.usage)) + '</td>';
          body.appendChild(row);
        });
      }

      (async function init() {
        try {
          if (pageKind === 'run') {
            renderRun(await loadRun());
            return;
          }
          renderHistory(await loadHistory());
          setInterval(async () => {
            if (document.hidden) return;
            try {
              renderHistory(await loadHistory());
            } catch (_error) {
            }
          }, 15000);
        } catch (error) {
          showError(String(error));
        }
      })();
    </script>
  </body>
</html>`))

func (server *Server) renderHistoryUI(writer http.ResponseWriter, status int, data historyPageData) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)
	if err := historyTemplate.Execute(writer, data); err != nil {
		server.logger.Error("render history html page", "error", err)
	}
}

func (server *Server) handleHistoryPage(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		server.writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if request.URL.Path != "/history" {
		http.NotFound(writer, request)
		return
	}
	if !server.requireAuth(writer, request) {
		return
	}
	server.renderHistoryUI(writer, http.StatusOK, historyPageData{Title: "Run history · symphony-go", PageKind: "history"})
}

func (server *Server) handleRunHistoryPage(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		server.writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	runID := strings.TrimSpace(strings.TrimPrefix(request.URL.Path, "/history/"))
	if runID == "" {
		http.NotFound(writer, request)
		return
	}
	if !server.requireAuth(writer, request) {
		return
	}
	server.renderHistoryUI(writer, http.StatusOK, historyPageData{Title: "Run detail · symphony-go", PageKind: "run", RunID: runID})
}
