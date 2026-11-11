#!/usr/bin/env bash
# Checks commit subjects against conventional commits (https://www.conventionalcommits.org).
# Usage: check-commits.sh <rev-range>   e.g. origin/master..HEAD (merge commits are skipped)
#        check-commits.sh --file <path>  a commit message file, as the commit-msg hook passes it
set -euo pipefail

types='feat|fix|perf|refactor|docs|test|build|ci|chore|style|revert'
pattern="^(${types})(\([[:alnum:]._/-]+\))?!?: [^[:space:]]"
# git's own subjects for reverts, fixups and squashes, which a rebase or merge resolves.
allowed='^(Revert "|fixup! |squash! |amend! )'

bad=0
check() {
  local sha=$1 subject=$2
  if [[ ! $subject =~ $pattern && ! $subject =~ $allowed ]]; then
    echo "not a conventional commit: ${sha:+$sha }$subject" >&2
    bad=1
  fi
}

if [[ ${1:-} == --file ]]; then
  check "" "$(grep -v '^#' "$2" | grep -m1 -v '^[[:space:]]*$' || true)"
else
  while IFS=$'\t' read -r sha subject; do
    check "$sha" "$subject"
  done < <(git log --no-merges --format='%h%x09%s' "${1:?usage: check-commits.sh <rev-range> | --file <path>}")
fi

if ((bad)); then
  echo "want <type>[(scope)][!]: <description>, with type one of ${types//|/, }" >&2
  exit 1
fi
