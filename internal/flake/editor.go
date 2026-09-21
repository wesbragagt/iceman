package flake

import (
	"fmt"
	"regexp"
	"strings"
)

// buildInputsRe matches a `buildInputs = with pkgs; [ ... ];` block,
// capturing the indentation of the opening line and the list body.
var buildInputsRe = regexp.MustCompile(`(?s)([ \t]*)buildInputs\s*=\s*with\s+pkgs;\s*\[(.*?)\]\s*;`)

// AddDeps inserts the given package names into the buildInputs list of an
// existing flake.nix's contents, skipping any that are already present.
// It returns the updated contents and the list of packages actually added.
func AddDeps(contents string, deps []string) (string, []string, error) {
	loc := buildInputsRe.FindStringSubmatchIndex(contents)
	if loc == nil {
		return "", nil, fmt.Errorf("could not find a `buildInputs = with pkgs; [ ... ];` block in flake.nix")
	}

	indent := contents[loc[2]:loc[3]]
	body := contents[loc[4]:loc[5]]

	existing := map[string]bool{}
	for _, f := range strings.Fields(body) {
		existing[f] = true
	}

	var added []string
	var toAppend strings.Builder
	for _, dep := range deps {
		dep = strings.TrimSpace(dep)
		if dep == "" || existing[dep] {
			continue
		}
		toAppend.WriteString(fmt.Sprintf("%s  %s\n", indent, dep))
		existing[dep] = true
		added = append(added, dep)
	}

	if len(added) == 0 {
		return contents, added, nil
	}

	newBody := strings.TrimRight(body, " \t")
	if !strings.HasSuffix(newBody, "\n") {
		newBody += "\n"
	}
	newBody += toAppend.String() + indent

	newBlock := fmt.Sprintf("%sbuildInputs = with pkgs; [%s]%s;", indent, newBody, "")

	updated := contents[:loc[0]] + newBlock + contents[loc[1]:]
	return updated, added, nil
}
