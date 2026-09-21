package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wesbragagt/iceman/internal/flake"
)

func runAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	file := fs.String("file", "", "path to the flake.nix file to edit (defaults to the nearest flake.nix, searched from the current directory upward)")
	fs.StringVar(file, "f", "", "path to the flake.nix file to edit (shorthand)")
	interactive := fs.Bool("interactive", false, "pick packages from a fuzzy-searchable nixpkgs list (requires nix on PATH); any positional packages are added too")
	fs.BoolVar(interactive, "i", false, "pick packages interactively (shorthand)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// With no packages and a terminal to draw on, fall into the picker rather
	// than erroring; piped/redirected runs keep the old error so nothing hangs.
	if fs.NArg() == 0 && !*interactive {
		if !isTerminal() {
			return fmt.Errorf("no packages given; usage: iceman add <package> [package...]")
		}
		*interactive = true
	}

	deps := normalizeDeps(strings.Join(fs.Args(), ","))

	if *interactive {
		picked, err := pickDeps()
		if err != nil {
			return err
		}
		deps = append(deps, picked...)
		if len(deps) == 0 {
			fmt.Println("Nothing selected.")
			return nil
		}
	}

	if len(deps) == 0 {
		return fmt.Errorf("no packages given")
	}

	path := *file
	if path == "" {
		found, err := findFlake()
		if err != nil {
			return err
		}
		path = found
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	updated, added, err := flake.AddDeps(string(raw), deps)
	if err != nil {
		return err
	}

	if len(added) == 0 {
		fmt.Println("No new packages to add; all already present.")
		return nil
	}

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	fmt.Printf("Added to %s: %s\n", path, strings.Join(added, ", "))
	return nil
}

// findFlake searches the current directory and its ancestors for a
// flake.nix, the way `git` finds the nearest `.git`.
func findFlake() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}

	for {
		candidate := filepath.Join(dir, "flake.nix")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no flake.nix found in %s or any parent directory", dir)
		}
		dir = parent
	}
}
