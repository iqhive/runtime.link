#!/usr/bin/env bash
# Merges github.com/quaadgras/runtime.link main into this fork through the
# qqmerge branch, then fast-forwards main to it.
#
# .github/workflows/upstream-sync.yml runs this from a copy in $RUNNER_TEMP, so
# nothing the merge or Codex changes in the checkout can alter it.
#
#   sync.sh prepare      start the merge and decide whether Codex is needed
#   sync.sh package      bundle the merge Codex resolved for the verify job
#   sync.sh finalize     check the result, commit it and push qqmerge and main
#   sync.sh save-failed  push a failed attempt to qqmerge-failed for a human
#
# Codex runs in the sync job, and its result is checked and pushed by a
# separate verify job on a fresh runner: openai/codex-action's drop-sudo
# locks down the root-owned sockets under /run for the rest of its job, and
# go test stalls there.
#
# With TEST_BRANCH set, it merges that branch of origin instead of
# quaadgras/main and pushes nothing, leaving the result in $RUNNER_TEMP.
set -euo pipefail

UPSTREAM_URL=${UPSTREAM_URL:-https://github.com/quaadgras/runtime.link.git}
MODULE=github.com/iqhive/runtime.link
SRC=quaadgras/main
[ -z "${TEST_BRANCH:-}" ] || SRC=origin/$TEST_BRANCH
HERE=$(cd "$(dirname "$0")" && pwd)
TMP=${RUNNER_TEMP:?}
# Inside .git so Codex can read it and it is never committed.
LOG=$(git rev-parse --git-dir)/upstream-sync.log

out() { echo "$1=$2" >>"${GITHUB_OUTPUT:-/dev/null}"; }
say() { echo "$*"; echo "$*" >>"${GITHUB_STEP_SUMMARY:-/dev/null}"; }

push() {
	local auth
	auth=$(printf 'x-access-token:%s' "${GH_TOKEN:?}" | base64 -w0)
	git -c http.https://github.com/.extraheader="AUTHORIZATION: basic $auth" push "$@"
}

# MERGED_ON is the commit the merge is on top of: HEAD while the merge is in
# progress, HEAD^1 once the verify job has it as a commit.
changed_go() { git diff --name-only --diff-filter=d "${MERGED_ON:-HEAD}" -- '*.go'; }
unmerged() { git diff --name-only --diff-filter=U; }

# keep_test_output leaves a test run's merge and log where the workflow uploads
# them from.
keep_test_output() {
	git diff HEAD^1 HEAD >"$TMP/test-merge.patch"
	cp "$LOG" "$TMP/upstream-sync.log" 2>/dev/null || true
}

# translate rewrites upstream's names to the fork's in the checkout it runs in.
# The merge applies it to upstream's side, both the merge base and the commit
# being merged, so the fork's renames don't show up as conflicts. It mirrors
# what the fork did when it renamed the module: import paths, go:linkname
# targets, path references in comments, the WASM ffi imports and the quoted
# WASM host module name. The [runtime.link/...] doc links, prose, and the guest's
# "//go:wasmimport runtime.link dlopen|dlsym" lines stay as upstream has them.
translate() {
	sed -i "s|^module runtime\.link\$|module $MODULE|" go.mod
	{ git grep -lz 'runtime\.link' -- '*.go' || true; } | xargs -0r sed -i \
		-e "s#\(^\|[^[]\)runtime\.link/#\1$MODULE/#g" \
		-e "s#\"runtime\.link\"#\"$MODULE\"#g"
}

# translated_commit prints a commit holding <commit>'s tree after translate.
translated_commit() {
	local src=$1 wt tree
	wt=$(mktemp -d)
	git worktree add -q --detach "$wt" "$src"
	(cd "$wt" && translate && git add -u)
	tree=$(git -C "$wt" write-tree)
	git worktree remove --force "$wt"
	git commit-tree "$tree" -p "$src" -m "$src with upstream's names translated to the fork's"
}

# merge_translated merges <upstream> into HEAD the way git merge --no-commit
# would, except that both upstream's side and the merge base are translated
# first. Conflicts are left in the index and the working tree as usual, and
# MERGE_HEAD is the real upstream commit, so history and later merge bases
# are unaffected.
merge_translated() {
	local base=$1 up=$2 gitdir out rc=0 tree info
	gitdir=$(git rev-parse --git-dir)
	git branch -f upstream-translated "$(translated_commit "$up")"
	out=$(git merge-tree --write-tree --merge-base="$(translated_commit "$base")" HEAD upstream-translated) || rc=$?
	if [ "$rc" -gt 1 ]; then
		echo "::error::git merge-tree failed: $out"
		exit 1
	fi
	tree=$(head -n1 <<<"$out")
	git read-tree -m -u HEAD "$tree"
	if [ "$rc" = 1 ]; then
		# Replace each conflicted path's merged entry with its unmerged stages.
		info=$(sed -n '2,/^$/{/^$/!p}' <<<"$out")
		cut -f2- <<<"$info" | sort -u |
			sed 's|^|0 0000000000000000000000000000000000000000\t|' | git update-index --index-info
		git update-index --index-info <<<"$info"
	fi
	echo "$up" >"$gitdir/MERGE_HEAD"
	echo "no-ff" >"$gitdir/MERGE_MODE"
	echo "Merge remote-tracking branch '$SRC' into qqmerge" >"$gitdir/MERGE_MSG"
}

# normalize rewrites upstream's module path to the fork's and, once nothing is
# left unmerged, formats the changed Go files and tidies go.mod the way CI
# expects.
normalize() {
	{ git grep -lz '"runtime\.link/' -- '*.go' || true; } |
		xargs -0r sed -i "s|\"runtime\\.link/|\"$MODULE/|g"
	[ -z "$(unmerged)" ] || return 0
	changed_go | xargs -r gofmt -s -w || true
	go mod tidy
}

# checks mirrors .github/workflows/ci.yml, which pushes made with the
# workflow's token don't trigger.
checks() {
	local fail=0 unformatted t
	if [ -n "$(unmerged)" ]; then echo "unmerged paths remain"; fail=1; fi
	if git grep -nE '^(<<<<<<<|>>>>>>>)( |$)' -- .; then echo "conflict markers remain"; fail=1; fi
	if git grep -n '"runtime\.link/' -- '*.go'; then echo "upstream module path remains"; fail=1; fi
	unformatted=$(changed_go | xargs -r gofmt -s -l 2>&1 || true)
	if [ -n "$unformatted" ]; then echo "not gofmt -s formatted: $unformatted"; fail=1; fi
	go build ./... || fail=1
	go vet -structtag=false ./... || fail=1
	go test -race ./... || fail=1
	for t in linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
		GOOS=${t%/*} GOARCH=${t#*/} go build ./... || { echo "build failed for $t"; fail=1; }
	done
	return $fail
}

identity() {
	git config user.name "github-actions[bot]"
	git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
}

commit_message() {
	{
		echo "Merge remote-tracking branch '$SRC' into qqmerge"
		echo
		echo "Upstream commits:"
		git log --reverse --format='- %h %s' "${BASE:?}..${UPSTREAM:?}"
		if [ -s "$TMP/codex-summary.md" ]; then
			echo
			echo "Codex resolved this merge in ${RUN_URL:-the upstream-sync workflow}:"
			echo
			cat "$TMP/codex-summary.md"
		fi
	} >"$TMP/commit-msg"
}

prepare() {
	identity
	if [ -n "${TEST_BRANCH:-}" ]; then
		git fetch -q --no-tags origin "$TEST_BRANCH:refs/remotes/$SRC"
	else
		git remote add quaadgras "$UPSTREAM_URL" 2>/dev/null || git remote set-url quaadgras "$UPSTREAM_URL"
		git fetch -q --no-tags quaadgras main
	fi

	local up base conflicted reason=""
	up=$(git rev-parse "$SRC")
	if git merge-base --is-ancestor "$up" HEAD; then
		say "main already has $SRC ${up:0:7}; nothing to merge."
		out pending false
		return
	fi
	if [ "${GITHUB_EVENT_NAME:-}" = schedule ] &&
		[ "$(git rev-parse -q --verify origin/qqmerge-failed^2)" = "$up" ]; then
		say "The last attempt to merge quaadgras/main ${up:0:7} failed (see branch qqmerge-failed) and upstream hasn't moved since. Run the workflow manually to retry."
		out pending false
		return
	fi
	if git rev-parse -q --verify origin/qqmerge >/dev/null &&
		! git merge-base --is-ancestor origin/qqmerge HEAD; then
		echo "::error::origin/qqmerge has commits that aren't in main. Merge them into main or drop them, then rerun."
		exit 1
	fi
	base=$(git merge-base HEAD "$up")
	out pending true
	out base "$base"
	out upstream "$up"

	# Codex has no network access, so download both sides' modules now.
	go mod download
	local mods
	mods=$(mktemp -d)
	git show "$up:go.mod" >"$mods/go.mod"
	git show "$up:go.sum" >"$mods/go.sum" 2>/dev/null || true
	(cd "$mods" && go mod download) || echo "::warning::Couldn't download upstream's modules."

	cp -a .github "$TMP/github-before"
	git switch -q -C qqmerge
	merge_translated "$base" "$up"
	conflicted=$(unmerged)
	normalize

	if [ -n "$conflicted" ]; then
		reason="the merge conflicted in: $(paste -sd' ' <<<"$conflicted")"
		{ echo "Conflicted files:"; echo "$conflicted"; } >"$LOG"
	elif ! checks >"$LOG" 2>&1; then
		reason="the merge was clean but the checks failed; their output is in .git/upstream-sync.log"
	elif [ "${FORCE_CODEX:-false}" = true ]; then
		reason="a Codex pass was requested for this run, though the merge was clean and the checks passed"
	fi
	cat "$LOG" 2>/dev/null || true

	sed -e "s|{{BASE}}|$base|g" -e "s|{{UPSTREAM}}|$up|g" -e "s|{{REASON}}|$reason|g" \
		"$HERE/codex-prompt.md" >"$TMP/codex-prompt.md"
	if [ -n "$reason" ]; then
		say "Codex is needed: $reason."
		out needs_codex true
	else
		out needs_codex false
	fi
}

# package runs in the sync job after Codex. It commits the merge as Codex left
# it and bundles the commit for the verify job.
package_result() {
	local commit
	if ! diff -r "$TMP/github-before" .github; then
		echo "::error::Codex changed .github; refusing to continue."
		exit 1
	fi
	# Codex can't write to .git, so its resolutions are still unstaged and the
	# index still lists them as conflicts. Leftover markers are caught by the
	# verify job's checks.
	git add -A
	commit_message
	commit=$(git commit-tree "$(git write-tree)" -p HEAD -p "${UPSTREAM:?}" -F "$TMP/commit-msg")
	git update-ref refs/heads/codex-result "$commit"
	git bundle create "$TMP/merge.bundle" codex-result ^main
}

finalize() {
	local up=${UPSTREAM:?}
	identity
	if [ "${NEEDS_CODEX:-false}" = true ]; then
		git fetch -q "$TMP/merge.bundle" refs/heads/codex-result:refs/heads/codex-result
		git switch -q -C qqmerge codex-result
		MERGED_ON=HEAD^1
		normalize
		git add -A
		git diff --cached --quiet || git commit -q --amend --no-edit
		if ! checks 2>&1 | tee "$LOG"; then
			echo "::error::The checks still fail after Codex."
			exit 1
		fi
	else
		commit_message
		git add -A
		git commit -q -F "$TMP/commit-msg"
	fi
	if [ -n "${TEST_BRANCH:-}" ]; then
		keep_test_output
		say "Test run: merged $SRC ${up:0:7} into a local qqmerge and pushed nothing. The merge is in the upstream-sync-test artifact."
		report
		return
	fi
	git switch -q main
	git merge -q --ff-only qqmerge
	push --atomic origin qqmerge main
	if git rev-parse -q --verify origin/qqmerge-failed >/dev/null; then
		push origin :qqmerge-failed || true
	fi

	say "Merged quaadgras/main ${up:0:7} into qqmerge and main."
	report
}

# report adds the merge's diffstat and Codex's summary to the job summary.
report() {
	{
		echo
		echo '```'
		git diff --stat HEAD^1 HEAD
		echo '```'
		if [ -s "$TMP/codex-summary.md" ]; then
			echo
			cat "$TMP/codex-summary.md"
		fi
	} >>"${GITHUB_STEP_SUMMARY:-/dev/null}"
}

save_failed() {
	local up=${UPSTREAM:?}
	identity
	if [ -f "$(git rev-parse --git-dir)/MERGE_HEAD" ]; then
		git add -A
		git commit -q --no-verify -m "WIP: failed merge of $SRC ${up:0:7}" -m "See ${RUN_URL:-the upstream-sync workflow run}."
	fi
	[ "$(git rev-parse -q --verify HEAD^2)" = "$up" ] || return 0
	if [ -n "${TEST_BRANCH:-}" ]; then
		keep_test_output
		say "Test run: couldn't merge $SRC ${up:0:7}; pushed nothing. The attempt is in the upstream-sync-test artifact."
		return
	fi
	push --force origin HEAD:refs/heads/qqmerge-failed
	say "Couldn't merge quaadgras/main ${up:0:7} automatically. The attempt is on branch qqmerge-failed."
}

case "${1:-}" in
prepare) prepare ;;
package) package_result ;;
finalize) finalize ;;
save-failed) save_failed ;;
*)
	echo "usage: $0 prepare|package|finalize|save-failed" >&2
	exit 2
	;;
esac
