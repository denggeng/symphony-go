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
      .lang-toggle { min-width: 92px; }
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
          <h1 class="headline" data-i18n="dashboard.headline">symphony-go dashboard</h1>
          <div class="subtle">
            <span data-i18n="dashboard.subtleTagline">Self-hosted task + Codex orchestration runtime.</span>
            <span class="dashboard-view" data-i18n="dashboard.subtleDashboard">Live snapshot of polling, running issues, retries, and backlog.</span>
            <span class="issue-view" data-i18n="dashboard.subtleIssue">Focused view for one running issue.</span>
          </div>
        </div>
        <div class="actions">
          <a class="button secondary" href="/" data-i18n="nav.overview">Overview</a>
          <a class="button secondary" href="/history" data-i18n="nav.history">History</a>
          <button id="language-toggle" class="button secondary lang-toggle" type="button">中文</button>
          <button id="refresh-button" class="button" type="button" data-i18n="nav.refresh">Refresh now</button>
          <div id="action-status" class="status-pill" data-i18n="status.connecting">Connecting live updates…</div>
        </div>
      </header>

      <section id="error-banner" class="error-banner"></section>

      <section class="grid cards">
        <article class="card">
          <h2 data-i18n="card.running.title">Running</h2>
          <div id="metric-running" class="metric">0</div>
          <div class="muted" data-i18n="card.running.desc">Current issue runs</div>
        </article>
        <article class="card">
          <h2 data-i18n="card.retry.title">Retry Queue</h2>
          <div id="metric-retrying" class="metric">0</div>
          <div class="muted" data-i18n="card.retry.desc">Scheduled retries</div>
        </article>
        <article class="card">
          <h2 data-i18n="card.ready.title">Ready Queue</h2>
          <div id="metric-ready" class="metric">0</div>
          <div class="muted" data-i18n="card.ready.desc">Ready but not running</div>
        </article>
        <article class="card">
          <h2 data-i18n="card.blocked.title">Blocked</h2>
          <div id="metric-blocked" class="metric">0</div>
          <div class="muted" data-i18n="card.blocked.desc">Waiting on dependencies</div>
        </article>
        <article class="card">
          <h2 data-i18n="card.tracker.title">Tracker</h2>
          <div id="metric-tracker" class="metric">—</div>
          <div id="metric-project" class="muted" data-i18n="card.tracker.scope">Scope —</div>
        </article>
        <article class="card">
          <h2 data-i18n="card.poll.title">Poll Interval</h2>
          <div id="metric-poll-interval" class="metric">—</div>
          <div id="metric-next-poll" class="muted" data-i18n="card.poll.next">Next poll —</div>
        </article>
      </section>

      <section class="grid panel-grid dashboard-view">
        <article class="panel">
          <h2 data-i18n="runtime.title">Runtime</h2>
          <dl class="kv">
            <dt data-i18n="runtime.service">Service</dt><dd id="runtime-service">—</dd>
            <dt data-i18n="runtime.uptime">Uptime</dt><dd id="runtime-uptime">—</dd>
            <dt data-i18n="runtime.workflow">Workflow</dt><dd id="runtime-workflow">—</dd>
            <dt data-i18n="runtime.promptLength">Prompt Length</dt><dd id="runtime-prompt-length">—</dd>
            <dt data-i18n="runtime.polling">Polling</dt><dd id="runtime-polling">—</dd>
            <dt data-i18n="runtime.lastPoll">Last Poll</dt><dd id="runtime-last-poll">—</dd>
            <dt data-i18n="runtime.nextPoll">Next Poll</dt><dd id="runtime-next-poll">—</dd>
          </dl>
        </article>
        <article class="panel">
          <h2 data-i18n="config.title">Config Summary</h2>
          <dl class="kv">
            <dt data-i18n="config.workspaceRoot">Workspace Root</dt><dd id="config-workspace">—</dd>
            <dt data-i18n="config.trackerQueryInbox">Tracker Query / Inbox</dt><dd id="config-jql">—</dd>
            <dt data-i18n="config.activeStates">Active States</dt><dd id="config-active-states">—</dd>
            <dt data-i18n="config.terminalStates">Terminal States</dt><dd id="config-terminal-states">—</dd>
            <dt data-i18n="config.maxAgents">Max Agents</dt><dd id="config-max-agents">—</dd>
            <dt data-i18n="config.agentMaxTurns">Agent Max Turns</dt><dd id="config-max-turns">—</dd>
            <dt data-i18n="config.codexCommand">Codex Command</dt><dd id="config-codex-command">—</dd>
            <dt data-i18n="config.serverAuth">Server Auth</dt><dd id="config-server-auth">—</dd>
          </dl>
        </article>
      </section>

      <section class="panel dashboard-view">
        <h2 data-i18n="running.title">Running Issues</h2>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th data-i18n="table.issue">Issue</th>
                <th data-i18n="table.state">State</th>
                <th data-i18n="table.turns">Turns</th>
                <th data-i18n="table.runtime">Runtime</th>
                <th data-i18n="table.session">Session</th>
                <th data-i18n="table.lastEvent">Last Event</th>
                <th data-i18n="table.workspace">Workspace</th>
              </tr>
            </thead>
            <tbody id="running-body"></tbody>
          </table>
        </div>
        <div id="running-empty" class="empty" data-i18n="running.empty">No issues are currently running.</div>
      </section>

      <section class="panel dashboard-view">
        <h2 data-i18n="card.retry.title">Retry Queue</h2>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th data-i18n="table.issue">Issue</th>
                <th data-i18n="table.attempt">Attempt</th>
                <th data-i18n="table.dueIn">Due In</th>
                <th data-i18n="table.type">Type</th>
                <th data-i18n="table.error">Error</th>
              </tr>
            </thead>
            <tbody id="retry-body"></tbody>
          </table>
        </div>
        <div id="retry-empty" class="empty" data-i18n="retry.empty">Retry queue is empty.</div>
      </section>

      <section class="panel dashboard-view">
        <h2 data-i18n="backlog.title">Pending Backlog</h2>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th data-i18n="table.issue">Issue</th>
                <th data-i18n="table.queue">Queue</th>
                <th data-i18n="table.state">State</th>
                <th data-i18n="table.priorityOrder">Priority / Order</th>
                <th data-i18n="table.dependencies">Dependencies</th>
                <th data-i18n="table.updated">Updated</th>
              </tr>
            </thead>
            <tbody id="backlog-body"></tbody>
          </table>
        </div>
        <div id="backlog-empty" class="empty" data-i18n="backlog.empty">No pending backlog items are currently tracked.</div>
      </section>

      <section class="panel dashboard-view">
        <h2 data-i18n="history.title">Recent Runs</h2>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th data-i18n="table.run">Run</th>
                <th data-i18n="table.status">Status</th>
                <th data-i18n="table.finished">Finished</th>
                <th data-i18n="table.runtime">Runtime</th>
                <th data-i18n="table.turns">Turns</th>
                <th data-i18n="table.events">Recorded Events</th>
                <th data-i18n="table.workspace">Workspace</th>
              </tr>
            </thead>
            <tbody id="history-body"></tbody>
          </table>
        </div>
        <div id="history-empty" class="empty" data-i18n="history.empty">No completed runs recorded yet.</div>
      </section>

      <section class="panel issue-view">
        <h2 class="issue-title" id="issue-heading" data-i18n="issue.heading">Issue</h2>
        <div class="issue-meta">
          <span id="issue-state" class="tag ok" data-i18n="issue.unknown">Unknown</span>
          <span id="issue-queue-status" class="tag warn" data-i18n="issue.queueUnknown">Queue: unknown</span>
          <span id="issue-turns" class="tag warn" data-i18n="issue.turnsPlaceholder">Turns: —</span>
          <span id="issue-runtime" class="tag warn" data-i18n="issue.runtimePlaceholder">Runtime: —</span>
        </div>
        <div id="issue-missing" class="empty" style="display:none;" data-i18n="issue.missing">This issue is not visible in the current running, retry, or backlog snapshots.</div>
        <div id="issue-content">
          <dl class="kv">
            <dt data-i18n="issue.titleLabel">Title</dt><dd id="issue-title">—</dd>
            <dt data-i18n="issue.issueID">Issue ID</dt><dd id="issue-id">—</dd>
            <dt data-i18n="issue.queueDetail">Queue Detail</dt><dd id="issue-queue-detail">—</dd>
            <dt data-i18n="issue.sessionID">Session ID</dt><dd id="issue-session">—</dd>
            <dt data-i18n="issue.codexPID">Codex PID</dt><dd id="issue-pid">—</dd>
            <dt data-i18n="issue.retryAttempt">Retry Attempt</dt><dd id="issue-retry">—</dd>
            <dt data-i18n="issue.retryDue">Retry Due</dt><dd id="issue-retry-due">—</dd>
            <dt data-i18n="issue.priorityOrder">Priority / Order</dt><dd id="issue-priority-order">—</dd>
            <dt data-i18n="issue.dependencies">Dependencies</dt><dd id="issue-dependencies">—</dd>
            <dt data-i18n="issue.blockedBy">Blocked By</dt><dd id="issue-blocked-by">—</dd>
            <dt data-i18n="issue.lastEvent">Last Event</dt><dd id="issue-last-event">—</dd>
            <dt data-i18n="issue.lastMessage">Last Message</dt><dd id="issue-last-message">—</dd>
            <dt data-i18n="issue.lastTimestamp">Last Timestamp</dt><dd id="issue-last-timestamp">—</dd>
            <dt data-i18n="issue.updatedAt">Updated At</dt><dd id="issue-updated-at">—</dd>
            <dt>Workspace</dt><dd class="mono" id="issue-workspace">—</dd>
            <dt data-i18n="issue.usage">Usage</dt><dd id="issue-usage">—</dd>
          </dl>
        </div>
      </section>

      <section class="panel issue-view">
        <h2 data-i18n="issue.historyTitle">Recent Runs for Issue</h2>
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th data-i18n="table.run">Run</th>
                <th data-i18n="table.status">Status</th>
                <th data-i18n="table.finished">Finished</th>
                <th data-i18n="table.runtime">Runtime</th>
                <th data-i18n="table.turns">Turns</th>
                <th data-i18n="table.result">Result</th>
              </tr>
            </thead>
            <tbody id="issue-history-body"></tbody>
          </table>
        </div>
        <div id="issue-history-empty" class="empty" data-i18n="issue.historyEmpty">No recent runs recorded for this issue.</div>
      </section>

      <div class="footer" data-i18n-html="footer.html">
        API: <a href="/api/v1/state">/api/v1/state</a> ·
        history: <a href="/history">/history</a> ·
        live stream: <a href="/events">/events</a> ·
        health: <a href="/healthz">/healthz</a>
      </div>
    </div>

    <script>
      const translations = {
        'page.title.dashboard': { en: 'symphony-go dashboard', zh: 'symphony-go 控制台' },
        'page.title.issue': { en: 'Issue · symphony-go', zh: '任务 · symphony-go' },
        'page.title.issueSuffix': { en: 'issue', zh: '任务' },
        'dashboard.headline': { en: 'symphony-go dashboard', zh: 'symphony-go 控制台' },
        'dashboard.subtleTagline': { en: 'Self-hosted task + Codex orchestration runtime.', zh: '自托管任务与 Codex 编排运行时。' },
        'dashboard.subtleDashboard': { en: 'Live snapshot of polling, running issues, retries, and backlog.', zh: '实时查看轮询、运行中任务、重试队列和待处理积压。' },
        'dashboard.subtleIssue': { en: 'Focused view for one running issue.', zh: '聚焦查看单个任务的运行状态。' },
        'nav.overview': { en: 'Overview', zh: '总览' },
        'nav.history': { en: 'History', zh: '历史' },
        'nav.refresh': { en: 'Refresh now', zh: '立即刷新' },
        'status.connecting': { en: 'Connecting live updates…', zh: '正在连接实时更新…' },
        'card.running.title': { en: 'Running', zh: '运行中' },
        'card.running.desc': { en: 'Current issue runs', zh: '当前正在执行的任务' },
        'card.retry.title': { en: 'Retry Queue', zh: '重试队列' },
        'card.retry.desc': { en: 'Scheduled retries', zh: '已安排的重试' },
        'card.ready.title': { en: 'Ready Queue', zh: '就绪队列' },
        'card.ready.desc': { en: 'Ready but not running', zh: '已就绪但尚未运行' },
        'card.blocked.title': { en: 'Blocked', zh: '阻塞中' },
        'card.blocked.desc': { en: 'Waiting on dependencies', zh: '等待依赖完成' },
        'card.tracker.title': { en: 'Tracker', zh: '跟踪器' },
        'card.tracker.scope': { en: 'Scope —', zh: '范围 —' },
        'card.poll.title': { en: 'Poll Interval', zh: '轮询间隔' },
        'card.poll.next': { en: 'Next poll —', zh: '下次轮询 —' },
        'runtime.title': { en: 'Runtime', zh: '运行时' },
        'runtime.service': { en: 'Service', zh: '服务' },
        'runtime.uptime': { en: 'Uptime', zh: '运行时长' },
        'runtime.workflow': { en: 'Workflow', zh: '工作流' },
        'runtime.promptLength': { en: 'Prompt Length', zh: '提示长度' },
        'runtime.polling': { en: 'Polling', zh: '轮询状态' },
        'runtime.lastPoll': { en: 'Last Poll', zh: '上次轮询' },
        'runtime.nextPoll': { en: 'Next Poll', zh: '下次轮询' },
        'config.title': { en: 'Config Summary', zh: '配置摘要' },
        'config.workspaceRoot': { en: 'Workspace Root', zh: '工作区根目录' },
        'config.trackerQueryInbox': { en: 'Tracker Query / Inbox', zh: '跟踪查询 / 收件箱' },
        'config.activeStates': { en: 'Active States', zh: '活跃状态' },
        'config.terminalStates': { en: 'Terminal States', zh: '终态' },
        'config.maxAgents': { en: 'Max Agents', zh: '最大 Agent 数' },
        'config.agentMaxTurns': { en: 'Agent Max Turns', zh: 'Agent 最大轮次' },
        'config.codexCommand': { en: 'Codex Command', zh: 'Codex 命令' },
        'config.serverAuth': { en: 'Server Auth', zh: '服务鉴权' },
        'running.title': { en: 'Running Issues', zh: '运行中的任务' },
        'table.issue': { en: 'Issue', zh: '任务' },
        'table.state': { en: 'State', zh: '状态' },
        'table.turns': { en: 'Turns', zh: '轮次' },
        'table.runtime': { en: 'Runtime', zh: '运行时长' },
        'table.session': { en: 'Session', zh: '会话' },
        'table.lastEvent': { en: 'Last Event', zh: '最近事件' },
        'table.workspace': { en: 'Workspace', zh: '工作区' },
        'running.empty': { en: 'No issues are currently running.', zh: '当前没有任务在运行。' },
        'retry.sectionTitle': { en: 'Retry Queue', zh: '重试队列' },
        'table.attempt': { en: 'Attempt', zh: '尝试次数' },
        'table.dueIn': { en: 'Due In', zh: '距触发还有' },
        'table.type': { en: 'Type', zh: '类型' },
        'table.error': { en: 'Error', zh: '错误' },
        'retry.empty': { en: 'Retry queue is empty.', zh: '重试队列为空。' },
        'backlog.title': { en: 'Pending Backlog', zh: '待处理积压' },
        'table.queue': { en: 'Queue', zh: '队列' },
        'table.priorityOrder': { en: 'Priority / Order', zh: '优先级 / 顺序' },
        'table.dependencies': { en: 'Dependencies', zh: '依赖' },
        'table.updated': { en: 'Updated', zh: '更新时间' },
        'backlog.empty': { en: 'No pending backlog items are currently tracked.', zh: '当前没有待处理的积压任务。' },
        'history.title': { en: 'Recent Runs', zh: '最近运行' },
        'table.run': { en: 'Run', zh: '运行' },
        'table.status': { en: 'Status', zh: '状态' },
        'table.finished': { en: 'Finished', zh: '完成时间' },
        'table.events': { en: 'Recorded Events', zh: '已记录事件' },
        'history.empty': { en: 'No completed runs recorded yet.', zh: '还没有已完成的运行记录。' },
        'issue.heading': { en: 'Issue', zh: '任务' },
        'issue.unknown': { en: 'Unknown', zh: '未知' },
        'issue.queueUnknown': { en: 'Queue: unknown', zh: '队列：未知' },
        'issue.turnsPlaceholder': { en: 'Turns: —', zh: '轮次：—' },
        'issue.runtimePlaceholder': { en: 'Runtime: —', zh: '运行时长：—' },
        'issue.missing': { en: 'This issue is not visible in the current running, retry, or backlog snapshots.', zh: '当前运行、重试或积压快照中都看不到这个任务。' },
        'issue.titleLabel': { en: 'Title', zh: '标题' },
        'issue.issueID': { en: 'Issue ID', zh: '任务 ID' },
        'issue.queueDetail': { en: 'Queue Detail', zh: '队列详情' },
        'issue.sessionID': { en: 'Session ID', zh: '会话 ID' },
        'issue.codexPID': { en: 'Codex PID', zh: 'Codex PID' },
        'issue.retryAttempt': { en: 'Retry Attempt', zh: '重试次数' },
        'issue.retryDue': { en: 'Retry Due', zh: '重试时间' },
        'issue.priorityOrder': { en: 'Priority / Order', zh: '优先级 / 顺序' },
        'issue.dependencies': { en: 'Dependencies', zh: '依赖' },
        'issue.blockedBy': { en: 'Blocked By', zh: '阻塞来源' },
        'issue.lastEvent': { en: 'Last Event', zh: '最近事件' },
        'issue.lastMessage': { en: 'Last Message', zh: '最近消息' },
        'issue.lastTimestamp': { en: 'Last Timestamp', zh: '最近时间戳' },
        'issue.updatedAt': { en: 'Updated At', zh: '更新时间' },
        'issue.workspace': { en: 'Workspace', zh: '工作区' },
        'issue.usage': { en: 'Usage', zh: '用量' },
        'issue.historyTitle': { en: 'Recent Runs for Issue', zh: '该任务的最近运行' },
        'table.result': { en: 'Result', zh: '结果' },
        'issue.historyEmpty': { en: 'No recent runs recorded for this issue.', zh: '这个任务还没有最近运行记录。' },
        'footer.html': { en: 'API: <a href="/api/v1/state">/api/v1/state</a> · history: <a href="/history">/history</a> · live stream: <a href="/events">/events</a> · health: <a href="/healthz">/healthz</a>', zh: 'API：<a href="/api/v1/state">/api/v1/state</a> · 历史：<a href="/history">/history</a> · 实时流：<a href="/events">/events</a> · 健康检查：<a href="/healthz">/healthz</a>' },
        'dynamic.localTasks': { en: 'Local tasks', zh: '本地任务' },
        'dynamic.projectPrefix': { en: 'Project ', zh: '项目 ' },
        'dynamic.projectUnknown': { en: 'Project —', zh: '项目 —' },
        'dynamic.nextPollPrefix': { en: 'Next poll ', zh: '下次轮询 ' },
        'dynamic.chars': { en: 'chars', zh: '字符' },
        'dynamic.checkingNow': { en: 'Checking now…', zh: '正在检查…' },
        'dynamic.idle': { en: 'Idle', zh: '空闲' },
        'dynamic.serverAuthEnabled': { en: 'Enabled (Basic auth)', zh: '已启用（Basic 认证）' },
        'dynamic.serverAuthDisabled': { en: 'Disabled', zh: '未启用' },
        'dynamic.continuation': { en: 'Continuation', zh: '续跑' },
        'dynamic.failure': { en: 'Failure', zh: '失败' },
        'dynamic.waitingOn': { en: 'Waiting on ', zh: '等待依赖：' },
        'dynamic.dependenciesSatisfied': { en: 'Dependencies satisfied', zh: '依赖已满足' },
        'dynamic.noDependencies': { en: 'No dependencies', zh: '无依赖' },
        'dynamic.issueMissingHistory': { en: 'This issue is not currently active. Recent runs are shown below.', zh: '这个任务当前未激活，下面展示的是最近运行记录。' },
        'dynamic.inactive': { en: 'Inactive', zh: '未激活' },
        'dynamic.notVisible': { en: 'Not visible', zh: '不可见' },
        'dynamic.queueHistoryOnly': { en: 'Queue: history only', zh: '队列：仅历史记录' },
        'dynamic.turnsPrefix': { en: 'Turns: ', zh: '轮次：' },
        'dynamic.runtimePrefix': { en: 'Runtime: ', zh: '运行时长：' },
        'dynamic.queuePrefix': { en: 'Queue: ', zh: '队列：' },
        'dynamic.unknownLower': { en: 'unknown', zh: '未知' },
        'dynamic.readyForDispatch': { en: 'Ready for dispatch', zh: '可调度' },
        'dynamic.continuationRetryScheduled': { en: 'Continuation retry scheduled', zh: '已安排续跑重试' },
        'dynamic.failureRetryScheduled': { en: 'Failure retry scheduled', zh: '已安排失败重试' },
        'dynamic.currentlyExecuting': { en: 'Currently executing', zh: '正在执行' },
        'dynamic.requestingRefresh': { en: 'Requesting refresh…', zh: '正在请求刷新…' },
        'dynamic.refreshAlreadyQueued': { en: 'Refresh already queued', zh: '刷新已在队列中' },
        'dynamic.refreshRequested': { en: 'Refresh requested', zh: '已请求刷新' },
        'dynamic.refreshFailed': { en: 'Refresh failed', zh: '刷新失败' },
        'dynamic.liveConnected': { en: 'Live updates connected', zh: '实时更新已连接' },
        'dynamic.initialLoadFailed': { en: 'Initial load failed', zh: '初次加载失败' },
        'dynamic.liveParseFailed': { en: 'Live update parse failed', zh: '实时更新解析失败' },
        'dynamic.liveDisconnected': { en: 'Live updates disconnected', zh: '实时更新已断开' },
        'error.loadSnapshot': { en: 'Failed to load snapshot', zh: '加载运行快照失败' },
        'error.loadHistory': { en: 'Failed to load history', zh: '加载历史记录失败' },
        'unit.ms': { en: 'ms', zh: '毫秒' },
        'unit.s': { en: 's', zh: '秒' },
        'usage.input': { en: 'input', zh: '输入' },
        'usage.output': { en: 'output', zh: '输出' },
        'usage.total': { en: 'total', zh: '总计' },
        'usage.cached': { en: 'cached', zh: '缓存' },
        'usage.reasoning': { en: 'reasoning', zh: '推理' }
      };

      const pageKind = document.body.dataset.pageKind || 'dashboard';
      const issueIdentifier = document.body.dataset.issueIdentifier || '';
      const refreshButton = document.getElementById('refresh-button');
      const languageToggle = document.getElementById('language-toggle');
      const actionStatus = document.getElementById('action-status');
      const errorBanner = document.getElementById('error-banner');
      let issueHistoryRuns = [];
      let latestSnapshot = null;
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

      function updatePageTitle(issueID) {
        if (pageKind === 'issue') {
          const label = issueID || issueIdentifier;
          document.title = label ? label + ' · ' + tr('page.title.issueSuffix') : tr('page.title.issue');
          return;
        }
        document.title = tr('page.title.dashboard');
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
        updatePageTitle();
      }

      function toggleLanguage() {
        currentLanguage = currentLanguage === 'zh' ? 'en' : 'zh';
        saveLanguage();
        applyStaticTranslations();
        if (latestSnapshot) renderDashboard(latestSnapshot);
      }

      function formatTime(value) {
        if (!value) return '—';
        const parsed = new Date(value);
        if (Number.isNaN(parsed.getTime())) return value;
        return parsed.toLocaleString(currentLanguage === 'zh' ? 'zh-CN' : 'en-US');
      }

      function formatMs(value) {
        if (value === null || value === undefined) return '—';
        if (value < 1000) return value + ' ' + tr('unit.ms');
        const seconds = Math.round((value / 1000) * 10) / 10;
        return seconds + ' ' + tr('unit.s');
      }

      function formatSeconds(value) {
        if (value === null || value === undefined) return '—';
        return String(value) + tr('unit.s');
      }

      function formatList(values) {
        if (!Array.isArray(values) || !values.length) return '—';
        return values.join(', ');
      }

      function formatPriorityOrder(item) {
        const priority = item && item.priority !== null && item.priority !== undefined ? item.priority : '—';
        const order = item && item.order !== null && item.order !== undefined ? item.order : '—';
        return 'P' + priority + ' / O' + order;
      }

      function formatUsage(usage) {
        if (!usage) return '—';
        const value = usage || {};
        let usageText = tr('usage.input') + '=' + (value.input_tokens || 0) + ', ' + tr('usage.output') + '=' + (value.output_tokens || 0) + ', ' + tr('usage.total') + '=' + (value.total_tokens || 0);
        if (value.cached_input_tokens) usageText += ', ' + tr('usage.cached') + '=' + value.cached_input_tokens;
        if (value.reasoning_output_tokens) usageText += ', ' + tr('usage.reasoning') + '=' + value.reasoning_output_tokens;
        return usageText;
      }

      function statusClass(text) {
        const value = String(text || '').toLowerCase();
        if (value.includes('fail') || value.includes('error') || value.includes('cancel') || value.includes('block')) return 'err';
        if (value.includes('wait') || value.includes('retry') || value.includes('progress') || value.includes('continue') || value.includes('stop')) return 'warn';
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
        latestSnapshot = snapshot;
        const running = snapshot.running || [];
        const retrying = snapshot.retrying || [];
        const backlog = snapshot.backlog || [];
        const history = snapshot.history || [];
        const config = snapshot.config || {};
        const polling = snapshot.polling || {};
        const service = snapshot.service || {};
        const workflow = snapshot.workflow || {};

        setText('metric-running', String(running.length));
        setText('metric-retrying', String(retrying.length));
        const readyCount = backlog.filter(function(item) { return item.queue_status === 'ready'; }).length;
        const blockedCount = backlog.filter(function(item) { return item.queue_status === 'blocked'; }).length;
        setText('metric-ready', String(readyCount));
        setText('metric-blocked', String(blockedCount));
        const trackerKind = (config.tracker && config.tracker.kind) || '';
        const localConfig = config.local || {};

        setText('metric-tracker', trackerKind || '—');
        setText('metric-project', trackerKind === 'local' ? tr('dynamic.localTasks') : ((config.tracker && config.tracker.project_key) ? tr('dynamic.projectPrefix') + config.tracker.project_key : tr('dynamic.projectUnknown')));
        setText('metric-poll-interval', formatMs(polling.poll_interval_ms));
        setText('metric-next-poll', tr('dynamic.nextPollPrefix') + formatTime(polling.next_poll_at));

        setText('runtime-service', (service.name || 'symphony-go') + ' (' + (service.version || 'dev') + ')');
        setText('runtime-uptime', service.uptime || '—');
        setText('runtime-workflow', workflow.path || '—');
        setText('runtime-prompt-length', workflow.prompt_length != null ? workflow.prompt_length + ' ' + tr('dynamic.chars') : '—');
        setText('runtime-polling', polling.checking ? tr('dynamic.checkingNow') : tr('dynamic.idle'));
        setText('runtime-last-poll', formatTime(polling.last_poll_at));
        setText('runtime-next-poll', formatTime(polling.next_poll_at));

        setText('config-workspace', config.workspace ? config.workspace.root : '—');
        setText('config-jql', trackerKind === 'local' ? (localConfig.inbox_dir || '—') : (config.tracker ? (config.tracker.jql || '—') : '—'));
        setText('config-active-states', config.tracker ? (config.tracker.active_states || []).join(', ') : '—');
        setText('config-terminal-states', config.tracker ? (config.tracker.terminal_states || []).join(', ') : '—');
        setText('config-max-agents', config.orchestrator ? String(config.orchestrator.max_concurrent_agents ?? '—') : '—');
        setText('config-max-turns', config.agent ? String(config.agent.max_turns ?? '—') : '—');
        setText('config-codex-command', config.codex ? (config.codex.command || '—') : '—');
        setText('config-server-auth', config.server && config.server.auth_enabled ? tr('dynamic.serverAuthEnabled') : tr('dynamic.serverAuthDisabled'));

        showError(polling.last_error || '');
        updatePageTitle();

        const runningBody = document.getElementById('running-body');
        const runningEmpty = document.getElementById('running-empty');
        runningBody.innerHTML = '';
        if (!running.length) {
          runningEmpty.style.display = 'block';
        } else {
          runningEmpty.style.display = 'none';
          running.forEach(function(item) {
            const row = document.createElement('tr');
            const issueLink = '/issues/' + encodeURIComponent(item.identifier);
            row.innerHTML = '' +
              '<td><a href="' + issueLink + '">' + escapeHTML(item.identifier) + '</a><div class="muted mono">' + escapeHTML(item.issue_id) + '</div></td>' +
              '<td><span class="tag ' + statusClass(item.state) + '">' + escapeHTML(item.state || tr('issue.unknown')) + '</span></td>' +
              '<td>' + escapeHTML(item.turns) + '</td>' +
              '<td>' + escapeHTML(formatSeconds(item.runtime_seconds)) + '</td>' +
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
          retrying.forEach(function(item) {
            const row = document.createElement('tr');
            row.innerHTML = '' +
              '<td>' + escapeHTML(item.identifier) + '<div class="muted mono">' + escapeHTML(item.issue_id) + '</div></td>' +
              '<td>' + escapeHTML(item.attempt) + '</td>' +
              '<td>' + escapeHTML(formatMs(item.due_in_ms)) + '</td>' +
              '<td><span class="tag ' + (item.continuation ? 'warn' : 'err') + '">' + escapeHTML(item.continuation ? tr('dynamic.continuation') : tr('dynamic.failure')) + '</span></td>' +
              '<td>' + escapeHTML(item.error || '—') + '</td>';
            retryBody.appendChild(row);
          });
        }

        const backlogBody = document.getElementById('backlog-body');
        const backlogEmpty = document.getElementById('backlog-empty');
        backlogBody.innerHTML = '';
        if (!backlog.length) {
          backlogEmpty.style.display = 'block';
        } else {
          backlogEmpty.style.display = 'none';
          backlog.forEach(function(item) {
            const row = document.createElement('tr');
            const dependencySummary = formatList(item.dependencies);
            const dependencyDetail = item.blocked_by && item.blocked_by.length
              ? tr('dynamic.waitingOn') + item.blocked_by.join(', ')
              : (item.dependencies && item.dependencies.length ? tr('dynamic.dependenciesSatisfied') : tr('dynamic.noDependencies'));
            row.innerHTML = '' +
              '<td>' + escapeHTML(item.identifier) + '<div class="muted">' + escapeHTML(item.title || '') + '</div></td>' +
              '<td><span class="tag ' + statusClass(item.queue_status) + '">' + escapeHTML(item.queue_status || 'ready') + '</span></td>' +
              '<td><span class="tag ' + statusClass(item.state) + '">' + escapeHTML(item.state || tr('issue.unknown')) + '</span></td>' +
              '<td>' + escapeHTML(formatPriorityOrder(item)) + '</td>' +
              '<td>' + escapeHTML(dependencySummary) + '<div class="muted">' + escapeHTML(dependencyDetail) + '</div></td>' +
              '<td>' + escapeHTML(formatTime(item.updated_at)) + '</td>';
            backlogBody.appendChild(row);
          });
        }

        const historyBody = document.getElementById('history-body');
        const historyEmpty = document.getElementById('history-empty');
        historyBody.innerHTML = '';
        if (!history.length) {
          historyEmpty.style.display = 'block';
        } else {
          historyEmpty.style.display = 'none';
          history.forEach(function(item) {
            const row = document.createElement('tr');
            const runLink = '/history/' + encodeURIComponent(item.run_id);
            row.innerHTML = '' +
              '<td><a href="' + runLink + '">' + escapeHTML(item.run_id) + '</a><div class="muted">' + escapeHTML(item.identifier || '—') + '</div></td>' +
              '<td><span class="tag ' + statusClass(item.status) + '">' + escapeHTML(item.status || '—') + '</span></td>' +
              '<td>' + escapeHTML(formatTime(item.finished_at)) + '</td>' +
              '<td>' + escapeHTML(formatSeconds(item.runtime_seconds)) + '</td>' +
              '<td>' + escapeHTML(item.turns) + '</td>' +
              '<td>' + escapeHTML(item.event_count) + '</td>' +
              '<td><code>' + escapeHTML(item.workspace_path || '—') + '</code></td>';
            historyBody.appendChild(row);
          });
        }

        const selectedHistory = issueHistoryRuns.length ? issueHistoryRuns : history;
        renderIssue(snapshot, selectedHistory);
      }

      function renderIssue(snapshot, historyRuns) {
        if (pageKind !== 'issue') return;
        const running = snapshot.running || [];
        const retrying = snapshot.retrying || [];
        const backlog = snapshot.backlog || [];
        const matchedHistory = (historyRuns || []).filter(function(item) { return item.identifier === issueIdentifier || item.issue_id === issueIdentifier; });
        let issue = running.find(function(item) { return item.identifier === issueIdentifier || item.issue_id === issueIdentifier; });
        if (!issue) issue = retrying.find(function(item) { return item.identifier === issueIdentifier || item.issue_id === issueIdentifier; });
        if (!issue) issue = backlog.find(function(item) { return item.identifier === issueIdentifier || item.issue_id === issueIdentifier; });
        setText('issue-heading', issueIdentifier || tr('issue.heading'));
        const missing = document.getElementById('issue-missing');
        const content = document.getElementById('issue-content');
        if (!issue) {
          missing.style.display = 'block';
          missing.textContent = matchedHistory.length ? tr('dynamic.issueMissingHistory') : tr('issue.missing');
          content.style.display = 'none';
          document.getElementById('issue-state').className = 'tag err';
          document.getElementById('issue-queue-status').className = 'tag err';
          setText('issue-state', matchedHistory.length ? tr('dynamic.inactive') : tr('dynamic.notVisible'));
          setText('issue-queue-status', matchedHistory.length ? tr('dynamic.queueHistoryOnly') : tr('issue.queueUnknown'));
          setText('issue-turns', tr('dynamic.turnsPrefix') + '—');
          setText('issue-runtime', tr('dynamic.runtimePrefix') + '—');
          updatePageTitle(issueIdentifier);
          renderIssueHistory(matchedHistory);
          return;
        }
        missing.style.display = 'none';
        content.style.display = 'block';
        const stateNode = document.getElementById('issue-state');
        const queueNode = document.getElementById('issue-queue-status');
        stateNode.className = 'tag ' + statusClass(issue.state || issue.queue_status);
        queueNode.className = 'tag ' + statusClass(issue.queue_status || issue.state);
        setText('issue-state', issue.state || tr('issue.unknown'));
        setText('issue-queue-status', tr('dynamic.queuePrefix') + (issue.queue_status || tr('dynamic.unknownLower')));
        setText('issue-turns', tr('dynamic.turnsPrefix') + (issue.turns !== undefined && issue.turns !== null ? issue.turns : '—'));
        setText('issue-runtime', tr('dynamic.runtimePrefix') + (issue.runtime_seconds !== undefined && issue.runtime_seconds !== null ? formatSeconds(issue.runtime_seconds) : '—'));
        setText('issue-heading', issue.identifier || issueIdentifier);
        setText('issue-title', issue.title || '—');
        setText('issue-id', issue.issue_id || '—');
        setText('issue-session', issue.session_id || '—');
        setText('issue-pid', issue.codex_app_server_pid || '—');
        setText('issue-retry', String(issue.retry_attempt ?? issue.attempt ?? 0));
        setText('issue-retry-due', issue.due_at ? (formatTime(issue.due_at) + ' (' + formatMs(issue.due_in_ms) + ')') : '—');
        setText('issue-priority-order', formatPriorityOrder(issue));
        setText('issue-dependencies', formatList(issue.dependencies));
        setText('issue-blocked-by', formatList(issue.blocked_by));
        const queueDetail = issue.blocked_by && issue.blocked_by.length
          ? tr('dynamic.waitingOn') + issue.blocked_by.join(', ')
          : (issue.queue_status === 'ready'
            ? tr('dynamic.readyForDispatch')
            : (issue.queue_status === 'retrying'
              ? (issue.continuation ? tr('dynamic.continuationRetryScheduled') : tr('dynamic.failureRetryScheduled'))
              : (issue.queue_status === 'running' ? tr('dynamic.currentlyExecuting') : '—')));
        setText('issue-queue-detail', queueDetail);
        setText('issue-last-event', issue.last_event || '—');
        setText('issue-last-message', issue.last_message || issue.error || '—');
        setText('issue-last-timestamp', formatTime(issue.last_timestamp));
        setText('issue-updated-at', formatTime(issue.updated_at));
        setText('issue-workspace', issue.workspace_path || '—');
        setText('issue-usage', formatUsage(issue.usage));
        updatePageTitle(issue.identifier || issueIdentifier);
        renderIssueHistory(matchedHistory);
      }

      function renderIssueHistory(historyRuns) {
        if (pageKind !== 'issue') return;
        const body = document.getElementById('issue-history-body');
        const empty = document.getElementById('issue-history-empty');
        body.innerHTML = '';
        if (!historyRuns || !historyRuns.length) {
          empty.style.display = 'block';
          return;
        }
        empty.style.display = 'none';
        historyRuns.slice(0, 10).forEach(function(item) {
          const row = document.createElement('tr');
          const runLink = '/history/' + encodeURIComponent(item.run_id);
          row.innerHTML = '' +
            '<td><a href="' + runLink + '">' + escapeHTML(item.run_id) + '</a></td>' +
            '<td><span class="tag ' + statusClass(item.status) + '">' + escapeHTML(item.status || '—') + '</span></td>' +
            '<td>' + escapeHTML(formatTime(item.finished_at)) + '</td>' +
            '<td>' + escapeHTML(formatSeconds(item.runtime_seconds)) + '</td>' +
            '<td>' + escapeHTML(item.turns) + '</td>' +
            '<td>' + escapeHTML(item.last_message || item.error || '—') + '</td>';
          body.appendChild(row);
        });
      }

      async function loadSnapshot() {
        const response = await fetch('/api/v1/state');
        if (!response.ok) throw new Error(tr('error.loadSnapshot'));
        return response.json();
      }

      async function loadHistory() {
        const response = await fetch('/api/v1/history');
        if (!response.ok) throw new Error(tr('error.loadHistory'));
        const payload = await response.json();
        issueHistoryRuns = payload.runs || [];
        return issueHistoryRuns;
      }

      async function refreshNow() {
        refreshButton.disabled = true;
        actionStatus.textContent = tr('dynamic.requestingRefresh');
        try {
          const response = await fetch('/api/v1/refresh', { method: 'POST' });
          const payload = await response.json();
          actionStatus.textContent = payload.coalesced ? tr('dynamic.refreshAlreadyQueued') : tr('dynamic.refreshRequested');
        } catch (_error) {
          actionStatus.textContent = tr('dynamic.refreshFailed');
        } finally {
          refreshButton.disabled = false;
        }
      }

      if (refreshButton) refreshButton.addEventListener('click', refreshNow);
      if (languageToggle) languageToggle.addEventListener('click', toggleLanguage);

      applyStaticTranslations();

      (async function init() {
        try {
          if (pageKind === 'issue') {
            const snapshotHistory = await Promise.all([
              loadSnapshot(),
              loadHistory().catch(function() { return []; })
            ]);
            renderDashboard(snapshotHistory[0]);
          } else {
            renderDashboard(await loadSnapshot());
          }
          actionStatus.textContent = tr('dynamic.liveConnected');
        } catch (error) {
          actionStatus.textContent = tr('dynamic.initialLoadFailed');
          showError(String(error));
        }

        const events = new EventSource('/events');
        events.onmessage = function(event) {
          try {
            renderDashboard(JSON.parse(event.data));
            actionStatus.textContent = tr('dynamic.liveConnected');
          } catch (_error) {
            actionStatus.textContent = tr('dynamic.liveParseFailed');
          }
        };
        events.onerror = function() {
          actionStatus.textContent = tr('dynamic.liveDisconnected');
        };

        setInterval(async function() {
          if (document.hidden) return;
          try {
            if (pageKind === 'issue') {
              const snapshotHistory = await Promise.all([
                loadSnapshot(),
                loadHistory().catch(function() { return issueHistoryRuns; })
              ]);
              renderDashboard(snapshotHistory[0]);
            } else {
              renderDashboard(await loadSnapshot());
            }
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
	if !server.requireAuth(writer, request) {
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
	if !server.requireAuth(writer, request) {
		return
	}
	server.renderPage(writer, http.StatusOK, pageData{Title: "Issue · symphony-go", PageKind: "issue", IssueIdentifier: identifier})
}

func (server *Server) handleEvents(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		server.writeError(writer, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !server.requireAuth(writer, request) {
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
