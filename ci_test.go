// Package ci_test checks the workflows and scripts under .github, which go
// test can't reach from inside because it ignores dot directories.
package ci_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
			if indentOf(entry) == 2 && !isBlank(entry) {
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

// sendToCodex makes prepare take the path that ends in a Codex run. The real
// merge is replaced because it needs GNU sed, which macOS runners don't have.
const sendToCodex = `
merge_translated() { echo 'name: changed upstream' >.github/workflows/ci.yml; }
normalize() { :; }
checks() { echo checks failed; return 1; }
`

const afterPrepare = `
UPSTREAM=$(git rev-parse quaadgras/main)
BASE=$(git merge-base HEAD "$UPSTREAM")
`

func TestSyncAcceptsUpstreamChangesToGithub(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	out, err := bashE(t, fork, syncFunctions(t)+sendToCodex+"prepare\n"+afterPrepare+"package_result\n", env...)
	if err != nil {
		t.Fatalf("the merge's own change to .github was rejected: %v\n%s", err, out)
	}
	if !strings.Contains(readFile(t, envValue(t, env, "GITHUB_OUTPUT")), "needs_codex=true") {
		t.Fatalf("prepare didn't ask for Codex:\n%s", out)
	}
}

func TestSyncRejectsCodexChangingGithub(t *testing.T) {
	requireShell(t)
	fork, env := syncRepo(t)
	codex := "echo 'name: changed by codex' >.github/workflows/ci.yml\n"
	out, err := bashE(t, fork, syncFunctions(t)+sendToCodex+"prepare\n"+afterPrepare+codex+"package_result\n", env...)
	if err == nil {
		t.Fatalf("package accepted a change Codex made to .github:\n%s", out)
	}
	if !strings.Contains(out, "Codex changed .github") {
		t.Fatalf("package failed for another reason: %v\n%s", err, out)
	}
}

func TestSyncPromptKeepsSedSpecialCharactersInReason(t *testing.T) {
	requireShell(t)
	const path = `a|b&c\d.go`
	fork, env := syncRepo(t)
	stubs := sendToCodex + `unmerged() { printf '%s\n' 'a|b&c\d.go'; }` + "\n"
	out, err := bashE(t, fork, syncFunctions(t)+stubs+"prepare\n", env...)
	if err != nil {
		t.Fatalf("prepare failed on a conflicted path: %v\n%s", err, out)
	}
	prompt := readFile(t, filepath.Join(envValue(t, env, "RUNNER_TEMP"), "codex-prompt.md"))
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
	out, err := bashE(t, fork, syncFunctions(t)+sendToCodex+"prepare\n", env...)
	if err != nil {
		t.Fatalf("prepare failed: %v\n%s", err, out)
	}
	log := readFile(t, filepath.Join(envValue(t, env, "RUNNER_TEMP"), "go.log"))
	if !strings.Contains(log, "wasm: go mod download") {
		t.Errorf("Codex has no network, but prepare didn't download api/wasm's modules:\n%s", log)
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
