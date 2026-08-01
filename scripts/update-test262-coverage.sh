#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
manifest="$repo_root/test262/selection.json"
temporary_root=""

cleanup() {
  if [[ -n "$temporary_root" && -d "$temporary_root" ]]; then
    rm -rf -- "$temporary_root"
  fi
}
trap cleanup EXIT INT TERM

for command_name in git go jq mktemp; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "error: required command not found: $command_name" >&2
    exit 2
  fi
done

test262_commit="$(jq -er '.commit | select(type == "string" and length > 0)' "$manifest")"
temporary_root="$(mktemp -d "${TMPDIR:-/tmp}/jgo-test262.XXXXXX")"
checkout="$temporary_root/test262"
cd -- "$repo_root"

echo "Checking out Test262 commit $test262_commit..."
git init --quiet "$checkout"
git -C "$checkout" remote add origin https://github.com/tc39/test262.git
git -C "$checkout" fetch --quiet --depth=1 origin "$test262_commit"
git -C "$checkout" checkout --quiet --detach FETCH_HEAD

echo "Running the complete pinned Test262 suite..."
runner_status=0
go run ./cmd/test262 \
  -all \
  -test262 "$checkout" \
  -selection "$manifest" \
  -baseline "$repo_root/test262/full-baseline.json" \
  -refresh-baseline \
  -timeout 100ms \
  -coverage "$repo_root/COVERAGE.md" \
  -json "$temporary_root/test262-report.json" \
  -junit "$temporary_root/test262-report.xml" \
  "$@" || runner_status=$?

if ((runner_status != 0)); then
  echo "Test262 exited with status $runner_status; generated coverage files were preserved for review." >&2
  exit "$runner_status"
fi

echo "Updated COVERAGE.md and test262/full-baseline.json."
