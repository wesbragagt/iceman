package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// SkillContent is the AGENTS.md content `iceman skill` writes into a project.
// main sets it from the embedded copy of this repo's own AGENTS.md, so
// there's one source of truth for iceman's own doc and what gets installed
// elsewhere.
var SkillContent string

func runSkill(args []string) error {
	fs := flag.NewFlagSet("skill", flag.ExitOnError)
	file := fs.String("file", "AGENTS.md", "name of the skill file to write")
	force := fs.Bool("force", false, "overwrite the file if it already exists")
	if err := fs.Parse(args); err != nil {
		return err
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

	path := filepath.Join(dir, *file)
	if !*force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use -force to overwrite)", path)
		}
	}

	if err := os.WriteFile(path, []byte(SkillContent), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	fmt.Printf("Created %s\n", path)
	return nil
}
