package picker

import (
	"testing"

	"github.com/wesbragagt/iceman/internal/nixsearch"
)

func pkgs() []nixsearch.Package {
	return []nixsearch.Package{
		{AttrPath: "a.ripgrep", PName: "ripgrep", Description: "fast search"},
		{AttrPath: "a.ripgrep-all", PName: "ripgrep-all", Description: "search pdfs"},
		{AttrPath: "a.fd", PName: "fd", Description: "find alternative"},
		{AttrPath: "a.go", PName: "go", Description: "go language"},
	}
}

func typeQuery(s *pickerState, q string) {
	for _, r := range q {
		s.HandleKey(KeyRune, r)
	}
}

func names(ps []nixsearch.Package) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.PName
	}
	return out
}

func TestInitialStateShowsEverything(t *testing.T) {
	s := newState(pkgs())
	if len(s.filtered) != 4 {
		t.Fatalf("filtered = %d; want 4", len(s.filtered))
	}
	if s.cursor != 0 {
		t.Errorf("cursor = %d; want 0", s.cursor)
	}
}

func TestTypingFilters(t *testing.T) {
	s := newState(pkgs())
	typeQuery(s, "ripg")
	if len(s.filtered) != 2 {
		t.Fatalf("filtered = %v; want the two ripgrep entries", names(s.filtered))
	}
	if s.filtered[0].PName != "ripgrep" {
		t.Errorf("best match = %q; want ripgrep", s.filtered[0].PName)
	}
}

func TestBackspaceWidensFilter(t *testing.T) {
	s := newState(pkgs())
	typeQuery(s, "ripg")
	for i := 0; i < 4; i++ {
		s.HandleKey(KeyBackspace, 0)
	}
	if len(s.filtered) != 4 {
		t.Fatalf("filtered = %d after backspacing the query away; want 4", len(s.filtered))
	}
	// Backspacing an empty query must not panic or underflow.
	s.HandleKey(KeyBackspace, 0)
	if len(s.query) != 0 {
		t.Errorf("query = %q; want empty", string(s.query))
	}
}

func TestArrowKeysStayInBounds(t *testing.T) {
	s := newState(pkgs())
	s.HandleKey(KeyUp, 0)
	if s.cursor != 0 {
		t.Errorf("up at top moved cursor to %d", s.cursor)
	}
	for i := 0; i < 10; i++ {
		s.HandleKey(KeyDown, 0)
	}
	if s.cursor != 3 {
		t.Errorf("cursor = %d; want 3 (last item)", s.cursor)
	}
	s.HandleKey(KeyUp, 0)
	if s.cursor != 2 {
		t.Errorf("cursor = %d; want 2", s.cursor)
	}
}

func TestFilteringClampsCursor(t *testing.T) {
	s := newState(pkgs())
	s.HandleKey(KeyDown, 0)
	s.HandleKey(KeyDown, 0)
	s.HandleKey(KeyDown, 0)
	typeQuery(s, "ripgrep")
	if s.cursor >= len(s.filtered) {
		t.Fatalf("cursor %d out of range for %d filtered rows", s.cursor, len(s.filtered))
	}
}

func TestEnterWithNothingToggledSelectsHighlighted(t *testing.T) {
	s := newState(pkgs())
	s.HandleKey(KeyDown, 0)
	if got := s.HandleKey(KeyEnter, 0); got != Confirmed {
		t.Fatalf("enter returned %v; want Confirmed", got)
	}
	sel := s.Selection()
	if len(sel) != 1 || sel[0].AttrPath != s.filtered[1].AttrPath {
		t.Fatalf("selection = %v; want the highlighted row", names(sel))
	}
}

func TestSpaceTogglesAndEnterReturnsToggledSet(t *testing.T) {
	s := newState(pkgs())
	s.HandleKey(KeySpace, 0) // ripgrep... whatever is first
	first := s.filtered[0].AttrPath
	s.HandleKey(KeyDown, 0)
	s.HandleKey(KeySpace, 0)
	second := s.filtered[1].AttrPath

	// Move the cursor elsewhere: toggled items win over the highlight.
	s.HandleKey(KeyDown, 0)

	if got := s.HandleKey(KeyEnter, 0); got != Confirmed {
		t.Fatal("expected Confirmed")
	}
	sel := s.Selection()
	if len(sel) != 2 {
		t.Fatalf("selection = %v; want 2 toggled items", names(sel))
	}
	got := map[string]bool{sel[0].AttrPath: true, sel[1].AttrPath: true}
	if !got[first] || !got[second] {
		t.Errorf("selection %v missing %q/%q", names(sel), first, second)
	}
}

func TestSpaceTogglesOff(t *testing.T) {
	s := newState(pkgs())
	s.HandleKey(KeySpace, 0)
	s.HandleKey(KeySpace, 0)
	if len(s.toggled) != 0 {
		t.Fatalf("toggled = %v; want empty after toggling twice", s.toggled)
	}
	sel := s.Selection()
	if len(sel) != 1 {
		t.Errorf("selection = %v; want the highlighted fallback", names(sel))
	}
}

func TestSpaceOnEmptyFilterIsNoop(t *testing.T) {
	s := newState(pkgs())
	typeQuery(s, "zzzzzz")
	if len(s.filtered) != 0 {
		t.Fatalf("expected no matches, got %v", names(s.filtered))
	}
	s.HandleKey(KeySpace, 0)
	if got := s.HandleKey(KeyEnter, 0); got != Confirmed {
		t.Fatal("expected Confirmed")
	}
	if sel := s.Selection(); len(sel) != 0 {
		t.Errorf("selection = %v; want empty", names(sel))
	}
}

func TestCancel(t *testing.T) {
	s := newState(pkgs())
	if got := s.HandleKey(KeyCancel, 0); got != Cancelled {
		t.Errorf("cancel returned %v; want Cancelled", got)
	}
}

func TestScrollOffsetFollowsCursor(t *testing.T) {
	var many []nixsearch.Package
	for i := 0; i < 40; i++ {
		many = append(many, nixsearch.Package{AttrPath: string(rune('a'+i%26)) + string(rune('0'+i/26)), PName: "pkg" + string(rune('a'+i%26)) + string(rune('0'+i/26))})
	}
	s := newState(many)
	for i := 0; i < 20; i++ {
		s.HandleKey(KeyDown, 0)
	}
	if s.cursor < s.offset || s.cursor >= s.offset+visibleRows {
		t.Fatalf("cursor %d outside visible window [%d,%d)", s.cursor, s.offset, s.offset+visibleRows)
	}
	for i := 0; i < 30; i++ {
		s.HandleKey(KeyUp, 0)
	}
	if s.cursor != 0 || s.offset != 0 {
		t.Fatalf("cursor=%d offset=%d; want both 0 at the top", s.cursor, s.offset)
	}
}

func TestDecodeKeys(t *testing.T) {
	cases := []struct {
		in   []byte
		want Key
	}{
		{[]byte{0x1b, '[', 'A'}, KeyUp},
		{[]byte{0x1b, '[', 'B'}, KeyDown},
		{[]byte{0x1b}, KeyCancel},
		{[]byte{0x03}, KeyCancel},
		{[]byte{'\r'}, KeyEnter},
		{[]byte{0x7f}, KeyBackspace},
		{[]byte{' '}, KeySpace},
		{[]byte("g"), KeyRune},
		{nil, KeyIgnore},
	}
	for _, c := range cases {
		if got, _ := decode(c.in); got != c.want {
			t.Errorf("decode(%v) = %v; want %v", c.in, got, c.want)
		}
	}
	if k, r := decode([]byte("x")); k != KeyRune || r != 'x' {
		t.Errorf("decode(x) = %v,%q", k, r)
	}
}
