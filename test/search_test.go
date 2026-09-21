// Package integration_test: CLI-level coverage for `iceman search`. The cases
// that would need a populated nixpkgs catalog are gated on `nix` being present
// and on ICEMAN_TEST_NIX=1, since the first real catalog fetch is slow; the fuzzy ranking
// and cache behaviour are covered by fast unit tests in internal/nixsearch.
package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func requireNixCLI(t *testing.T) {
	t.Helper()
	// A cold full-catalog fetch takes minutes, so these are opt-in rather than
	// part of the default `go test ./...` run.
	if testing.Short() || os.Getenv("ICEMAN_TEST_NIX") != "1" {
		t.Skip("set ICEMAN_TEST_NIX=1 to run tests that invoke the real `nix search`")
	}
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("no `nix` on PATH; skipping nixpkgs search test")
	}
}

// TestSearchNoQueryNonInteractive must fail fast rather than block on input
// when stdout is a pipe, which is how `run` invokes the binary.
func TestSearchNoQueryNonInteractive(t *testing.T) {
	res := run(t, t.TempDir(), "search")
	if res.exitCode == 0 {
		t.Fatalf("expected a non-zero exit, got 0 with stdout:\n%s", res.stdout)
	}
	if !strings.Contains(res.stderr, "no query given") {
		t.Errorf("stderr = %q; want a usage error about a missing query", res.stderr)
	}
}

func TestSearchHelpMentionsSearch(t *testing.T) {
	res := run(t, t.TempDir(), "help")
	if res.exitCode != 0 {
		t.Fatalf("help failed: %d %s", res.exitCode, res.stderr)
	}
	if !strings.Contains(res.stdout, "iceman search") {
		t.Errorf("help output does not document `iceman search`:\n%s", res.stdout)
	}
}

func TestSearchJSON(t *testing.T) {
	requireNixCLI(t)
	res := run(t, t.TempDir(), "search", "--json", "--limit", "5", "ripgrep")
	if res.exitCode != 0 {
		t.Fatalf("search failed: %d %s", res.exitCode, res.stderr)
	}

	var pkgs []struct {
		AttrPath    string `json:"attrPath"`
		PName       string `json:"pname"`
		Version     string `json:"version"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(res.stdout), &pkgs); err != nil {
		t.Fatalf("output is not a JSON array (%v):\n%s", err, res.stdout)
	}
	if len(pkgs) == 0 || len(pkgs) > 5 {
		t.Fatalf("got %d results; want 1..5", len(pkgs))
	}
	if pkgs[0].PName != "ripgrep" {
		t.Errorf("best match = %q; want ripgrep", pkgs[0].PName)
	}
}

func TestSearchPlainText(t *testing.T) {
	requireNixCLI(t)
	res := run(t, t.TempDir(), "search", "--limit", "3", "ripgrep")
	if res.exitCode != 0 {
		t.Fatalf("search failed: %d %s", res.exitCode, res.stderr)
	}
	lines := strings.Split(strings.TrimSpace(res.stdout), "\n")
	if len(lines) > 3 {
		t.Fatalf("--limit 3 printed %d lines", len(lines))
	}
	if !strings.Contains(lines[0], "ripgrep") || !strings.Contains(lines[0], "(") {
		t.Errorf("unexpected line format: %q", lines[0])
	}
}
