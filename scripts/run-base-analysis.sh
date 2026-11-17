#!/usr/bin/env bash
# Analyze the pull request's base commit with the head's analyzer and configuration.
# Every failure means "base unavailable"; this step never fails the job.
set -uo pipefail

unavailable() {
  printf '::warning::Columbo feedback skipped: the base commit could not be analyzed (%s).\n' "$1"
  printf 'base-status=unavailable\n' >> "$GITHUB_OUTPUT"
  exit 0
}

[[ "${COLUMBO_BASE_SHA:-}" =~ ^[0-9a-f]{40}$ ]] || unavailable "no base commit in the pull request event"
[[ -x "${COLUMBO_ANALYZER:-}" ]] || unavailable "the analyzer was not built"
config="${COLUMBO_CONFIG:-}"
[[ "$config" != /* ]] || unavailable "an absolute config path cannot be mapped into the base commit"

head_module="$GITHUB_WORKSPACE/$COLUMBO_WORKING_DIRECTORY"
log="$COLUMBO_EVIDENCE/base-git.txt"
prefix=$(git -C "$head_module" rev-parse --show-prefix 2>>"$log") || unavailable "the head checkout is not a Git repository"
# The default pull_request checkout merges the PR into the current base tip.
# Read the commit object: a depth-1 checkout cannot resolve HEAD^1.
base="$COLUMBO_BASE_SHA"
parents=()
while IFS= read -r line; do
  [[ -n "$line" ]] || break
  if [[ "$line" == parent\ * ]]; then parents+=("${line#parent }"); fi
done < <(git -C "$head_module" cat-file -p HEAD 2>>"$log")
if [[ ${#parents[@]} -eq 2 && "${parents[1]}" == "${COLUMBO_PR_HEAD_SHA:-}" ]]; then
  base="${parents[0]}"
fi
git -C "$head_module" fetch --no-tags --depth=1 origin "$base" >>"$log" 2>&1 \
  || unavailable "fetching the base commit failed"
checkout="$(mktemp -d "$RUNNER_TEMP/columbo-base.XXXXXX")/src"
git -C "$head_module" worktree add --detach "$checkout" "$base" >>"$log" 2>&1 \
  || unavailable "checking out the base commit failed"
trap 'git -C "$head_module" worktree remove --force "$checkout" >>"$log" 2>&1' EXIT

base_module="$checkout/$prefix"
[[ -d "$base_module" ]] || unavailable "the working directory does not exist at the base commit"
# The analyzer only reads configuration inside the module, so the head's file is
# copied to the same place in the base checkout. Both runs then share one config.
if [[ -f "$head_module/${config:-.columbo.yml}" ]]; then
  mkdir -p "$(dirname "$base_module/${config:-.columbo.yml}")"
  cp "$head_module/${config:-.columbo.yml}" "$base_module/${config:-.columbo.yml}"
elif [[ -n "$config" ]]; then
  unavailable "the configured file is missing at head"
else
  rm -f "$base_module/.columbo.yml"
fi

args=(--staged=false --no-history --output "$COLUMBO_EVIDENCE/base.sqlite")
if [[ -n "$config" ]]; then args+=(--config "$config"); fi
status=0
(cd "$base_module" && "$COLUMBO_ANALYZER" "${args[@]}" ./...) >"$COLUMBO_EVIDENCE/base-report.txt" 2>&1 || status=$?
printf '%s\n' "$status" > "$COLUMBO_EVIDENCE/base-exit-status.txt"
case "$status" in
  0|1) printf 'base-snapshot=%s\n' "$COLUMBO_EVIDENCE/base.sqlite" >> "$GITHUB_OUTPUT" ;;
  *) unavailable "the analyzer exited $status; see base-report.txt in the evidence" ;;
esac
