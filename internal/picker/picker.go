package picker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/wesbragagt/iceman/internal/nixsearch"
	"golang.org/x/term"
)

// ErrCancelled is returned when the user aborts the picker with Esc or Ctrl-C.
// Callers should treat it as a clean no-op, not a failure.
var ErrCancelled = errors.New("selection cancelled")

// Pick renders a live, type-to-filter list on the terminal and returns the
// packages the user chose. It returns ErrCancelled if the user aborts.
func Pick(pkgs []nixsearch.Package) ([]nixsearch.Package, error) {
	// The TTY is opened directly so the picker still works when stdin is a
	// pipe (e.g. `echo x | iceman add -i`).
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("opening terminal: %w", err)
	}
	defer tty.Close()

	fd := int(tty.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("entering raw terminal mode: %w", err)
	}
	// Runs on every exit path, including panics.
	defer func() {
		_ = term.Restore(fd, oldState)
		fmt.Fprint(tty, "\r\n")
	}()

	s := newState(pkgs)
	drawn := 0
	buf := make([]byte, 16)
	th := newTheme(os.Getenv("NO_COLOR") != "")

	for {
		drawn = render(tty, s, drawn, th)

		n, err := tty.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil, ErrCancelled
			}
			return nil, fmt.Errorf("reading terminal input: %w", err)
		}

		k, r := decode(buf[:n])
		switch s.HandleKey(k, r) {
		case Cancelled:
			clearFrame(tty, drawn)
			return nil, ErrCancelled
		case Confirmed:
			clearFrame(tty, drawn)
			return s.Selection(), nil
		}
	}
}

// decode translates a raw read from the terminal into a single Key. Only the
// first keystroke in the buffer is honoured, which is plenty at human typing
// speed and keeps arrow-key escape sequences unambiguous.
func decode(b []byte) (Key, rune) {
	if len(b) == 0 {
		return KeyIgnore, 0
	}
	if len(b) >= 3 && b[0] == 0x1b && b[1] == '[' {
		switch b[2] {
		case 'A':
			return KeyUp, 0
		case 'B':
			return KeyDown, 0
		}
		return KeyIgnore, 0
	}
	switch b[0] {
	case 0x1b: // bare Esc
		return KeyCancel, 0
	case 0x03: // Ctrl-C
		return KeyCancel, 0
	case '\r', '\n':
		return KeyEnter, 0
	case 0x7f, 0x08:
		return KeyBackspace, 0
	case ' ':
		return KeySpace, 0
	case 0x0e: // Ctrl-N
		return KeyDown, 0
	case 0x10: // Ctrl-P
		return KeyUp, 0
	}
	if b[0] < 0x20 {
		return KeyIgnore, 0
	}
	return KeyRune, []rune(string(b))[0]
}

// theme holds the ANSI codes the picker draws with. Every field is "" when
// colors are disabled, so callers can splice them in unconditionally.
type theme struct {
	reset, dim, bold, cyan, yellow, green, gray string
}

func newTheme(disabled bool) theme {
	if disabled {
		return theme{}
	}
	return theme{
		reset:  "\x1b[0m",
		dim:    "\x1b[2m",
		bold:   "\x1b[1m",
		cyan:   "\x1b[36m",
		yellow: "\x1b[33m",
		green:  "\x1b[32m",
		gray:   "\x1b[90m",
	}
}

// render redraws the picker and returns the number of lines it drew, so the
// next frame knows how far up to move the cursor.
func render(w io.Writer, s *pickerState, prev int, th theme) int {
	var b strings.Builder
	if prev > 1 {
		fmt.Fprintf(&b, "\x1b[%dA", prev-1)
	}
	b.WriteString("\r\x1b[J")

	fmt.Fprintf(&b, "%ssearch>%s %s\r\n", th.bold, th.reset, string(s.query))
	lines := 1

	end := min(s.offset+visibleRows, len(s.filtered))
	for i := s.offset; i < end; i++ {
		p := s.filtered[i]
		mark, markColor := "[ ]", ""
		if s.toggled[p.AttrPath] {
			mark, markColor = "[x]", th.green
		}
		name := highlightName(p.PName, s.query, th)
		pad := strings.Repeat(" ", max(nameColumn-len([]rune(p.PName)), 1))
		desc := fmt.Sprintf("%s%s%s", th.gray, truncate(p.Description, 60), th.reset)
		cursor, nameStyle := "  ", ""
		if i == s.cursor {
			cursor = th.bold + th.cyan + "❯ " + th.reset
			nameStyle = th.bold
		}
		fmt.Fprintf(&b, "%s%s%s%s%s  %s%s%s%s\x1b[K\r\n", cursor, markColor, mark, th.reset, nameStyle, name, th.reset, pad, desc)
		lines++
	}

	fmt.Fprintf(&b, "%s%d match(es) · type to filter · ↑/↓ move · space toggle · enter confirm · esc cancel%s", th.dim, len(s.filtered), th.reset)
	lines++

	fmt.Fprint(w, b.String())
	return lines
}

// highlightName renders name with the characters that matched query in bold
// yellow, so a scanning eye can see why a result surfaced.
func highlightName(name string, query []rune, th theme) string {
	if len(query) == 0 || th.yellow == "" {
		return name
	}
	positions := nixsearch.MatchPositions(string(query), name)
	if positions == nil {
		return name
	}
	hit := make(map[int]bool, len(positions))
	for _, p := range positions {
		hit[p] = true
	}

	var b strings.Builder
	on := false
	for i := 0; i < len(name); i++ {
		if hit[i] && !on {
			b.WriteString(th.bold)
			b.WriteString(th.yellow)
			on = true
		} else if !hit[i] && on {
			b.WriteString(th.reset)
			on = false
		}
		b.WriteByte(name[i])
	}
	if on {
		b.WriteString(th.reset)
	}
	return b.String()
}

func clearFrame(w io.Writer, drawn int) {
	if drawn > 0 {
		fmt.Fprintf(w, "\x1b[%dA", drawn-1)
	}
	fmt.Fprint(w, "\r\x1b[J")
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
