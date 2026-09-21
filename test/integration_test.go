// Package integration_test exercises the compiled iceman binary end-to-end.
package integration_test

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite snapshot files in testdata/ from actual output")

// binPath is the path to the iceman binary built once in TestMain.
var binPath string

func TestMain(m *testing.M) {
	flag.Parse()
	if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
		*update = true
	}

	tmp, err := os.MkdirTemp("", "iceman-bin")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mktemp:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	binPath = filepath.Join(tmp, "iceman")
	build := exec.Command("go", "build", "-o", binPath, "github.com/wesbragagt/iceman")
	build.Stdout = os.Stderr
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "building iceman:", err)
		os.RemoveAll(tmp)
		os.Exit(1)
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

type result struct {
	stdout   string
	stderr   string
	exitCode int
}

// run invokes the compiled binary with dir as the working directory.
func run(t *testing.T, dir string, args ...string) result {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			t.Fatalf("running %v: %v", args, err)
		}
	}
	return result{stdout: out.String(), stderr: errb.String(), exitCode: code}
}

// assertSnapshot compares actual against testdata/<name>, rewriting it when -update is set.
func assertSnapshot(t *testing.T, name, actual string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(actual), 0o644); err != nil {
			t.Fatalf("writing snapshot %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading snapshot %s (run with -update to create it): %v", path, err)
	}
	if !bytes.Equal(want, []byte(actual)) {
		t.Errorf("snapshot mismatch for %s (run with -update to accept):\n%s", path, diff(string(want), actual))
	}
}

// diff renders a minimal line-by-line comparison; enough to spot content drift.
func diff(want, got string) string {
	w := strings.Split(want, "\n")
	g := strings.Split(got, "\n")
	var b strings.Builder
	n := len(w)
	if len(g) > n {
		n = len(g)
	}
	for i := 0; i < n; i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl == gl {
			b.WriteString("  " + wl + "\n")
			continue
		}
		if i < len(w) {
			b.WriteString("- " + wl + "\n")
		}
		if i < len(g) {
			b.WriteString("+ " + gl + "\n")
		}
	}
	return b.String()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}

func TestInitDefault(t *testing.T) {
	// The default description derives from the working directory name, which is
	// random under t.TempDir(); use a fixed -d so the snapshot is deterministic.
	dir := t.TempDir()
	res := run(t, dir, "init", "-d", "demo dev environment")
	if res.exitCode != 0 {
		t.Fatalf("init failed: %d %s", res.exitCode, res.stderr)
	}
	if !strings.Contains(res.stdout, "Created flake.nix") {
		t.Errorf("unexpected stdout: %q", res.stdout)
	}
	assertSnapshot(t, "init_default.flake.nix", readFile(t, filepath.Join(dir, "flake.nix")))
}

func TestInitDefaultDescriptionUsesDirName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "myproj")
	res := run(t, t.TempDir(), "init", dir)
	if res.exitCode != 0 {
		t.Fatalf("init failed: %d %s", res.exitCode, res.stderr)
	}
	got := readFile(t, filepath.Join(dir, "flake.nix"))
	if !strings.Contains(got, `description = "myproj dev environment";`) {
		t.Errorf("expected description derived from dir name, got:\n%s", got)
	}
}

func TestInitWithDeps(t *testing.T) {
	dir := t.TempDir()
	res := run(t, dir, "init", "-d", "my project", "-p", "go,gopls,git", "somedir")
	if res.exitCode != 0 {
		t.Fatalf("init failed: %d %s", res.exitCode, res.stderr)
	}
	assertSnapshot(t, "init_with_deps.flake.nix", readFile(t, filepath.Join(dir, "somedir", "flake.nix")))
}

func TestInitCreatesEnvrc(t *testing.T) {
	dir := t.TempDir()
	res := run(t, dir, "init", "-d", "demo dev environment")
	if res.exitCode != 0 {
		t.Fatalf("init failed: %d %s", res.exitCode, res.stderr)
	}
	if !strings.Contains(res.stdout, "Created .envrc") {
		t.Errorf("unexpected stdout: %q", res.stdout)
	}
	got := readFile(t, filepath.Join(dir, ".envrc"))
	if got != "use flake\n" {
		t.Errorf("unexpected .envrc contents: %q", got)
	}
}

func TestInitDirenvFalseSkipsEnvrc(t *testing.T) {
	dir := t.TempDir()
	res := run(t, dir, "init", "-d", "demo dev environment", "-direnv=false")
	if res.exitCode != 0 {
		t.Fatalf("init failed: %d %s", res.exitCode, res.stderr)
	}
	if strings.Contains(res.stdout, ".envrc") {
		t.Errorf("expected no .envrc mention with -direnv=false, got stdout: %q", res.stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, ".envrc")); !os.IsNotExist(err) {
		t.Errorf("expected no .envrc file, stat err: %v", err)
	}
}

func TestInitEnvrcAlreadyExistsFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".envrc"), []byte("custom\n"), 0o644); err != nil {
		t.Fatalf("seeding .envrc: %v", err)
	}
	res := run(t, dir, "init", "-d", "demo dev environment")
	if res.exitCode == 0 {
		t.Fatalf("expected init to fail when .envrc already exists, got exit 0")
	}
	if !strings.Contains(res.stderr, ".envrc already exists") {
		t.Errorf("unexpected stderr: %q", res.stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "flake.nix")); !os.IsNotExist(err) {
		t.Errorf("expected flake.nix not to be created when .envrc conflict is detected, stat err: %v", err)
	}
	got := readFile(t, filepath.Join(dir, ".envrc"))
	if got != "custom\n" {
		t.Errorf("existing .envrc was modified: %q", got)
	}
}

func TestInitTwiceMergesDeps(t *testing.T) {
	dir := t.TempDir()
	if res := run(t, dir, "init", "-d", "demo dev environment", "-p", "go"); res.exitCode != 0 {
		t.Fatalf("first init failed: %s", res.stderr)
	}

	res := run(t, dir, "init", "-d", "something else", "-p", "go,git")
	if res.exitCode != 0 {
		t.Fatalf("second init failed: %s", res.stderr)
	}
	if !strings.Contains(res.stdout, "already exists") {
		t.Errorf("expected 'already exists' notice, got stdout: %q", res.stdout)
	}

	after := readFile(t, filepath.Join(dir, "flake.nix"))
	if !strings.Contains(after, "demo dev environment") {
		t.Error("second init should not have changed the existing description")
	}
	if !strings.Contains(after, "go") || !strings.Contains(after, "git") {
		t.Errorf("expected merged deps go and git in flake.nix, got:\n%s", after)
	}
}

func TestAddDeps(t *testing.T) {
	dir := t.TempDir()
	if res := run(t, dir, "init", "-d", "my project", "-p", "go"); res.exitCode != 0 {
		t.Fatalf("init failed: %s", res.stderr)
	}

	res := run(t, dir, "add", "gopls", "git")
	if res.exitCode != 0 {
		t.Fatalf("add failed: %d %s", res.exitCode, res.stderr)
	}
	// `iceman add` reports the flake.nix path it resolved, which is absolute
	// when discovered by the upward search, so match on the path suffix while
	// still pinning the exact set of packages reported as added.
	if got, want := strings.TrimSpace(res.stdout), "Added to "+filepath.Join(dir, "flake.nix")+": gopls, git"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}

	// A second add must keep indentation consistent with the first.
	if res := run(t, dir, "add", "ripgrep"); res.exitCode != 0 {
		t.Fatalf("second add failed: %s", res.stderr)
	}
	assertSnapshot(t, "add_deps.flake.nix", readFile(t, filepath.Join(dir, "flake.nix")))
}

func TestAddToEmptyBuildInputs(t *testing.T) {
	dir := t.TempDir()
	if res := run(t, dir, "init", "-d", "my project"); res.exitCode != 0 {
		t.Fatalf("init failed: %s", res.stderr)
	}
	if res := run(t, dir, "add", "go"); res.exitCode != 0 {
		t.Fatalf("add failed: %s", res.stderr)
	}
	if res := run(t, dir, "add", "git"); res.exitCode != 0 {
		t.Fatalf("add failed: %s", res.stderr)
	}
	assertSnapshot(t, "add_to_empty.flake.nix", readFile(t, filepath.Join(dir, "flake.nix")))
}

func TestAddAlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	if res := run(t, dir, "init", "-d", "my project", "-p", "go,git"); res.exitCode != 0 {
		t.Fatalf("init failed: %s", res.stderr)
	}
	path := filepath.Join(dir, "flake.nix")
	before := readFile(t, path)

	res := run(t, dir, "add", "go")
	if res.exitCode != 0 {
		t.Fatalf("add failed: %d %s", res.exitCode, res.stderr)
	}
	if !strings.Contains(strings.ToLower(res.stdout), "no new packages") {
		t.Errorf("expected 'no new packages' message, got %q", res.stdout)
	}
	if after := readFile(t, path); after != before {
		t.Errorf("file changed:\n%s", diff(before, after))
	}
}

func TestAddNoBuildInputsBlock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "flake.nix")
	contents := "{\n  description = \"broken\";\n  outputs = { self }: { };\n}\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	res := run(t, dir, "add", "go")
	if res.exitCode == 0 {
		t.Fatal("expected non-zero exit for missing buildInputs block")
	}
	if !strings.Contains(res.stderr, "buildInputs") {
		t.Errorf("expected a clear error mentioning buildInputs, got %q", res.stderr)
	}
	if after := readFile(t, path); after != contents {
		t.Error("file was modified despite the error")
	}
}

func TestAddMissingFile(t *testing.T) {
	dir := t.TempDir()
	res := run(t, dir, "add", "go")
	if res.exitCode == 0 {
		t.Fatal("expected non-zero exit when flake.nix is missing")
	}
	if !strings.Contains(res.stderr, "flake.nix") {
		t.Errorf("unexpected stderr: %q", res.stderr)
	}
}

func TestAddCustomFileFlag(t *testing.T) {
	dir := t.TempDir()
	if res := run(t, dir, "init", "-d", "my project", "sub"); res.exitCode != 0 {
		t.Fatalf("init failed: %s", res.stderr)
	}
	res := run(t, dir, "add", "-f", "sub/flake.nix", "go")
	if res.exitCode != 0 {
		t.Fatalf("add failed: %s", res.stderr)
	}
	if !strings.Contains(readFile(t, filepath.Join(dir, "sub", "flake.nix")), "go") {
		t.Error("expected go to be added to sub/flake.nix")
	}
}

func TestSkillWritesAgentsFile(t *testing.T) {
	dir := t.TempDir()
	res := run(t, dir, "skill")
	if res.exitCode != 0 {
		t.Fatalf("skill failed: %d %s", res.exitCode, res.stderr)
	}
	if !strings.Contains(res.stdout, "Created AGENTS.md") {
		t.Errorf("unexpected stdout: %q", res.stdout)
	}
	got := readFile(t, filepath.Join(dir, "AGENTS.md"))
	if !strings.HasPrefix(got, "# Using iceman") {
		t.Errorf("AGENTS.md does not start with the expected heading:\n%s", got)
	}
	if !strings.Contains(got, "iceman init") || !strings.Contains(got, "iceman add") || !strings.Contains(got, "iceman search") {
		t.Errorf("AGENTS.md is missing expected command references:\n%s", got)
	}
}

func TestSkillRefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	if res := run(t, dir, "skill"); res.exitCode != 0 {
		t.Fatalf("first skill failed: %s", res.stderr)
	}
	res := run(t, dir, "skill")
	if res.exitCode == 0 {
		t.Fatal("expected second skill invocation to fail without -force")
	}
	if !strings.Contains(res.stderr, "already exists") {
		t.Errorf("unexpected stderr: %q", res.stderr)
	}
}

func TestSkillForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	if res := run(t, dir, "skill"); res.exitCode != 0 {
		t.Fatalf("first skill failed: %s", res.stderr)
	}
	res := run(t, dir, "skill", "-force")
	if res.exitCode != 0 {
		t.Fatalf("skill -force failed: %d %s", res.exitCode, res.stderr)
	}
}

func TestSkillCustomFileAndDirectory(t *testing.T) {
	dir := t.TempDir()
	res := run(t, dir, "skill", "-file", "SKILL.md", "sub")
	if res.exitCode != 0 {
		t.Fatalf("skill failed: %d %s", res.exitCode, res.stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "sub", "SKILL.md")); err != nil {
		t.Errorf("expected sub/SKILL.md to exist: %v", err)
	}
}

func TestUnknownCommand(t *testing.T) {
	res := run(t, t.TempDir(), "bogus")
	if res.exitCode == 0 {
		t.Fatal("expected non-zero exit for unknown command")
	}
	if !strings.Contains(res.stderr, "unknown command") {
		t.Errorf("unexpected stderr: %q", res.stderr)
	}
}
