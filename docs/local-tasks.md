# Local Markdown tasks

This is the simplest way to run `symphony-go` in a full local closed loop.

Instead of creating a Jira issue, you drop a Markdown file into an inbox directory. `symphony-go` turns that file into a task, creates a workspace, runs Codex, and waits for Codex to finish by calling `task_update`.

## What is supported

In local mode today:

- `tracker.kind: local`
- polling a task inbox directory for `*.md` files
- optional front matter fields: `id`, `title`, `state`
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
SYMPHONY_LOCAL_INBOX_DIR=./local_tasks/inbox
SYMPHONY_LOCAL_STATE_DIR=./local_tasks/state
SYMPHONY_LOCAL_ARCHIVE_DIR=./local_tasks/archive
SYMPHONY_LOCAL_RESULTS_DIR=./local_tasks/results
```

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
- a prompt that tells Codex to finish with `task_update`

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

- `task_update` with `state: Done` when the task is complete, or
- `task_update` with `state: Blocked` when it cannot proceed

The `summary` should include:

- what changed
- what validation ran
- any follow-up notes or blockers

That final call is what moves the loop from “work happened” to “task closed.”

## What success looks like

A successful first local run usually looks like this:

- the task appears in the dashboard running list
- a workspace appears under `SYMPHONY_WORKSPACE_ROOT/<task-id>`
- the target repo is cloned into that workspace
- Codex edits files and runs checks in that workspace
- the task leaves the inbox
- the task file appears in `local_tasks/archive/`
- a completed entry appears in `/history`
- result files appear under `local_tasks/results/<task-id>/`

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

## Important current limitation

In v1 local mode, one `symphony-go` instance still points at one prepared target repo through your workspace hooks.

That means:

- the task file itself does not choose a repo or branch
- repo selection still comes from `hooks.after_create`
- if you want multi-repo routing later, add another abstraction above the current workspace hook model

## Recommended first tasks

Start with one of these:

- add a tiny read-only API route
- update a README section and verify formatting
- fix a small test and re-run the targeted suite
- add a single CLI flag with one test
