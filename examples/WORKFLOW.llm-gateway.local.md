---
tracker:
  kind: local
local:
  inbox_dir: $SYMPHONY_LOCAL_INBOX_DIR
  state_dir: $SYMPHONY_LOCAL_STATE_DIR
  archive_dir: $SYMPHONY_LOCAL_ARCHIVE_DIR
  results_dir: $SYMPHONY_LOCAL_RESULTS_DIR
  active_states:
    - To Do
    - In Progress
  terminal_states:
    - Done
    - Reviewed
    - Blocked
orchestrator:
  poll_interval_ms: 30000
  max_concurrent_agents: 2
  max_retry_backoff_ms: 300000
  concurrency_limits:
    default: 1
    review: 1
workspace:
  root: $SYMPHONY_WORKSPACE_ROOT
  seed:
    path: $SYMPHONY_WORKSPACE_BASELINE_DIR
    excludes:
      - .gocache
      - .gotmp
  sync_back:
    path: $SYMPHONY_WORKSPACE_BASELINE_DIR
    on_states:
      - Done
    excludes:
      - .gocache
      - .gotmp
hooks:
  after_create: |
    "$SYMPHONY_CONTROL_ROOT/scripts/llm-gateway-after-create.sh"
  before_run: |
    "$SYMPHONY_CONTROL_ROOT/scripts/review-target-before-run.sh"
  after_run: |
    "$SYMPHONY_CONTROL_ROOT/scripts/git-review-artifacts-after-run.sh"
  timeout_ms: 180000
agent:
  max_turns: 20
codex:
  command: codex app-server
  approval_policy: never
  thread_sandbox: workspace-write
  turn_sandbox_policy:
    type: workspaceWrite
  read_timeout_ms: 5000
  turn_timeout_ms: 3600000
  stall_timeout_ms: 300000
server:
  host: 127.0.0.1
  port: 8080
  username: $SYMPHONY_SERVER_AUTH_USERNAME
  password: $SYMPHONY_SERVER_AUTH_PASSWORD
---
You are working on the `llm-gateway` repository through a local Markdown task.

Identifier: {{ issue.identifier }}
Task ID: {{ issue.id }}
Title: {{ issue.title }}
Current state: {{ issue.state }}
{% if issue.review_of %}
Review target task: {{ issue.review_of }}
Prepared review bundle: .symphony/review-target/README.md
{% endif %}

Body:
{% if issue.description %}
{{ issue.description }}
{% else %}
No description provided.
{% endif %}

Required repository context:
- Read `AGENTS.md` before making non-trivial code, schema, auth, billing, routing, or provider changes.
- Read `docs/llm-gateway-design.md` selectively: start with the sections explicitly listed in the task body, and only read additional sections if they are needed to finish the current slice safely.
- Consult repo-local skills under `.agents/skills/` only when the current slice directly touches those areas.
- If the task touches portal/admin browser login or OA cookie session bootstrap, consult the OA login skill under `.agents/skills/`.

Operating rules:
- Treat the Markdown task body as the delivery slice for this run, not as a request to implement the whole product.
- Start from the task's `Relevant design sections`; do not spend time reading unrelated sections first.
- Work only inside the current workspace and cloned repository.
- The baseline repository may already contain user-owned in-progress changes; do not assume a clean git status and do not try to reset or clean unrelated files.
- Keep changes narrowly scoped to the current task.
- Prefer incremental scaffolding, package boundaries, interfaces, and targeted tests over broad one-shot implementation.
- Prefer the simplest compiling implementation that satisfies the current slice; defer optional framework alignment to later slices if it would significantly slow initial delivery.
- Preserve the compatibility-first, HTTP-first, and control-plane/data-plane separation rules from `AGENTS.md`.
- Do not invent a new public unified inference API by default.
- If the task is still too large after reading it, implement the smallest coherent sub-slice you can complete safely and explain the remaining work in the final summary.
- If the task is explicitly a review or audit slice, do not change production code unless the task body explicitly allows it; inspect, validate, and report instead.
- If `issue.review_of` is set, inspect `.symphony/review-target/README.md` and the copied git bundle before you start reviewing.
- If a review or audit slice finds problems, do not silently fix them in the same task; summarize the defects and the recommended follow-up slice in the final `task_update`.
- Run targeted validation before you stop.
- When you finish an implementation task successfully, call `task_update` on {{ issue.identifier }} with:
  - `state`: `Done`
  - `summary`: a concise handoff covering what changed, what validation ran, and any follow-up notes
- When you finish a review/audit task successfully, prefer `task_update` with:
  - `state`: `Reviewed`
  - `summary`: what you reviewed, what validation ran, the concrete defects or risks you found, and the recommended follow-up slices
- If you are blocked because the task itself cannot be completed, call `task_update` on {{ issue.identifier }} with:
  - `state`: `Blocked`
  - `summary`: what blocked you, what you tried, and the next recommended action
- Do not push, merge, or open a PR unless the task explicitly asks for it and the environment already has working git credentials.
