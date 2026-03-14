#!/bin/sh
set -eu

if [ -z "${SYMPHONY_CONTROL_ROOT:-}" ]; then
  echo "SYMPHONY_CONTROL_ROOT is required" >&2
  exit 1
fi

log_dir="${SYMPHONY_CONTROL_ROOT}/runlogs"
mkdir -p "$log_dir"
log_file="$log_dir/hook-sync.log"

echo "[$(date '+%Y-%m-%dT%H:%M:%S%z')] $0 issue=${SYMPHONY_ISSUE_IDENTIFIER:-} workspace=${SYMPHONY_WORKSPACE_PATH:-$PWD}" >> "$log_file"
repo_source_path="${SOURCE_REPO_URL#file://}"
if [ "$repo_source_path" = "${SOURCE_REPO_URL:-}" ] || [ ! -d "$repo_source_path" ]; then
  git submodule update --init --recursive >> "$log_file" 2>&1 || true
  exit 0
fi
git clone --branch "$SOURCE_REPO_REF" "$SOURCE_REPO_URL" . >> "$log_file" 2>&1
rsync -a   --exclude '.git/'   --exclude '.gocache/'   --exclude 'tmp/'   "$repo_source_path"/ ./ >> "$log_file" 2>&1

rm -f "$SYMPHONY_WORKSPACE_PATH/.agents/skills/.git" 2>/dev/null || true
