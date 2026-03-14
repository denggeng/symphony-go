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

copy_if_present() {
  source_path="$1"
  target_path="$2"
  if [ -e "$source_path" ]; then
    cp -R "$source_path" "$target_path"
  fi
}

if [ -z "${SYMPHONY_ISSUE_REVIEW_OF:-}" ]; then
  log "skip no review_of"
  exit 0
fi

if [ -z "${SYMPHONY_LOCAL_RESULTS_DIR:-}" ] || [ -z "${SYMPHONY_WORKSPACE_PATH:-}" ]; then
  log "skip missing results/workspace env"
  exit 0
fi

review_root="${SYMPHONY_LOCAL_RESULTS_DIR}/${SYMPHONY_ISSUE_REVIEW_OF}"
if [ ! -d "$review_root" ]; then
  log "skip missing review target root=${review_root}"
  exit 0
fi

target_root="${SYMPHONY_WORKSPACE_PATH}/.symphony/review-target"
rm -rf "$target_root"
mkdir -p "$target_root"

copy_if_present "$review_root/summary.md" "$target_root/summary.md"
copy_if_present "$review_root/metadata.json" "$target_root/metadata.json"
copy_if_present "$review_root/comments.md" "$target_root/comments.md"
copy_if_present "$review_root/git" "$target_root/git"

cat > "$target_root/README.md" <<INFO
# Review Target

- Review task: ${SYMPHONY_ISSUE_IDENTIFIER}
- Reviewing task: ${SYMPHONY_ISSUE_REVIEW_OF}
- Source artifacts: ${review_root}

Suggested reading order:

1. ./summary.md
2. ./git/review-target.md
3. ./git/commit.txt (if present)
4. ./git/commit.patch or ./git/diff.patch
5. ./git/changed-files.txt
INFO

log "prepared review target at ${target_root}"
