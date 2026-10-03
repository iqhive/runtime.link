You are finishing an automated merge of upstream changes into a Go repository.

## Situation

- This checkout is github.com/iqhive/runtime.link, a fork of github.com/quaadgras/runtime.link. Upstream's Go module path is `runtime.link`; the fork's is `github.com/iqhive/runtime.link`.
- A merge of upstream commit {{UPSTREAM}} into the fork is in progress (`git merge --no-commit`). The merge base is {{BASE}}.
- Before merging, upstream's side (the merge base and the commit being merged) had upstream's module path rewritten to the fork's with sed, so the fork's renames don't show up as conflicts: the conflicts that remain are real. `git diff {{UPSTREAM}} upstream-translated` shows that rewrite.
- You were called because {{REASON}}.
- `.git/upstream-sync.log` has the details for this run, including any build or test output.

Useful read-only commands:

- `git log --oneline {{BASE}}..{{UPSTREAM}}` lists the new upstream commits.
- `git diff {{BASE}} {{UPSTREAM}}` is the new upstream code.
- `git diff {{BASE}} HEAD` is the fork's own changes since the merge base.
- `git diff --name-only --diff-filter=U` lists the files still in conflict.

## Tasks

1. **Resolve every conflict.** Keep the intent of both sides: upstream's fixes and features, and the fork's own changes. Where both sides added different code at the same place (new tests, functions or cases), keep both. Remove every conflict marker.
2. **Use the fork's module path.** Imports of `runtime.link/...`, including those inside Go string literals that hold source code, must be `github.com/iqhive/runtime.link/...`. Leave `runtime.link` in comments and doc links as it is, and don't change the `module` line in go.mod.
3. **Bug-scrub the new upstream code.** Review `git diff {{BASE}} {{UPSTREAM}}`, as it now sits in the fork, for correctness bugs: wrong logic, unhandled errors, nil dereferences, races, resource leaks, unbounded reads, injection or escaping bugs, and incompatibilities with the fork's own changes. Fix the real bugs you are confident about with the smallest change that fits the surrounding code, and add a test for a fix when practical. Don't refactor, rename, reformat or make style changes, and leave the fork's existing code alone unless the merge breaks it.
4. **Make it build and pass.** `go build ./...`, `go vet -structtag=false ./...` and `go test ./...` must pass. The Go toolchain and modules are already downloaded and there is no network access, so don't add dependencies. Don't delete, skip or weaken a test to make it pass unless the test itself is wrong, and say so if you do.

## Rules

- Don't run git commands that change the repository (add, commit, checkout, switch, reset, restore, stash, merge, rebase). The workflow stages and commits your changes.
- Don't change anything under `.github/`. The run is rejected if you do.
- Don't leave notes, scratch files or build output in the checkout.

## Final message

Your final message becomes part of the merge commit message. Write it as plain Markdown with these sections:

### Conflicts

One bullet per conflicted file saying how you resolved it, or "None" if nothing conflicted.

### Bug fixes

One bullet per fix: `file:line`, the bug, the fix. "None found" if there were none.

### For review

Anything a reviewer should double-check, such as tests you changed, judgement calls, or bugs you found but didn't fix. "Nothing" if there's nothing.
