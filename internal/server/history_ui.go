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
        appearance: none;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        border-radius: 12px;
        padding: 12px 16px;
        font-weight: 600;
        font-size: 14px;
        line-height: 1;
        color: var(--text);
        background: linear-gradient(180deg, #1d2850, #182243);
        border: 1px solid var(--panel-border);
        cursor: pointer;
      }
      .lang-toggle { min-width: 92px; }
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
          <h1 class="headline" data-i18n="history.headline">run history</h1>
          <div class="subtle" data-i18n="history.subtle">
            Recent completed runs and sanitized event logs for self-hosted operations.
          </div>
        </div>
        <div class="actions">
          <a class="button" href="/" data-i18n="nav.overview">Overview</a>
          <a class="button" href="/history" data-i18n="nav.history">History</a>
          <button id="language-toggle" class="button lang-toggle" type="button">中文</button>
        </div>
      </header>

      <section id="error-banner" class="error-banner"></section>

      <section class="history-page">
        <section class="grid meta">
          <article class="card">
            <h2 data-i18n="metric.total">Total Runs</h2>
            <div id="metric-total" class="metric">0</div>
            <div class="muted" data-i18n="metric.totalDesc">In-memory retention window</div>
          </article>
          <article class="card">
            <h2 data-i18n="metric.success">Successful</h2>
            <div id="metric-success" class="metric">0</div>
            <div class="muted" data-i18n="metric.successDesc">Succeeded without retry continuation</div>
          </article>
          <article class="card">
            <h2 data-i18n="metric.failed">Failed</h2>
            <div id="metric-failed" class="metric">0</div>
            <div class="muted" data-i18n="metric.failedDesc">Failed or stopped runs</div>
          </article>
        </section>

        <section class="panel">
          <h2 data-i18n="history.sectionTitle">Recent Runs</h2>
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th data-i18n="table.run">Run</th>
                  <th data-i18n="table.status">Status</th>
                  <th data-i18n="table.attempt">Attempt</th>
                  <th data-i18n="table.finished">Finished</th>
                  <th data-i18n="table.runtime">Runtime</th>
                  <th data-i18n="table.turns">Turns</th>
                  <th data-i18n="table.events">Events</th>
                  <th data-i18n="table.workspace">Workspace</th>
                </tr>
              </thead>
              <tbody id="history-body"></tbody>
            </table>
          </div>
          <div id="history-empty" class="empty" data-i18n="history.empty">No completed runs recorded yet.</div>
        </section>
      </section>

      <section class="run-page">
        <section class="panel" style="margin-bottom:20px;">
          <h2 id="run-heading" data-i18n="run.heading">Run detail</h2>
          <dl class="kv">
            <dt data-i18n="run.runID">Run ID</dt><dd class="mono" id="run-id">—</dd>
            <dt data-i18n="run.issue">Issue</dt><dd id="run-issue">—</dd>
            <dt data-i18n="run.status">Status</dt><dd id="run-status">—</dd>
            <dt data-i18n="run.attempt">Attempt</dt><dd id="run-attempt">—</dd>
            <dt data-i18n="run.started">Started</dt><dd id="run-started">—</dd>
            <dt data-i18n="run.finished">Finished</dt><dd id="run-finished">—</dd>
            <dt data-i18n="run.runtime">Runtime</dt><dd id="run-runtime">—</dd>
            <dt data-i18n="run.turns">Turns</dt><dd id="run-turns">—</dd>
            <dt data-i18n="run.workspace">Workspace</dt><dd class="mono" id="run-workspace">—</dd>
            <dt data-i18n="run.session">Session</dt><dd class="mono" id="run-session">—</dd>
            <dt data-i18n="run.pid">PID</dt><dd id="run-pid">—</dd>
            <dt data-i18n="run.lastEvent">Last Event</dt><dd id="run-last-event">—</dd>
            <dt data-i18n="run.lastMessage">Last Message</dt><dd id="run-last-message">—</dd>
            <dt data-i18n="run.error">Error</dt><dd id="run-error">—</dd>
            <dt data-i18n="run.usage">Usage</dt><dd id="run-usage">—</dd>
          </dl>
        </section>

        <section class="panel">
          <h2 data-i18n="eventLog.title">Event Log</h2>
          <div class="table-wrap">
            <table>
              <thead>
                <tr>
                  <th data-i18n="table.time">Time</th>
                  <th data-i18n="table.type">Type</th>
                  <th data-i18n="table.turn">Turn</th>
                  <th data-i18n="table.message">Message</th>
                  <th data-i18n="table.usage">Usage</th>
                </tr>
              </thead>
              <tbody id="event-body"></tbody>
            </table>
          </div>
          <div id="event-empty" class="empty" data-i18n="event.empty">This run has no recorded events.</div>
        </section>
      </section>

      <div class="footer" data-i18n-html="footer.html">
        API: <a href="/api/v1/history">/api/v1/history</a> · overview: <a href="/">/</a>
      </div>
    </div>

    <script>
      const translations = {
        'page.title.history': { en: 'Run history · symphony-go', zh: '运行历史 · symphony-go' },
        'page.title.runDetail': { en: 'Run detail · symphony-go', zh: '运行详情 · symphony-go' },
        'page.title.runSuffix': { en: 'run history', zh: '运行历史' },
        'history.headline': { en: 'run history', zh: '运行历史' },
        'history.subtle': { en: 'Recent completed runs and sanitized event logs for self-hosted operations.', zh: '查看最近完成的运行记录，以及适合自托管场景的脱敏事件日志。' },
        'nav.overview': { en: 'Overview', zh: '总览' },
        'nav.history': { en: 'History', zh: '历史' },
        'metric.total': { en: 'Total Runs', zh: '总运行数' },
        'metric.totalDesc': { en: 'In-memory retention window', zh: '内存保留窗口' },
        'metric.success': { en: 'Successful', zh: '成功' },
        'metric.successDesc': { en: 'Succeeded without retry continuation', zh: '未经过续跑重试而成功' },
        'metric.failed': { en: 'Failed', zh: '失败' },
        'metric.failedDesc': { en: 'Failed or stopped runs', zh: '失败或停止的运行' },
        'history.sectionTitle': { en: 'Recent Runs', zh: '最近运行' },
        'table.run': { en: 'Run', zh: '运行' },
        'table.status': { en: 'Status', zh: '状态' },
        'table.attempt': { en: 'Attempt', zh: '尝试次数' },
        'table.finished': { en: 'Finished', zh: '完成时间' },
        'table.runtime': { en: 'Runtime', zh: '运行时长' },
        'table.turns': { en: 'Turns', zh: '轮次' },
        'table.events': { en: 'Events', zh: '事件数' },
        'table.workspace': { en: 'Workspace', zh: '工作区' },
        'table.time': { en: 'Time', zh: '时间' },
        'table.type': { en: 'Type', zh: '类型' },
        'table.message': { en: 'Message', zh: '消息' },
        'table.usage': { en: 'Usage', zh: '用量' },
        'table.turn': { en: 'Turn', zh: '轮次' },
        'history.empty': { en: 'No completed runs recorded yet.', zh: '还没有已完成的运行记录。' },
        'run.heading': { en: 'Run detail', zh: '运行详情' },
        'run.runID': { en: 'Run ID', zh: '运行 ID' },
        'run.issue': { en: 'Issue', zh: '任务' },
        'run.status': { en: 'Status', zh: '状态' },
        'run.attempt': { en: 'Attempt', zh: '尝试次数' },
        'run.started': { en: 'Started', zh: '开始时间' },
        'run.finished': { en: 'Finished', zh: '完成时间' },
        'run.runtime': { en: 'Runtime', zh: '运行时长' },
        'run.turns': { en: 'Turns', zh: '轮次' },
        'run.workspace': { en: 'Workspace', zh: '工作区' },
        'run.session': { en: 'Session', zh: '会话' },
        'run.pid': { en: 'PID', zh: 'PID' },
        'run.lastEvent': { en: 'Last Event', zh: '最近事件' },
        'run.lastMessage': { en: 'Last Message', zh: '最近消息' },
        'run.error': { en: 'Error', zh: '错误' },
        'run.usage': { en: 'Usage', zh: '用量' },
        'eventLog.title': { en: 'Event Log', zh: '事件日志' },
        'event.empty': { en: 'This run has no recorded events.', zh: '这次运行还没有记录任何事件。' },
        'footer.html': { en: 'API: <a href="/api/v1/history">/api/v1/history</a> · overview: <a href="/">/</a>', zh: 'API：<a href="/api/v1/history">/api/v1/history</a> · 总览：<a href="/">/</a>' },
        'error.loadHistory': { en: 'Failed to load history', zh: '加载历史记录失败' },
        'error.loadRun': { en: 'Failed to load run detail', zh: '加载运行详情失败' },
        'unit.s': { en: 's', zh: '秒' },
        'usage.input': { en: 'input', zh: '输入' },
        'usage.output': { en: 'output', zh: '输出' },
        'usage.total': { en: 'total', zh: '总计' },
        'usage.cached': { en: 'cached', zh: '缓存' },
        'usage.reasoning': { en: 'reasoning', zh: '推理' }
      };

      const pageKind = document.body.dataset.pageKind || 'history';
      const runID = document.body.dataset.runId || '';
      const languageToggle = document.getElementById('language-toggle');
      const errorBanner = document.getElementById('error-banner');
      let latestHistoryPayload = null;
      let latestRunDetail = null;
      let currentLanguage = detectLanguage();

      function detectLanguage() {
        try {
          const stored = localStorage.getItem('symphony-ui-language');
          if (stored === 'zh' || stored === 'en') return stored;
        } catch (_error) {
        }
        return (navigator.language || '').toLowerCase().startsWith('zh') ? 'zh' : 'en';
      }

      function saveLanguage() {
        try {
          localStorage.setItem('symphony-ui-language', currentLanguage);
        } catch (_error) {
        }
      }

      function tr(key) {
        const entry = translations[key];
        if (!entry) return key;
        return currentLanguage === 'zh' ? (entry.zh || entry.en) : entry.en;
      }

      function setLanguageToggleLabel() {
        if (!languageToggle) return;
        languageToggle.textContent = currentLanguage === 'zh' ? 'English' : '中文';
        languageToggle.setAttribute('aria-label', currentLanguage === 'zh' ? 'Switch to English' : '切换到中文');
      }

      function updatePageTitle(run) {
        if (pageKind === 'run') {
          const identifier = run && run.identifier ? run.identifier : '';
          document.title = identifier ? identifier + ' · ' + tr('page.title.runSuffix') : tr('page.title.runDetail');
          return;
        }
        document.title = tr('page.title.history');
      }

      function applyStaticTranslations() {
        document.documentElement.lang = currentLanguage === 'zh' ? 'zh-CN' : 'en';
        document.querySelectorAll('[data-i18n]').forEach(function(node) {
          node.textContent = tr(node.dataset.i18n);
        });
        document.querySelectorAll('[data-i18n-html]').forEach(function(node) {
          node.innerHTML = tr(node.dataset.i18nHtml);
        });
        setLanguageToggleLabel();
        updatePageTitle(latestRunDetail ? latestRunDetail.run : null);
      }

      function toggleLanguage() {
        currentLanguage = currentLanguage === 'zh' ? 'en' : 'zh';
        saveLanguage();
        applyStaticTranslations();
        if (pageKind === 'run') {
          if (latestRunDetail) renderRun(latestRunDetail);
        } else if (latestHistoryPayload) {
          renderHistory(latestHistoryPayload);
        }
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
          .replace(/\"/g, '&quot;')
          .replace(/'/g, '&#39;');
      }

      function formatTime(value) {
        if (!value) return '—';
        const parsed = new Date(value);
        if (Number.isNaN(parsed.getTime())) return value;
        return parsed.toLocaleString(currentLanguage === 'zh' ? 'zh-CN' : 'en-US');
      }

      function formatDuration(value) {
        if (value === null || value === undefined) return '—';
        return String(value) + tr('unit.s');
      }

      function usageText(usage) {
        const value = usage || {};
        let text = tr('usage.input') + '=' + (value.input_tokens || 0) + ', ' + tr('usage.output') + '=' + (value.output_tokens || 0) + ', ' + tr('usage.total') + '=' + (value.total_tokens || 0);
        if (value.cached_input_tokens) text += ', ' + tr('usage.cached') + '=' + value.cached_input_tokens;
        if (value.reasoning_output_tokens) text += ', ' + tr('usage.reasoning') + '=' + value.reasoning_output_tokens;
        return text;
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
        if (!response.ok) throw new Error(tr('error.loadHistory'));
        return response.json();
      }

      async function loadRun() {
        const response = await fetch('/api/v1/history/' + encodeURIComponent(runID));
        if (!response.ok) throw new Error(tr('error.loadRun'));
        return response.json();
      }

      function renderHistory(payload) {
        latestHistoryPayload = payload;
        const runs = (payload && payload.runs) || [];
        setText('metric-total', String(runs.length));
        setText('metric-success', String(runs.filter(function(item) { return item.status === 'succeeded'; }).length));
        setText('metric-failed', String(runs.filter(function(item) { return item.status !== 'succeeded'; }).length));
        updatePageTitle();

        const body = document.getElementById('history-body');
        const empty = document.getElementById('history-empty');
        body.innerHTML = '';
        if (!runs.length) {
          empty.style.display = 'block';
          return;
        }
        empty.style.display = 'none';
        runs.forEach(function(item) {
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
        latestRunDetail = detail;
        const run = (detail && detail.run) || {};
        const events = (detail && detail.events) || [];
        updatePageTitle(run);
        setText('run-heading', run.identifier ? run.identifier + ' · ' + tr('run.heading') : tr('run.heading'));
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
        events.forEach(function(item) {
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

      if (languageToggle) languageToggle.addEventListener('click', toggleLanguage);
      applyStaticTranslations();

      (async function init() {
        try {
          if (pageKind === 'run') {
            renderRun(await loadRun());
            return;
          }
          renderHistory(await loadHistory());
          setInterval(async function() {
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
