package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/wesbragagt/iceman/internal/nixsearch"
	"github.com/wesbragagt/iceman/internal/picker"
	"golang.org/x/term"
)

const defaultSearchLimit = 20

func runSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "print results as a JSON array")
	limit := fs.Int("limit", defaultSearchLimit, "maximum number of results to print")
	refresh := fs.Bool("refresh", false, "refresh the cached nixpkgs catalog before searching")
	if err := fs.Parse(args); err != nil {
		return err
	}

	query := strings.Join(fs.Args(), " ")
	if query == "" {
		if !isTerminal() {
			return fmt.Errorf("no query given; usage: iceman search <query>")
		}
		pkgs, err := fetchCatalog(*refresh)
		if err != nil {
			return err
		}
		selected, err := pick(pkgs)
		if err != nil || len(selected) == 0 {
			return err
		}
		for _, p := range selected {
			fmt.Println(p.InstallName())
		}
		return nil
	}

	pkgs, err := fetchCatalog(*refresh)
	if err != nil {
		return err
	}
	results := nixsearch.RankPackages(query, pkgs, *limit)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if results == nil {
			results = []nixsearch.Package{}
		}
		return enc.Encode(results)
	}

	for _, p := range results {
		fmt.Printf("%s  (%s)  — %s\n", p.PName, p.AttrPath, truncate(p.Description, 80))
	}
	return nil
}

func fetchCatalog(refresh bool) ([]nixsearch.Package, error) {
	return nixsearch.Fetch(nixsearch.Options{Refresh: refresh})
}

// pick runs the interactive picker, translating a user cancellation into an
// empty selection with no error so callers can treat it as a clean no-op.
func pick(pkgs []nixsearch.Package) ([]nixsearch.Package, error) {
	selected, err := picker.Pick(pkgs)
	if err != nil {
		if errors.Is(err, picker.ErrCancelled) {
			return nil, nil
		}
		return nil, err
	}
	return selected, nil
}

// pickDeps fetches the catalog and returns the buildInputs names the user
// selected. A cancelled picker yields an empty slice and no error.
func pickDeps() ([]string, error) {
	pkgs, err := fetchCatalog(false)
	if err != nil {
		return nil, err
	}
	selected, err := pick(pkgs)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, p := range selected {
		names = append(names, p.InstallName())
	}
	return names, nil
}

func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
