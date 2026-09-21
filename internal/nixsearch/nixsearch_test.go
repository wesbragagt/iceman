package nixsearch

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const fixtureJSON = `{
  "legacyPackages.x86_64-linux.ripgrep": {"pname": "ripgrep", "version": "14.1.0", "description": "Utility that combines the usability of The Silver Searcher with the raw speed of grep"},
  "legacyPackages.x86_64-linux.fd": {"pname": "fd", "version": "9.0.0", "description": "Simple, fast and user-friendly alternative to find"},
  "legacyPackages.x86_64-linux.weird": {"pname": "", "version": "1", "description": ""}
}`

type fakeRunner struct {
	out   []byte
	err   error
	calls int
}

func (f *fakeRunner) Output() ([]byte, error) {
	f.calls++
	return f.out, f.err
}

func TestFetchParsesAndCaches(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{out: []byte(fixtureJSON)}

	pkgs, err := Fetch(Options{CacheDir: dir, Runner: r})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(pkgs) != 3 {
		t.Fatalf("got %d packages, want 3", len(pkgs))
	}
	if pkgs[0].AttrPath != "legacyPackages.x86_64-linux.fd" || pkgs[0].PName != "fd" || pkgs[0].Version != "9.0.0" {
		t.Errorf("unexpected first package: %+v", pkgs[0])
	}
	// A missing pname falls back to the last attr path segment.
	if pkgs[2].PName != "weird" {
		t.Errorf("pname fallback = %q; want weird", pkgs[2].PName)
	}

	if _, err := os.Stat(filepath.Join(dir, cacheFile)); err != nil {
		t.Fatalf("cache file not written: %v", err)
	}

	if _, err := Fetch(Options{CacheDir: dir, Runner: r}); err != nil {
		t.Fatalf("second Fetch: %v", err)
	}
	if r.calls != 1 {
		t.Errorf("runner called %d times; fresh cache should have been reused", r.calls)
	}
}

func TestFetchRefreshBypassesCache(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{out: []byte(fixtureJSON)}
	if _, err := Fetch(Options{CacheDir: dir, Runner: r}); err != nil {
		t.Fatal(err)
	}
	if _, err := Fetch(Options{CacheDir: dir, Runner: r, Refresh: true}); err != nil {
		t.Fatal(err)
	}
	if r.calls != 2 {
		t.Errorf("runner called %d times; want 2 with Refresh", r.calls)
	}
}

func TestFetchStaleCacheRefetches(t *testing.T) {
	dir := t.TempDir()
	r := &fakeRunner{out: []byte(fixtureJSON)}
	if _, err := Fetch(Options{CacheDir: dir, Runner: r}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * CacheTTL)
	if err := os.Chtimes(filepath.Join(dir, cacheFile), old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := Fetch(Options{CacheDir: dir, Runner: r}); err != nil {
		t.Fatal(err)
	}
	if r.calls != 2 {
		t.Errorf("runner called %d times; stale cache should refetch", r.calls)
	}
}

func TestFetchRunnerErrorPropagates(t *testing.T) {
	sentinel := errors.New("boom")
	_, err := Fetch(Options{CacheDir: t.TempDir(), Runner: &fakeRunner{err: sentinel}})
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v; want wrapped sentinel", err)
	}
}

func TestFetchBadJSON(t *testing.T) {
	_, err := Fetch(Options{CacheDir: t.TempDir(), Runner: &fakeRunner{out: []byte("not json")}})
	if err == nil {
		t.Fatal("expected a parse error")
	}
}
