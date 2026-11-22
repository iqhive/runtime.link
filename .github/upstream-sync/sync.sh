#!/usr/bin/env bash
# Merges github.com/quaadgras/runtime.link main into this fork through the
# qqmerge branch, then fast-forwards main to it.
#
# .github/workflows/upstream-sync.yml runs this from a copy in $RUNNER_TEMP, so
# nothing the merge or Codex changes in the checkout can alter it.
#
#   sync.sh prepare         start the merge, check it if it is clean, and
#                           decide whether Codex is needed
#   sync.sh commit          commit a clean merge and bundle it for the push job
#   sync.sh remerge         start the merge again for Codex, checking nothing
#   sync.sh package         bundle the merge Codex resolved for the verify job
#   sync.sh verify          check that merge and bundle it for the push job
#   sync.sh package-failed  bundle a failed attempt for the push job
#   sync.sh finalize        push the bundled merge to qqmerge and main
#   sync.sh save-failed     push the bundled failed attempt to qqmerge-failed
#
# The merged tree is upstream's code and Codex's, and building or testing it
# runs that code. Whatever has run on a runner can reach what the job's later
# steps are given, so a secret only goes to a job where none of that code has
# run before:
#
#   - The token that can push goes to the push job, which runs finalize or
#     save-failed. They move branches to a commit that another job bundled, and
#     never check it out. The other jobs can only read the repository.
#   - The OpenAI key goes to the codex job, which runs remerge and then Codex.
#     The sync job has already run the checks on the merge, if it was clean, so
#     remerge makes the merge again on its own runner and runs nothing of it.
#
# A merge that changes anything under .github is never pushed: the push job
# takes this script from main, so such a merge would decide what the next run
# does with the token. finalize saves it as a failed attempt for a person to
# review.
#
# Codex's result is checked by a separate verify job on a fresh runner:
# openai/codex-action's drop-sudo locks down the root-owned sockets under /run
# for the rest of its job, and go test stalls there.
#
# With TEST_BRANCH set, it merges that branch of origin instead of
# quaadgras/main and the push job doesn't run, leaving the result in
# $RUNNER_TEMP.
set -euo pipefail

UPSTREAM_URL=${UPSTREAM_URL:-https://github.com/quaadgras/runtime.link.git}
MODULE=github.com/iqhive/runtime.link
SRC=quaadgras/main
[ -z "${TEST_BRANCH:-}" ] || SRC=origin/$TEST_BRANCH
HERE=$(cd "$(dirname "$0")" && pwd)
TMP=${RUNNER_TEMP:?}
# Where the workflow unpacks what an earlier job uploaded: directories of
# their own, away from this script, because the job that made an artifact
# decides what files are in it.
CODEX_INPUT=$TMP/codex-input
CODEX_RESULT=$TMP/codex-result
MERGE_RESULT=$TMP/merge-result
# Inside .git so Codex can read it and it is never committed.
LOG=$(git rev-parse --git-dir)/upstream-sync.log

out() { echo "$1=$2" >>"${GITHUB_OUTPUT:-/dev/null}"; }
say() { echo "$*"; echo "$*" >>"${GITHUB_STEP_SUMMARY:-/dev/null}"; }

# without_token stops a command of the jobs that run the merged code, or have
# Codex work on it, from starting where the push token is in reach.
without_token() {
	if [ -n "${GH_TOKEN:-}" ]; then
		echo "::error::sync.sh $1 belongs to a job that runs the merged code, so it must not be given GH_TOKEN."
		exit 1
	fi
}

push() {
	local auth
	auth=$(printf 'x-access-token:%s' "${GH_TOKEN:?}" | base64 -w0)
	git -c http.https://github.com/.extraheader="AUTHORIZATION: basic $auth" push "$@"
}

# MERGED_ON is the commit the merge is on top of: HEAD while the merge is in
# progress, HEAD^1 once the verify job has it as a commit. The names are
# NUL-separated, for xargs -0: a name can hold spaces, and without -z git
# quotes one that isn't ASCII.
changed_go() { git diff -z --name-only --diff-filter=d "${MERGED_ON:-HEAD}" -- '*.go'; }
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
# targets, path references in comments, the WASM ffi imports, the quoted WASM
# host module name and the guest's "//go:wasmimport runtime.link" lines, which
# have to name the same module as the host. The [runtime.link/...] doc links
# and prose stay as upstream has them.
translate() {
	sed -i "s|^module runtime\.link\$|module $MODULE|" go.mod
	{ git grep -lz 'runtime\.link' -- '*.go' || true; } | xargs -0r sed -i \
		-e "s#\(^\|[^[]\)runtime\.link/#\1$MODULE/#g" \
		-e "s#\"runtime\.link\"#\"$MODULE\"#g" \
		-e "s#^//go:wasmimport runtime\.link #//go:wasmimport $MODULE #"
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

# wasm_module runs a command in api/wasm, a module of its own that the root's
# ./... doesn't reach.
wasm_module() { (cd api/wasm && "$@"); }

# normalize rewrites upstream's module path to the fork's and, once nothing is
# left unmerged, formats the changed Go files and tidies go.mod the way CI
# expects. api/wasm requires the root module, so it can need tidying as well.
normalize() {
	{ git grep -lz '"runtime\.link/' -- '*.go' || true; } |
		xargs -0r sed -i "s|\"runtime\\.link/|\"$MODULE/|g"
	[ -z "$(unmerged)" ] || return 0
	# gofmt fails on a file that doesn't parse, which is for checks to report.
	changed_go | xargs -0r gofmt -s -w || true
	go mod tidy
	wasm_module go mod tidy
}

# checks mirrors .github/workflows/ci.yml, which pushes made with the
# workflow's token don't trigger.
checks() {
	local fail=0 unformatted t
	if [ -n "$(unmerged)" ]; then echo "unmerged paths remain"; fail=1; fi
	if git grep -nE '^(<<<<<<<|>>>>>>>)( |$)' -- .; then echo "conflict markers remain"; fail=1; fi
	if git grep -n '"runtime\.link/' -- '*.go'; then echo "upstream module path remains"; fail=1; fi
	unformatted=$(changed_go | xargs -0r gofmt -s -l 2>&1 || true)
	if [ -n "$unformatted" ]; then echo "not gofmt -s formatted: $unformatted"; fail=1; fi
	go build ./... || fail=1
	go vet -structtag=false ./... || fail=1
	go test -race ./... || fail=1
	wasm_module go build ./... || fail=1
	wasm_module go vet -structtag=false ./... || fail=1
	GOOS=wasip1 GOARCH=wasm wasm_module go vet -structtag=false ./... || fail=1
	wasm_module go test -race ./... || fail=1
	for t in linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
		GOOS=${t%/*} GOARCH=${t#*/} go build ./... || { echo "build failed for $t"; fail=1; }
		GOOS=${t%/*} GOARCH=${t#*/} wasm_module go build ./... || { echo "api/wasm build failed for $t"; fail=1; }
	done
	return $fail
}

# sed_literal escapes text for the replacement side of a sed s||| command.
sed_literal() { sed -e 's/[\\|&]/\\&/g' <<<"$1"; }

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

# github_tree prints the id of a commit's .github tree. The id covers every
# name, mode and byte under it, links as links, so two commits have the same
# one only if nothing there differs. A commit without .github prints nothing.
github_tree() { git rev-parse -q --verify "$1:.github" || true; }

# github_snapshot prints the same for .github as it is in the checkout, which
# is where Codex works. It goes through an index of its own: the real one may
# hold conflicts, which no tree can be written from. Files that .gitignore
# names are included, as they are part of what the checkout holds.
github_snapshot() {
	local index=$TMP/github-index
	rm -f "$index"
	if [ -e .github ] || [ -L .github ]; then
		GIT_INDEX_FILE=$index git add -A -f -- .github
	fi
	GIT_INDEX_FILE=$index git write-tree
}

# ignored_files lists the files in the checkout that .gitignore names and git
# doesn't track, NUL-separated and sorted. The repository ignores everything
# but a list of kinds, so a new file of any other kind is one of these, and
# git add -A leaves it out without a word.
ignored_files() { git ls-files -z --others --ignored --exclude-standard | sort -z; }

# stage_ignored stages the files that .gitignore names and that have come to
# be in the checkout since remerge listed them, which is what Codex added: a
# test's data, or a file that is embedded. They are part of the merge as Codex
# left it, and the checks of the verify job need them. It says which they
# were. In a job that Codex doesn't work in there is no list, and nothing that
# the checks left behind is staged.
stage_ignored() {
	[ -f "$TMP/ignored-before" ] || return 0
	ignored_files >"$TMP/ignored-after"
	comm -z -13 "$TMP/ignored-before" "$TMP/ignored-after" >"$TMP/ignored-added"
	[ -s "$TMP/ignored-added" ] || return 0
	xargs -0 git add -f -- <"$TMP/ignored-added"
	say "Files that .gitignore names were added to the merge, as it was left with them: $(tr '\0' ' ' <"$TMP/ignored-added")"
}

# fetch_upstream leaves the commit to merge at $SRC.
fetch_upstream() {
	if [ -n "${TEST_BRANCH:-}" ]; then
		git fetch -q --no-tags origin "$TEST_BRANCH:refs/remotes/$SRC"
		return
	fi
	git remote add quaadgras "$UPSTREAM_URL" 2>/dev/null || git remote set-url quaadgras "$UPSTREAM_URL"
	git fetch -q --no-tags quaadgras main
}

# begin_merge leaves the merge of <upstream> in progress on qqmerge, and
# CONFLICTED listing the paths that conflicted. The snapshot is taken after
# the merge, so that package only sees what Codex did to .github, and not what
# upstream did.
begin_merge() {
	git switch -q -C qqmerge
	merge_translated "$1" "$2"
	CONFLICTED=$(unmerged)
	normalize
	github_snapshot >"$TMP/github-before"
}

# for_codex notes that Codex is needed and leaves the log for the codex job,
# which starts on another runner.
for_codex() {
	mkdir -p "$CODEX_INPUT"
	cp "$LOG" "$CODEX_INPUT/upstream-sync.log"
	say "Codex is needed: $1."
	out needs_codex true
}

# last_attempt prints the upstream commit of the attempt that qqmerge-failed
# records, or nothing: the second parent of an attempt that was saved there,
# or the commit that the note of one that couldn't be saved names, see
# note_unsaved.
last_attempt() {
	git rev-parse -q --verify origin/qqmerge-failed^2 && return
	git rev-parse -q --verify origin/qqmerge-failed >/dev/null || return 0
	git log -1 --format=%B origin/qqmerge-failed | sed -n 's/^Upstream-Commit: //p'
}

# prepare runs in the sync job. It runs the checks on a clean merge, which is
# upstream's code, so nothing in its job may be given a secret.
prepare() {
	without_token prepare
	identity
	fetch_upstream

	local up base
	up=$(git rev-parse "$SRC")
	if git merge-base --is-ancestor "$up" HEAD; then
		say "main already has $SRC ${up:0:7}; nothing to merge."
		out pending false
		return
	fi
	if [ "${GITHUB_EVENT_NAME:-}" = schedule ] && [ "$(last_attempt)" = "$up" ]; then
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

	begin_merge "$base" "$up"
	if [ -n "$CONFLICTED" ]; then
		{ echo "Conflicted files:"; echo "$CONFLICTED"; } >"$LOG"
		cat "$LOG"
		for_codex "the merge conflicted in: $(paste -sd' ' - <<<"$CONFLICTED")"
		return
	fi
	local passed=true
	checks >"$LOG" 2>&1 || passed=false
	cat "$LOG"
	if [ "$passed" = false ]; then
		for_codex "the merge was clean but the checks failed"
		return
	fi
	if [ "${FORCE_CODEX:-false}" = true ]; then
		for_codex "a Codex pass was requested for this run, though the merge was clean and the checks passed"
		return
	fi
	out needs_codex false
}

# remerge runs in the codex job, which is given the OpenAI key, so it runs
# nothing from the merged tree: it makes the merge that prepare made, on a
# runner where the checks have not run, and leaves them to the verify job.
# What the sync job's checks printed comes as a file, which only Codex reads.
remerge() {
	without_token remerge
	identity
	fetch_upstream

	local up base reason
	up=$(git rev-parse "$SRC")
	if git merge-base --is-ancestor "$up" HEAD; then
		echo "::error::main already has $SRC ${up:0:7}, though the sync job found it missing. Rerun the workflow."
		exit 1
	fi
	base=$(git merge-base HEAD "$up")
	out base "$base"
	out upstream "$up"

	# Codex has no network access, so download both sides' modules now.
	go mod download
	wasm_module go mod download
	local mods
	mods=$(mktemp -d)
	git show "$up:go.mod" >"$mods/go.mod"
	git show "$up:go.sum" >"$mods/go.sum" 2>/dev/null || true
	(cd "$mods" && go mod download) || echo "::warning::Couldn't download upstream's modules."

	begin_merge "$base" "$up"
	# The files that .gitignore names are listed before Codex works: the ones
	# that are there afterwards and not now are Codex's, see stage_ignored.
	ignored_files >"$TMP/ignored-before"
	if [ -n "$CONFLICTED" ]; then
		reason="the merge conflicted in: $(paste -sd' ' - <<<"$CONFLICTED")"
		{ echo "Conflicted files:"; echo "$CONFLICTED"; } >"$LOG"
	else
		reason="the merge was clean but the checks failed; their output is in .git/upstream-sync.log"
		if [ "${FORCE_CODEX:-false}" = true ]; then
			reason="a Codex pass was requested for this run; the merge was clean, and the output of its checks is in .git/upstream-sync.log"
		fi
		cp "$CODEX_INPUT/upstream-sync.log" "$LOG"
	fi
	cat "$LOG"

	# The reason can name conflicted paths, which may hold any character.
	sed -e "s|{{BASE}}|$base|g" -e "s|{{UPSTREAM}}|$up|g" -e "s|{{REASON}}|$(sed_literal "$reason")|g" \
		"$HERE/codex-prompt.md" >"$TMP/codex-prompt.md"
	say "Codex is called because $reason."
}

# package runs in the codex job after Codex. It commits the merge as Codex left
# it and bundles the commit for the verify job. A file under .github that
# Codex turned into a link, or made executable, has changed like any other.
package_result() {
	without_token package
	local commit
	if [ "$(github_snapshot)" != "$(cat "$TMP/github-before")" ]; then
		echo "::error::Codex changed .github; refusing to continue."
		exit 1
	fi
	# Codex can't write to .git, so its resolutions are still unstaged and the
	# index still lists them as conflicts. Leftover markers are caught by the
	# verify job's checks.
	git add -A
	stage_ignored
	commit_message
	commit=$(git commit-tree "$(git write-tree)" -p HEAD -p "${UPSTREAM:?}" -F "$TMP/commit-msg")
	git update-ref refs/heads/codex-result "$commit"
	git bundle create "$TMP/merge.bundle" codex-result ^main
}

# hand_over bundles the merge on qqmerge for the push job. A test run stops
# here instead, with the merge where the workflow uploads it from.
hand_over() {
	local up=${UPSTREAM:?}
	if [ -n "${TEST_BRANCH:-}" ]; then
		keep_test_output
		say "Test run: merged $SRC ${up:0:7} into a local qqmerge and pushed nothing. The merge is in the upstream-sync-test artifact."
		report qqmerge "$TMP/codex-summary.md"
		return
	fi
	git bundle create "$TMP/merged.bundle" qqmerge ^main
}

# commit_clean runs in the sync job, on a merge that was clean and passed the
# checks.
commit_clean() {
	without_token commit
	identity
	commit_message
	git add -A
	git commit -q -F "$TMP/commit-msg"
	hand_over
}

# verify runs in the verify job, on the merge that package bundled.
verify() {
	without_token verify
	identity
	# Codex's summary goes on to the push job with this job's bundle.
	[ ! -f "$CODEX_RESULT/codex-summary.md" ] || cp "$CODEX_RESULT/codex-summary.md" "$TMP/codex-summary.md"
	git fetch -q "$CODEX_RESULT/merge.bundle" refs/heads/codex-result:refs/heads/codex-result
	git switch -q -C qqmerge codex-result
	MERGED_ON=HEAD^1
	normalize
	git add -A
	git diff --cached --quiet || git commit -q --amend --no-edit
	if ! checks 2>&1 | tee "$LOG"; then
		echo "::error::The checks still fail after Codex."
		exit 1
	fi
	hand_over
}

# package_failed runs in the sync, codex or verify job once it has failed, and
# bundles what the job had of the merge for the push job.
package_failed() {
	local up
	without_token package-failed
	up=${UPSTREAM:?}
	identity
	if [ -f "$(git rev-parse --git-dir)/MERGE_HEAD" ]; then
		git add -A
		stage_ignored
		git commit -q --no-verify -m "WIP: failed merge of $SRC ${up:0:7}" -m "See ${RUN_URL:-the upstream-sync workflow run}."
	fi
	[ "$(git rev-parse -q --verify HEAD^2)" = "$up" ] || return 0
	if [ -n "${TEST_BRANCH:-}" ]; then
		keep_test_output
		say "Test run: couldn't merge $SRC ${up:0:7}; pushed nothing. The attempt is in the upstream-sync-test artifact."
		return
	fi
	git update-ref refs/heads/qqmerge-failed HEAD
	git bundle create "$TMP/failed.bundle" qqmerge-failed ^main
}

# The push job names every ref in full and fetches no tags. A bundle can carry
# a tag called origin/main or qqmerge, and git would take a short name for the
# tag: the checks below would then be made on a commit of the bundle's choosing.
MAIN=refs/remotes/origin/main
MERGED=refs/heads/qqmerge
FAILED=refs/heads/qqmerge-failed

# note_unsaved runs in the push job when an attempt couldn't be pushed to
# qqmerge-failed, which is the only record of it: the token of a workflow may
# not push a commit that changes .github/workflows, and an attempt that merges
# such a change of upstream's is one. Without a record every scheduled run
# makes the same merge again, with a Codex pass if it needs one. So the branch
# is given a note in place of the attempt: a commit on main with main's own
# files, which changes no workflow, whose message names the upstream commit
# for last_attempt to read. If that can't be pushed either, the run fails as
# it would have, and the next one tries again.
note_unsaved() {
	local up=$1 note
	identity
	note=$(git commit-tree "$MAIN^{tree}" -p "$MAIN" \
		-m "The attempt to merge quaadgras/main ${up:0:7} couldn't be saved here" \
		-m "The push of it was refused. Its bundle is in the merge-result artifact of ${RUN_URL:-the upstream-sync workflow run}." \
		-m "Upstream-Commit: $up")
	push --force origin "$note:refs/heads/qqmerge-failed" ||
		echo "::warning::Couldn't note on qqmerge-failed that the attempt wasn't saved; the next scheduled run will make it again."
}

# hold_for_review runs in the push job, on a merge that changes .github. The
# next run's push job takes sync.sh from main and gives it the token, so a
# merge that reaches main with another .github decides what that token does.
# The merge was made where the merged code runs, and nothing the push job is
# handed can be trusted to say who changed what, so every such merge waits
# for a person. It is saved the way a failed attempt is, which also keeps
# scheduled runs from making it again until upstream moves.
hold_for_review() {
	local up=$1 where="It is on branch qqmerge-failed."
	git diff-tree -r "$MAIN" "$MERGED" -- .github
	if ! push --force origin "$MERGED:refs/heads/qqmerge-failed"; then
		where="It couldn't be saved to qqmerge-failed; its bundle is in this run's merge-result artifact."
		note_unsaved "$up"
	fi
	say "The merge of quaadgras/main ${up:0:7} changes .github, so it was not pushed to main or qqmerge: a person must review it. $where"
	echo "::error::The merge changes .github and must be reviewed by a person before it goes to main. $where"
	exit 1
}

# finalize runs in the push job, on what commit or verify bundled. The fetch
# takes the branch as a ref and writes none of its files, and the check on
# .github compares the trees of the two commits, so nothing of the merge is
# checked out here either.
finalize() {
	local up
	if [ ! -f "$MERGE_RESULT/merged.bundle" ]; then
		echo "::error::The sync and verify jobs left no merge to push."
		exit 1
	fi
	git fetch -q --no-tags "$MERGE_RESULT/merged.bundle" "$MERGED:$MERGED"
	up=$(git rev-parse --verify "$MERGED^2")
	if ! git merge-base --is-ancestor "$MAIN" "$MERGED"; then
		echo "::error::main has moved since the merge was made. Rerun the workflow."
		exit 1
	fi
	if [ "$(github_tree "$MAIN")" != "$(github_tree "$MERGED")" ]; then
		hold_for_review "$up"
	fi
	push --atomic origin "$MERGED:refs/heads/qqmerge" "$MERGED:refs/heads/main"
	if git rev-parse -q --verify refs/remotes/origin/qqmerge-failed >/dev/null; then
		push origin :refs/heads/qqmerge-failed || true
	fi

	say "Merged quaadgras/main ${up:0:7} into qqmerge and main."
	report "$MERGED" "$MERGE_RESULT/codex-summary.md"
}

# report adds a merge's diffstat and Codex's summary, from the given file, to
# the job summary.
report() {
	{
		echo
		echo '```'
		git diff --stat "$1^1" "$1"
		echo '```'
		if [ -s "$2" ]; then
			echo
			cat "$2"
		fi
	} >>"${GITHUB_STEP_SUMMARY:-/dev/null}"
}

# save_failed runs in the push job, on what package_failed bundled. An attempt
# that can't be pushed is noted on the branch instead, see note_unsaved, and
# the job fails so that the refusal is seen.
save_failed() {
	local up
	if [ ! -f "$MERGE_RESULT/failed.bundle" ]; then
		say "The run failed before it had a merge of upstream, so there is no attempt to save."
		return
	fi
	git fetch -q --no-tags "$MERGE_RESULT/failed.bundle" "$FAILED:$FAILED"
	up=$(git rev-parse --verify "$FAILED^2")
	if ! push --force origin "$FAILED:$FAILED"; then
		note_unsaved "$up"
		say "Couldn't merge quaadgras/main ${up:0:7} automatically. The attempt couldn't be saved to qqmerge-failed; its bundle is in this run's merge-result artifact."
		echo "::error::The failed attempt couldn't be pushed to qqmerge-failed; its bundle is in this run's merge-result artifact."
		exit 1
	fi
	say "Couldn't merge quaadgras/main ${up:0:7} automatically. The attempt is on branch qqmerge-failed."
}

case "${1:-}" in
prepare) prepare ;;
commit) commit_clean ;;
remerge) remerge ;;
package) package_result ;;
verify) verify ;;
package-failed) package_failed ;;
finalize) finalize ;;
save-failed) save_failed ;;
*)
	echo "usage: $0 prepare|commit|remerge|package|verify|package-failed|finalize|save-failed" >&2
	exit 2
	;;
esac
