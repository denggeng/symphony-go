#!/bin/sh
set -eu

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

resolve_local_source() {
  source_value="${SOURCE_REPO_URL:-}"
  if [ -z "$source_value" ]; then
    return 1
  fi
  case "$source_value" in
    file://*) source_value="${source_value#file://}" ;;
  esac
  if [ -d "$source_value" ]; then
    printf '%s\n' "$source_value"
    return 0
  fi
  return 1
}

if ! command -v rsync >/dev/null 2>&1; then
  echo "rsync is required" >&2
  exit 1
fi

repo_source_path="$(resolve_local_source || true)"
if [ -z "$repo_source_path" ]; then
  log "skip non-local source=${SOURCE_REPO_URL:-}"
  if [ -f .gitmodules ]; then
    run git submodule update --init --recursive || true
  fi
  exit 0
fi

log "sync from local source=${repo_source_path}"
run rsync -a \
  --exclude '.git' \
  --exclude '.git/' \
  --exclude '.gocache/' \
  --exclude '.gotmp/' \
  --exclude 'tmp/' \
  "$repo_source_path"/ "$SYMPHONY_WORKSPACE_PATH"/

if [ -f .gitmodules ]; then
  run git submodule update --init --recursive || true
fi
