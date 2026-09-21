// Package cmd implements the iceman CLI commands.
package cmd

import (
	"fmt"
	"os"
)

const usage = `iceman bootstraps and edits Nix flake dev environments.

Usage:
  iceman init [directory] [-d description] [-p pkg1,pkg2,...] [-i]
      Bootstrap a new flake.nix built on flake-utils' eachDefaultSystem,
      so the resulting dev shell works across Linux and macOS, on both
      x86_64 and aarch64.

  iceman add [package...] [-f flake.nix] [-i]
      Add nixpkgs packages to an existing flake.nix's devShell. With -i, or
      with no packages on a terminal, pick them from an interactive list.

  iceman search [query] [--json] [--limit N] [--refresh]
      Fuzzy-search nixpkgs. With no query on a terminal, opens the same
      interactive picker. Requires nix on PATH.

  iceman skill [directory] [-file AGENTS.md] [-force]
      Write iceman's AGENTS.md skill doc into a project, so Claude Code,
      Codex, Cursor, or any agent that reads AGENTS.md knows to use iceman
      instead of hand-editing flake.nix.

  iceman help
      Show this message.
`

// Execute dispatches to the requested subcommand.
func Execute() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "init":
		err = runInit(os.Args[2:])
	case "add":
		err = runAdd(os.Args[2:])
	case "search":
		err = runSearch(os.Args[2:])
	case "skill":
		err = runSkill(os.Args[2:])
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "iceman: unknown command %q\n\n", os.Args[1])
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "iceman: %v\n", err)
		os.Exit(1)
	}
}
