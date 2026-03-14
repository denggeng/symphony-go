#!/bin/sh
set -eu

if [ -z "${SOURCE_REPO_URL:-}" ]; then
  echo "SOURCE_REPO_URL is required" >&2
  exit 1
fi

log_target=""
if [ -n "${SYMPHONY_CONTROL_ROOT:-}" ]; then
  log_dir="${SYMPHONY_CONTROL_ROOT}/runlogs"
  mkdir -p "$log_dir"
  log_target="$log_dir/hook-sync.log"
fi

log() {
  line="[$(date '+%Y-%m-%dT%H:%M:%S%z')] $0 issue=${SYMPHONY_ISSUE_IDENTIFIER:-} workspace=${SYMPHONY_WORKSPACE_PATH:-$PWD} $*"
  if [ -n "$log_target" ]; then
    echo "$line" >> "$log_target"
  else
    echo "$line" >&2
  fi
}

run() {
  if [ -n "$log_target" ]; then
    "$@" >> "$log_target" 2>&1
  else
    "$@"
  fi
}

clone_depth="${SOURCE_REPO_DEPTH:-1}"
set -- clone
if [ -n "$clone_depth" ] && [ "$clone_depth" != "0" ]; then
  set -- "$@" --depth "$clone_depth"
fi
if [ -n "${SOURCE_REPO_REF:-}" ]; then
  set -- "$@" --branch "$SOURCE_REPO_REF"
fi
set -- "$@" "$SOURCE_REPO_URL" .

log "clone source=${SOURCE_REPO_URL} ref=${SOURCE_REPO_REF:-} depth=${clone_depth}"
run git "$@"

if [ -f .gitmodules ]; then
  log "sync submodules"
  run git submodule update --init --recursive
fi
