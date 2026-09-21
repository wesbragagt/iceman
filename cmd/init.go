package cmd

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wesbragagt/iceman/internal/flake"
)

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	description := fs.String("description", "", "description for the flake")
	fs.StringVar(description, "d", "", "description for the flake (shorthand)")
	deps := fs.String("deps", "", "comma-separated list of nixpkgs packages to include")
	fs.StringVar(deps, "p", "", "comma-separated list of nixpkgs packages to include (shorthand)")
	direnv := fs.Bool("direnv", true, "also create a .envrc that runs 'use flake' (requires direnv)")
	interactive := fs.Bool("interactive", false, "pick the initial packages from a fuzzy-searchable nixpkgs list instead of -p (requires nix on PATH)")
	fs.BoolVar(interactive, "i", false, "pick the initial packages interactively (shorthand)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *interactive && *deps != "" {
		return fmt.Errorf("-p/--deps and -i/--interactive are mutually exclusive")
	}

	dir := "."
	if fs.NArg() == 1 {
		dir = fs.Arg(0)
	} else if fs.NArg() > 1 {
		return fmt.Errorf("expected at most one directory argument")
	}

	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	depList := normalizeDeps(*deps)
	if *interactive {
		picked, err := pickDeps()
		if err != nil {
			return err
		}
		depList = picked
	}

	flakePath := filepath.Join(dir, "flake.nix")
	if _, err := os.Stat(flakePath); err == nil {
		return mergeIntoExistingFlake(flakePath, dir, depList, *direnv)
	}

	envrcPath := filepath.Join(dir, ".envrc")
	if *direnv {
		if _, err := os.Stat(envrcPath); err == nil {
			return fmt.Errorf("%s already exists", envrcPath)
		}
	}

	desc := *description
	if desc == "" {
		name := filepath.Base(dir)
		if name == "." || name == "" {
			if wd, err := os.Getwd(); err == nil {
				name = filepath.Base(wd)
			} else {
				name = "dev environment"
			}
		}
		desc = fmt.Sprintf("%s dev environment", name)
	}

	contents, err := flake.Render(flake.Data{
		Description: desc,
		Deps:        depList,
	})
	if err != nil {
		return err
	}

	if err := os.WriteFile(flakePath, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", flakePath, err)
	}
	fmt.Printf("Created %s\n", flakePath)

	if *direnv {
		if err := os.WriteFile(envrcPath, []byte("use flake\n"), 0o644); err != nil {
			return fmt.Errorf("writing %s: %w", envrcPath, err)
		}
		fmt.Printf("Created %s\n", envrcPath)

		if direnvBin, err := exec.LookPath("direnv"); err == nil {
			cmd := exec.Command(direnvBin, "allow", dir)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "warning: direnv allow failed: %v\n", err)
			}
		} else {
			fmt.Println("direnv not found on PATH; run 'direnv allow' after installing it")
		}
	}

	return nil
}

// mergeIntoExistingFlake handles `iceman init` on a directory that already
// has a flake.nix: rather than erroring out, it behaves like `iceman add`,
// merging any requested deps into the existing buildInputs and leaving the
// file's description and structure untouched.
func mergeIntoExistingFlake(flakePath, dir string, deps []string, direnv bool) error {
	fmt.Printf("%s already exists; merging into it instead of creating a new one\n", flakePath)

	if len(deps) > 0 {
		raw, err := os.ReadFile(flakePath)
		if err != nil {
			return fmt.Errorf("reading %s: %w", flakePath, err)
		}

		updated, added, err := flake.AddDeps(string(raw), deps)
		if err != nil {
			return err
		}

		if len(added) > 0 {
			if err := os.WriteFile(flakePath, []byte(updated), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", flakePath, err)
			}
			fmt.Printf("Added to %s: %s\n", flakePath, strings.Join(added, ", "))
		} else {
			fmt.Println("No new packages to add; all already present.")
		}
	}

	envrcPath := filepath.Join(dir, ".envrc")
	if direnv {
		if _, err := os.Stat(envrcPath); err != nil {
			if err := os.WriteFile(envrcPath, []byte("use flake\n"), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", envrcPath, err)
			}
			fmt.Printf("Created %s\n", envrcPath)

			if direnvBin, err := exec.LookPath("direnv"); err == nil {
				cmd := exec.Command(direnvBin, "allow", dir)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					fmt.Fprintf(os.Stderr, "warning: direnv allow failed: %v\n", err)
				}
			} else {
				fmt.Println("direnv not found on PATH; run 'direnv allow' after installing it")
			}
		}
	}

	return nil
}

func normalizeDeps(raw string) []string {
	var out []string
	for part := range strings.SplitSeq(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
