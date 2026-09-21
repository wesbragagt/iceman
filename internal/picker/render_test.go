package picker

import (
	"strings"
	"testing"
)

func TestNewThemeDisabledIsAllEmpty(t *testing.T) {
	th := newTheme(true)
	if th != (theme{}) {
		t.Errorf("disabled theme = %+v; want zero value", th)
	}
}

func TestHighlightNameNoColorReturnsPlain(t *testing.T) {
	th := newTheme(true)
	if got := highlightName("ripgrep", []rune("rip"), th); got != "ripgrep" {
		t.Errorf("highlightName with disabled theme = %q; want plain name", got)
	}
}

func TestHighlightNameEmptyQueryReturnsPlain(t *testing.T) {
	th := newTheme(false)
	if got := highlightName("ripgrep", nil, th); got != "ripgrep" {
		t.Errorf("highlightName with empty query = %q; want plain name", got)
	}
}

func TestHighlightNameWrapsMatchedRunes(t *testing.T) {
	th := newTheme(false)
	got := highlightName("ripgrep", []rune("rg"), th)
	if !strings.Contains(got, th.yellow) {
		t.Errorf("highlightName(%q) = %q; want it to contain the yellow escape", "rg", got)
	}
	if !strings.Contains(got, th.reset) {
		t.Errorf("highlightName(%q) = %q; want it to contain a reset", "rg", got)
	}
	// Stripping every escape code should recover the original name.
	stripped := got
	for _, code := range []string{th.bold, th.yellow, th.reset} {
		stripped = strings.ReplaceAll(stripped, code, "")
	}
	if stripped != "ripgrep" {
		t.Errorf("highlighted name strips down to %q; want %q", stripped, "ripgrep")
	}
}

func TestHighlightNameNoMatchReturnsPlain(t *testing.T) {
	th := newTheme(false)
	if got := highlightName("ripgrep", []rune("zzz"), th); got != "ripgrep" {
		t.Errorf("highlightName with no match = %q; want plain name", got)
	}
}
