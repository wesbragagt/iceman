// Package nixsearch fetches and caches the nixpkgs package catalog.
package nixsearch

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Package is a single nixpkgs attribute as reported by `nix search`.
type Package struct {
	AttrPath    string `json:"attrPath"`
	PName       string `json:"pname"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

// CacheTTL is how long a cached catalog is considered fresh. The first full
// catalog fetch is slow, so the window is deliberately generous.
const CacheTTL = 24 * time.Hour

const cacheFile = "nixpkgs-search.json"

// Runner produces the raw `nix search` JSON. It exists so tests can supply
// fixture bytes without invoking Nix.
type Runner interface {
	Output() ([]byte, error)
}

// Options configures Fetch.
type Options struct {
	// Refresh forces a re-run of the runner even if the cache is fresh.
	Refresh bool
	// CacheDir overrides the default cache location; tests always set it.
	CacheDir string
	// Runner overrides the real `nix search` invocation.
	Runner Runner
}

type nixRunner struct{}

func (nixRunner) Output() ([]byte, error) {
	bin, err := exec.LookPath("nix")
	if err != nil {
		return nil, fmt.Errorf("nix not found on PATH: install Nix to use iceman search: %w", err)
	}
	// `^` is nix's idiom for "match everything"; a literally empty query is
	// rejected by the POSIX regex grammar nix uses.
	cmd := exec.Command(bin, "--extra-experimental-features", "nix-command flakes", "search", "nixpkgs", "^", "--json")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running `nix search nixpkgs`: %w", err)
	}
	return out, nil
}

// DefaultCacheDir resolves $XDG_CACHE_HOME/iceman, falling back to the
// platform user cache directory.
func DefaultCacheDir() (string, error) {
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "iceman"), nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("resolving cache directory: %w", err)
	}
	return filepath.Join(base, "iceman"), nil
}

// Fetch returns the nixpkgs catalog, reading a fresh on-disk cache when
// possible and otherwise running `nix search` and repopulating the cache.
func Fetch(opts Options) ([]Package, error) {
	dir := opts.CacheDir
	if dir == "" {
		d, err := DefaultCacheDir()
		if err != nil {
			return nil, err
		}
		dir = d
	}
	path := filepath.Join(dir, cacheFile)

	if !opts.Refresh {
		if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) < CacheTTL {
			if raw, err := os.ReadFile(path); err == nil {
				if pkgs, err := parse(raw); err == nil {
					return pkgs, nil
				}
			}
		}
	}

	runner := opts.Runner
	if runner == nil {
		runner = nixRunner{}
	}
	raw, err := runner.Output()
	if err != nil {
		return nil, err
	}

	pkgs, err := parse(raw)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dir, 0o755); err == nil {
		// A cache write failure must not fail the search.
		_ = os.WriteFile(path, raw, 0o644)
	}
	return pkgs, nil
}

func parse(raw []byte) ([]Package, error) {
	var entries map[string]struct {
		PName       string `json:"pname"`
		Version     string `json:"version"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parsing `nix search` JSON: %w", err)
	}

	pkgs := make([]Package, 0, len(entries))
	for attr, e := range entries {
		name := e.PName
		if name == "" {
			name = shortName(attr)
		}
		pkgs = append(pkgs, Package{AttrPath: attr, PName: name, Version: e.Version, Description: e.Description})
	}
	// Map iteration order is random; sort so results are reproducible.
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].AttrPath < pkgs[j].AttrPath })
	return pkgs, nil
}

// shortName strips the `legacyPackages.<system>.` prefix `nix search` uses.
func shortName(attr string) string {
	if i := strings.LastIndex(attr, "."); i >= 0 {
		return attr[i+1:]
	}
	return attr
}

// InstallName is the name to write into buildInputs: the attribute path with
// its `legacyPackages.<system>.` / `packages.<system>.` prefix removed, since a
// flake's `pkgs` is already that set.
func (p Package) InstallName() string {
	parts := strings.Split(p.AttrPath, ".")
	if len(parts) > 2 && (parts[0] == "legacyPackages" || parts[0] == "packages") {
		return strings.Join(parts[2:], ".")
	}
	return p.AttrPath
}
