// Package picker provides an interactive, type-to-filter package chooser.
package picker

import (
	"github.com/wesbragagt/iceman/internal/nixsearch"
)

// visibleRows is how many result lines the picker draws at once.
const visibleRows = 15

// nameColumn is the minimum width reserved for a package name before its
// description starts, so descriptions line up in a column instead of
// crowding right against whatever name happens to precede them.
const nameColumn = 28

// rankLimit caps how many results are ranked per keystroke; the full nixpkgs
// catalog is ~100k entries and nobody scrolls past a few hundred.
const rankLimit = 200

// Key is a decoded keystroke. Raw terminal bytes are translated into these by
// the terminal layer so the state machine stays free of escape-sequence logic.
type Key int

const (
	KeyRune Key = iota
	KeyUp
	KeyDown
	KeyBackspace
	KeySpace
	KeyEnter
	KeyCancel
	KeyIgnore
)

// Outcome reports what a keystroke did to the picker.
type Outcome int

const (
	// Continue means the picker should redraw and keep reading.
	Continue Outcome = iota
	// Confirmed means the user accepted the current selection.
	Confirmed
	// Cancelled means the user pressed Esc or Ctrl-C.
	Cancelled
)

type pickerState struct {
	all      []nixsearch.Package
	query    []rune
	filtered []nixsearch.Package
	cursor   int
	offset   int
	toggled  map[string]bool
}

func newState(pkgs []nixsearch.Package) *pickerState {
	s := &pickerState{all: pkgs, toggled: map[string]bool{}}
	s.refilter()
	return s
}

func (s *pickerState) refilter() {
	s.filtered = nixsearch.RankPackages(string(s.query), s.all, rankLimit)
	s.clampCursor()
}

func (s *pickerState) clampCursor() {
	if s.cursor >= len(s.filtered) {
		s.cursor = len(s.filtered) - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+visibleRows {
		s.offset = s.cursor - visibleRows + 1
	}
	if s.offset < 0 {
		s.offset = 0
	}
}

// HandleKey applies one decoded keystroke and reports what happened.
func (s *pickerState) HandleKey(k Key, r rune) Outcome {
	switch k {
	case KeyCancel:
		return Cancelled
	case KeyEnter:
		return Confirmed
	case KeyUp:
		if s.cursor > 0 {
			s.cursor--
		}
		s.clampCursor()
	case KeyDown:
		if s.cursor < len(s.filtered)-1 {
			s.cursor++
		}
		s.clampCursor()
	case KeySpace:
		if len(s.filtered) > 0 {
			key := s.filtered[s.cursor].AttrPath
			if s.toggled[key] {
				delete(s.toggled, key)
			} else {
				s.toggled[key] = true
			}
		}
	case KeyBackspace:
		if len(s.query) > 0 {
			s.query = s.query[:len(s.query)-1]
			s.refilter()
		}
	case KeyRune:
		s.query = append(s.query, r)
		s.refilter()
	}
	return Continue
}

// Selection returns the toggled packages, or just the highlighted one when
// nothing is toggled. Toggled packages are returned in catalog order so the
// result does not depend on the order the user pressed space.
func (s *pickerState) Selection() []nixsearch.Package {
	if len(s.toggled) > 0 {
		var out []nixsearch.Package
		for _, p := range s.all {
			if s.toggled[p.AttrPath] {
				out = append(out, p)
			}
		}
		return out
	}
	if len(s.filtered) == 0 {
		return nil
	}
	return []nixsearch.Package{s.filtered[s.cursor]}
}
