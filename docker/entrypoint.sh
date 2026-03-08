#!/bin/sh
set -eu

workflow_path="${SYMPHONY_WORKFLOW_PATH:-/app/WORKFLOW.md}"
workspace_root="${SYMPHONY_WORKSPACE_ROOT:-/var/lib/symphony/workspaces}"

if [ ! -f "$workflow_path" ]; then
  echo "error: workflow file not found at $workflow_path" >&2
  exit 1
fi

mkdir -p "$workspace_root"

if ! command -v codex >/dev/null 2>&1; then
  echo "warning: codex CLI not found in container PATH." >&2
  echo "warning: set CODEX_INSTALL_COMMAND during docker build or provide a custom image with codex installed." >&2
fi

exec "$@"
