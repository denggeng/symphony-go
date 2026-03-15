#!/bin/sh
set -eu

if [ -z "${SYMPHONY_CONTROL_ROOT:-}" ]; then
  echo "SYMPHONY_CONTROL_ROOT is required" >&2
  exit 1
fi

if [ -z "${SOURCE_REPO_URL:-}" ]; then
  echo "SOURCE_REPO_URL is required" >&2
  exit 1
fi

log_dir="${SYMPHONY_CONTROL_ROOT}/runlogs"
mkdir -p "$log_dir"
log_file="$log_dir/hook-sync.log"

echo "[$(date '+%Y-%m-%dT%H:%M:%S%z')] $0 issue=${SYMPHONY_ISSUE_IDENTIFIER:-} workspace=${SYMPHONY_WORKSPACE_PATH:-$PWD}" >> "$log_file"

if [ -n "${SOURCE_REPO_REF:-}" ]; then
  git clone --branch "$SOURCE_REPO_REF" "$SOURCE_REPO_URL" . >> "$log_file" 2>&1
else
  git clone "$SOURCE_REPO_URL" . >> "$log_file" 2>&1
fi

git submodule update --init --recursive >> "$log_file" 2>&1 || true
