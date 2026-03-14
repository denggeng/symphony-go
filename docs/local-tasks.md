# Local Markdown tasks

This is the simplest way to run `symphony-go` in a full local closed loop.

Instead of creating a Jira issue, you drop a Markdown file into an inbox directory. `symphony-go` turns that file into a task, creates a workspace, runs Codex, and waits for Codex to finish by calling `task_update`.

## What is supported

In local mode today:

- `tracker.kind: local`
- polling a task inbox directory for `*.md` files
- optional front matter fields: `id`, `title`, `state`, `priority`, `order`, `depends_on`
- result state persisted in sidecar JSON files
- terminal tasks moved from inbox to archive
- result artifacts written under `local.results_dir/<task-id>/`
- final writeback through the `task_update` tool

## Task file format

A local task is a Markdown file.

Minimal example:

```md
---
id: hello-endpoint
title: Add hello endpoint
state: To Do
---
Goal
Add a `/hello` endpoint that returns `{ "message": "hello" }`.

Validation
- go test ./...
```

Rules:

- if `id` is missing, the filename stem becomes the task id
- the Markdown body becomes the task description shown to Codex
- `state` must map to one of your configured active or terminal states

## Dispatch order and dependencies

Optional scheduling fields let you control local queue order without relying on file timestamps:

- `priority`: lower numbers run first
- `order`: lower numbers run first within the same priority
- `depends_on`: a YAML list or comma-separated string of task ids that must finish first
- a dependency only becomes ready after it reaches a successful terminal state such as `Done` or `Reviewed`
- `Blocked`, `Failed`, `Cancelled`, missing, or still-active dependencies keep the task pending
- when `priority` or `order` is omitted, sorting falls back to file modification time and task id

Example:

```md
---
id: api-routing
title: Build routing layer
state: To Do
priority: 1
order: 20
depends_on:
  - schema
  - auth-bootstrap
---
Implement the API routing after schema and auth bootstrap are complete.
```

## Implementation + review pattern

A practical closed loop for higher-risk work is to split implementation and review into separate Markdown tasks:

- create the implementation task first
- create a second review task with `depends_on` pointing at the implementation task id
- make the review task body explicit that it is review-only and should not change production code
- if review finds issues but the review itself completed, have it finish with `task_update(state: Reviewed, ...)` and list the recommended follow-up slices
- reserve `Blocked` for cases where the review task itself cannot proceed

That review task runs in a fresh Codex session, so it acts like a second pass instead of continuing the original agent thread.

## Directory model

By default, local mode uses these directories:

- `./local_tasks/inbox`
- `./local_tasks/state`
- `./local_tasks/archive`
- `./local_tasks/results`

Meaning:

- `inbox` holds active task files waiting to run
- `state` stores sidecar JSON state per task
- `archive` stores completed or blocked task files
- `results` stores human-readable output and metadata

## Minimal setup

### 1. Prepare env

```bash
cp .env.example .env
```

Set at least:

```dotenv
SYMPHONY_WORKSPACE_ROOT=/absolute/path/to/symphony-workspaces
SOURCE_REPO_URL=git@github.com:your-org/your-repo.git
SOURCE_REPO_REF=main
SYMPHONY_WORKSPACE_BASELINE_DIR=/absolute/path/to/your/source-repo
SYMPHONY_LOCAL_INBOX_DIR=./local_tasks/inbox
SYMPHONY_LOCAL_STATE_DIR=./local_tasks/state
SYMPHONY_LOCAL_ARCHIVE_DIR=./local_tasks/archive
SYMPHONY_LOCAL_RESULTS_DIR=./local_tasks/results
```

Leave `SYMPHONY_WORKSPACE_BASELINE_DIR` empty if you only want the clone hook and not the built-in carry-over.

Optional but recommended for local UI access:

```dotenv
SYMPHONY_SERVER_AUTH_USERNAME=admin
SYMPHONY_SERVER_AUTH_PASSWORD=change-me
```

### 2. Choose the workflow example

Copy the local workflow example into place:

```bash
cp examples/WORKFLOW.local.md WORKFLOW.md
```

This example already includes:

- `tracker.kind: local`
- local inbox/archive/state/results directories
- repo bootstrap via `hooks.after_create`
- built-in `workspace.seed` / `workspace.sync_back` baseline carry-over
- a prompt that tells Codex to finish with `task_update`


If `SYMPHONY_WORKSPACE_BASELINE_DIR` is set:

- the repo still bootstraps through `hooks.after_create`
- Symphony overlays that baseline into each newly created workspace
- when Codex closes a task with `task_update(state: Done)`, Symphony syncs the workspace files back to the baseline directory
- later tasks start from that updated baseline automatically

### 3. Create your first task

```bash
mkdir -p local_tasks/inbox
cp examples/local_tasks/hello-endpoint.md local_tasks/inbox/hello-endpoint.md
```

### 4. Start the service

```bash
go run ./cmd/symphonyd -workflow ./WORKFLOW.md -log-level info
```

## What Codex must do to close the loop

In local mode, the task is not considered finished just because files changed.

Codex must call:

- `task_update` with `state: Done` when an implementation task is complete
- `task_update` with `state: Reviewed` when a review/audit task finished successfully and is reporting findings or follow-up slices
- `task_update` with `state: Blocked` only when the task itself cannot proceed

The `summary` should include:

- what changed
- what validation ran
- any follow-up notes or blockers

That final call is what moves the loop from “work happened” to “task closed.”

## What success looks like

The `/issues/<task-id>` page now works for running, retrying, ready, and blocked local tasks. It also shows recent run history for the same task.


A successful first local run usually looks like this:

- the dashboard shows running tasks plus separate ready / blocked backlog rows
- the task appears in the dashboard running list
- a workspace appears under `SYMPHONY_WORKSPACE_ROOT/<task-id>`
- the target repo is cloned into that workspace
- Codex edits files and runs checks in that workspace
- the task leaves the inbox
- the task file appears in `local_tasks/archive/`
- a completed entry appears in `/history`
- result files appear under `local_tasks/results/<task-id>/`
- if baseline sync is enabled, completed `Done` changes land back in `SYMPHONY_WORKSPACE_BASELINE_DIR`

## Result files

For each local task, `symphony-go` writes:

- `summary.md` — concise human-readable outcome
- `metadata.json` — persisted task state, timestamps, and summary
- `comments.md` — tracker comments, if any were written through the comment helper

Example:

```bash
ls local_tasks/results/hello-endpoint
cat local_tasks/results/hello-endpoint/summary.md
cat local_tasks/results/hello-endpoint/metadata.json
```

## Review handoff pattern

For review or audit tasks, a useful local pattern is:

- add `Reviewed` to `local.terminal_states` in your workflow
- have the reviewer finish with `task_update(state: Reviewed, summary: ...)` instead of `Blocked` when the review itself completed
- keep the findings in `local_tasks/results/<review-task-id>/summary.md` plus any extra artifacts that review task writes in the same results directory
- turn each concrete defect or recommendation into a normal follow-up Markdown task under `local_tasks/inbox/`
- let regular implementation Codex workers pick up those follow-up tasks and close them with `Done` or `Blocked`

That keeps review results durable, keeps dependencies unblocked, and makes it explicit that the reviewer reports while implementer tasks repair.

## Important current limitation

In v1 local mode, one `symphony-go` instance still points at one prepared target repo through your workspace hooks.

That means:

- the task file itself does not choose a repo or branch
- repo selection still comes from `hooks.after_create`
- built-in `workspace.seed` / `workspace.sync_back` remove the need for extra overlay hooks, but they do not choose the repo for you
- if you want multi-repo routing later, add another abstraction above the current workspace hook model

## Recommended first tasks

Start with one of these:

- add a tiny read-only API route
- update a README section and verify formatting
- fix a small test and re-run the targeted suite
- add a single CLI flag with one test
