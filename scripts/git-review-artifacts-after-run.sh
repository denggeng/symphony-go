#!/bin/sh
set -eu

log_target=""
if [ -n "${SYMPHONY_CONTROL_ROOT:-}" ]; then
  log_dir="${SYMPHONY_CONTROL_ROOT}/runlogs"
  mkdir -p "$log_dir"
  log_target="$log_dir/hook-review.log"
fi

log() {
  line="[$(date '+%Y-%m-%dT%H:%M:%S%z')] $0 issue=${SYMPHONY_ISSUE_IDENTIFIER:-} review_of=${SYMPHONY_ISSUE_REVIEW_OF:-} workspace=${SYMPHONY_WORKSPACE_PATH:-$PWD} $*"
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

is_truthy() {
  case "$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

state_allowed() {
  state="$1"
  allowed_list="$2"
  old_ifs="$IFS"
  IFS=','
  set -- $allowed_list
  IFS="$old_ifs"
  normalized_state="$(printf '%s' "$state" | tr '[:upper:]' '[:lower:]' | xargs)"
  for item in "$@"; do
    normalized_item="$(printf '%s' "$item" | tr '[:upper:]' '[:lower:]' | xargs)"
    if [ -n "$normalized_item" ] && [ "$normalized_item" = "$normalized_state" ]; then
      return 0
    fi
  done
  return 1
}

if ! command -v git >/dev/null 2>&1; then
  log "skip git missing"
  exit 0
fi
if ! command -v jq >/dev/null 2>&1; then
  log "skip jq missing"
  exit 0
fi
if [ -z "${SYMPHONY_LOCAL_RESULTS_DIR:-}" ] || [ -z "${SYMPHONY_ISSUE_IDENTIFIER:-}" ] || [ -z "${SYMPHONY_WORKSPACE_PATH:-}" ]; then
  log "skip missing results/task/workspace env"
  exit 0
fi
if ! git -C "$SYMPHONY_WORKSPACE_PATH" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  log "skip workspace is not a git repo"
  exit 0
fi

result_root="${SYMPHONY_LOCAL_RESULTS_DIR}/${SYMPHONY_ISSUE_IDENTIFIER}"
metadata_path="${result_root}/metadata.json"
if [ ! -f "$metadata_path" ]; then
  log "skip missing metadata path=${metadata_path}"
  exit 0
fi

artifact_root="${result_root}/git"
mkdir -p "$artifact_root"

task_state="$(jq -r '.state // empty' "$metadata_path" 2>/dev/null || true)"
task_title="$(jq -r '.title // empty' "$metadata_path" 2>/dev/null || true)"
review_of="${SYMPHONY_ISSUE_REVIEW_OF:-}"
base_head="$(git -C "$SYMPHONY_WORKSPACE_PATH" rev-parse HEAD 2>/dev/null || true)"
branch_name="$(git -C "$SYMPHONY_WORKSPACE_PATH" symbolic-ref --quiet --short HEAD 2>/dev/null || printf 'DETACHED')"
status_output="$(git -C "$SYMPHONY_WORKSPACE_PATH" status --short --branch 2>/dev/null || true)"

printf '%s\n' "$base_head" > "$artifact_root/head.txt"
printf '%s\n' "$branch_name" > "$artifact_root/branch.txt"
printf '%s\n' "$status_output" > "$artifact_root/status.txt"

index_file="$(mktemp "${TMPDIR:-/tmp}/symphony-git-index.XXXXXX")"
cleanup() {
  rm -f "$index_file"
}
trap cleanup EXIT INT TERM
if [ -n "$base_head" ]; then
  GIT_INDEX_FILE="$index_file" git -C "$SYMPHONY_WORKSPACE_PATH" read-tree HEAD >/dev/null 2>&1 || true
fi
GIT_INDEX_FILE="$index_file" git -C "$SYMPHONY_WORKSPACE_PATH" add -A . >/dev/null 2>&1 || true
GIT_INDEX_FILE="$index_file" git -C "$SYMPHONY_WORKSPACE_PATH" diff --cached --stat --binary --root > "$artifact_root/diff.stat" 2>/dev/null || :
GIT_INDEX_FILE="$index_file" git -C "$SYMPHONY_WORKSPACE_PATH" diff --cached --binary --root > "$artifact_root/diff.patch" 2>/dev/null || :
GIT_INDEX_FILE="$index_file" git -C "$SYMPHONY_WORKSPACE_PATH" diff --cached --name-only --root > "$artifact_root/changed-files.txt" 2>/dev/null || :

commit_enabled="${SYMPHONY_TASK_GIT_AUTO_COMMIT:-}"
commit_states="${SYMPHONY_TASK_GIT_COMMIT_STATES:-Done}"
commit_sha=""
commit_message=""
if is_truthy "$commit_enabled" && state_allowed "$task_state" "$commit_states"; then
  if [ -n "$(git -C "$SYMPHONY_WORKSPACE_PATH" status --porcelain 2>/dev/null || true)" ]; then
    if ! git -C "$SYMPHONY_WORKSPACE_PATH" config user.name >/dev/null 2>&1; then
      run git -C "$SYMPHONY_WORKSPACE_PATH" config user.name "Symphony Bot"
    fi
    if ! git -C "$SYMPHONY_WORKSPACE_PATH" config user.email >/dev/null 2>&1; then
      run git -C "$SYMPHONY_WORKSPACE_PATH" config user.email "symphony@local"
    fi
    commit_message="[symphony] ${SYMPHONY_ISSUE_IDENTIFIER}: ${task_title:-task update}"
    run git -C "$SYMPHONY_WORKSPACE_PATH" add -A
    if run git -C "$SYMPHONY_WORKSPACE_PATH" commit --no-verify -m "$commit_message"; then
      commit_sha="$(git -C "$SYMPHONY_WORKSPACE_PATH" rev-parse HEAD 2>/dev/null || true)"
      printf 'commit=%s\nmessage=%s\n' "$commit_sha" "$commit_message" > "$artifact_root/commit.txt"
      git -C "$SYMPHONY_WORKSPACE_PATH" show --stat --patch --binary --format=fuller HEAD > "$artifact_root/commit.patch" 2>/dev/null || :
      log "created local task commit sha=${commit_sha}"
    fi
  fi
fi

cat > "$artifact_root/review-target.md" <<INFO
# Review Target

- Task: ${SYMPHONY_ISSUE_IDENTIFIER}
- Title: ${task_title}
- Final state: ${task_state}
- Review of: ${review_of}
- Captured at: $(date '+%Y-%m-%dT%H:%M:%S%z')
- Base HEAD: ${base_head:-none}
- Local task commit: ${commit_sha:-not-created}

Artifacts:

- head.txt
- branch.txt
- status.txt
- changed-files.txt
- diff.stat
- diff.patch
- commit.txt (when auto-commit creates a local commit)
- commit.patch (when auto-commit creates a local commit)
INFO

log "exported git review artifacts to ${artifact_root} state=${task_state:-unknown}"
