// Package integration_test: these tests go beyond the snapshot suite and assert
// that the flake.nix files iceman writes are actually parseable Nix, using the
// real Nix toolchain when it is available on the machine.
package integration_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// nixParserOnce caches the resolved parser command for the whole run.
var nixParser []string

func init() {
	if p, err := exec.LookPath("nix-instantiate"); err == nil {
		nixParser = []string{p, "--parse"}
		return
	}
	if p, err := exec.LookPath("nix"); err == nil {
		// `nix eval --file` would evaluate (and fetch inputs); the stable
		// nix-instantiate interface under `nix` still only parses.
		nixParser = []string{p, "--extra-experimental-features", "nix-command", "eval", "--parse"}
	}
}

// requireNix skips the test when no Nix parser is installed, so the suite still
// runs end-to-end on machines without Nix.
func requireNix(t *testing.T) {
	t.Helper()
	if len(nixParser) == 0 {
		t.Skip("no `nix-instantiate` or `nix` on PATH; skipping Nix syntax validation")
	}
}

// assertValidNix parses path with the Nix toolchain and fails with the parser's
// own diagnostics (plus the offending file) when it is not valid Nix syntax.
// Parsing only: it never evaluates, so no nixpkgs/flake-utils fetch is needed.
func assertValidNix(t *testing.T, path string, context string) {
	t.Helper()
	args := append(append([]string{}, nixParser[1:]...), path)
	cmd := exec.Command(nixParser[0], args...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			contents = []byte("<unreadable: " + readErr.Error() + ">")
		}
		t.Fatalf("%s: generated flake.nix is not valid Nix syntax (%v)\nparser stderr:\n%s\nfile contents:\n%s",
			context, err, errb.String(), contents)
	}
}

func TestInitProducesValidNix(t *testing.T) {
	requireNix(t)
	dir := t.TempDir()
	if res := run(t, dir, "init", "-d", "demo dev environment"); res.exitCode != 0 {
		t.Fatalf("init failed: %d %s", res.exitCode, res.stderr)
	}
	assertValidNix(t, filepath.Join(dir, "flake.nix"), "after `iceman init`")
}

func TestInitWithDepsProducesValidNix(t *testing.T) {
	requireNix(t)
	dir := t.TempDir()
	if res := run(t, dir, "init", "-d", "my project", "-p", "go,gopls,git,ripgrep"); res.exitCode != 0 {
		t.Fatalf("init failed: %d %s", res.exitCode, res.stderr)
	}
	assertValidNix(t, filepath.Join(dir, "flake.nix"), "after `iceman init -p ...`")
}

func TestAddProducesValidNix(t *testing.T) {
	requireNix(t)
	dir := t.TempDir()
	if res := run(t, dir, "init", "-d", "my project", "-p", "go"); res.exitCode != 0 {
		t.Fatalf("init failed: %s", res.stderr)
	}
	if res := run(t, dir, "add", "gopls", "git", "ripgrep", "jq"); res.exitCode != 0 {
		t.Fatalf("add failed: %d %s", res.exitCode, res.stderr)
	}
	assertValidNix(t, filepath.Join(dir, "flake.nix"), "after `iceman add` with several packages")
}

func TestAddToEmptyBuildInputsProducesValidNix(t *testing.T) {
	requireNix(t)
	dir := t.TempDir()
	if res := run(t, dir, "init", "-d", "my project"); res.exitCode != 0 {
		t.Fatalf("init failed: %s", res.stderr)
	}
	assertValidNix(t, filepath.Join(dir, "flake.nix"), "after `iceman init` with no packages")
	if res := run(t, dir, "add", "go"); res.exitCode != 0 {
		t.Fatalf("add failed: %s", res.stderr)
	}
	assertValidNix(t, filepath.Join(dir, "flake.nix"), "after adding the first package to an empty buildInputs list")
}

// TestSequentialAddsStayValidNix simulates real incremental usage: the file must
// be valid Nix after *every* add, not only at the end.
func TestSequentialAddsStayValidNix(t *testing.T) {
	requireNix(t)
	dir := t.TempDir()
	if res := run(t, dir, "init", "-d", "incremental project"); res.exitCode != 0 {
		t.Fatalf("init failed: %s", res.stderr)
	}
	path := filepath.Join(dir, "flake.nix")
	assertValidNix(t, path, "after init")

	steps := [][]string{
		{"go"},
		{"gopls"},
		{"git", "ripgrep"},
		{"go"}, // already present: must be a no-op that leaves a valid file
		{"nodejs_20"},
		{"python3"},
		{"jq", "fd", "bat"},
	}
	for i, pkgs := range steps {
		res := run(t, dir, append([]string{"add"}, pkgs...)...)
		if res.exitCode != 0 {
			t.Fatalf("add step %d (%v) failed: %d %s", i, pkgs, res.exitCode, res.stderr)
		}
		assertValidNix(t, path, "after sequential add step "+strings.Join(pkgs, ","))
	}
}

// TestAddAttributePathPackageProducesValidNix covers packages users really do
// add by attribute path (e.g. `python3Packages.requests`).
func TestAddAttributePathPackageProducesValidNix(t *testing.T) {
	requireNix(t)
	dir := t.TempDir()
	if res := run(t, dir, "init", "-d", "attr path project"); res.exitCode != 0 {
		t.Fatalf("init failed: %s", res.stderr)
	}
	if res := run(t, dir, "add", "python3Packages.requests", "nodePackages.typescript"); res.exitCode != 0 {
		t.Fatalf("add failed: %d %s", res.exitCode, res.stderr)
	}
	assertValidNix(t, filepath.Join(dir, "flake.nix"), "after adding attribute-path packages")
}

// TestInitDescriptionWithQuotesProducesValidNix: the description is interpolated
// straight into a Nix string literal, so quotes/backslashes/`${` must be escaped
// or the resulting file is not parseable.
func TestInitDescriptionWithQuotesProducesValidNix(t *testing.T) {
	requireNix(t)
	for _, desc := range []string{
		`a "quoted" project`,
		`path C:\temp project`,
		`interpolate ${HOME} please`,
	} {
		t.Run(desc, func(t *testing.T) {
			dir := t.TempDir()
			if res := run(t, dir, "init", "-d", desc); res.exitCode != 0 {
				t.Fatalf("init failed: %d %s", res.exitCode, res.stderr)
			}
			assertValidNix(t, filepath.Join(dir, "flake.nix"), "after `iceman init -d "+desc+"`")
		})
	}
}

// TestInitDirNameWithQuoteProducesValidNix: the default description is derived
// from the target directory name, which the user does not control character by
// character; it must still produce parseable Nix.
func TestInitDirNameWithQuoteProducesValidNix(t *testing.T) {
	requireNix(t)
	parent := t.TempDir()
	target := filepath.Join(parent, `my"proj`)
	if res := run(t, parent, "init", target); res.exitCode != 0 {
		t.Fatalf("init failed: %d %s", res.exitCode, res.stderr)
	}
	assertValidNix(t, filepath.Join(target, "flake.nix"), "after `iceman init` into a directory whose name contains a quote")
}
