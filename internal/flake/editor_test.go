package flake

import (
	"strings"
	"testing"
)

func TestAddDepsSkipsDuplicatesAndKeepsIndent(t *testing.T) {
	base, err := Render(Data{Description: "d", Deps: []string{"go"}})
	if err != nil {
		t.Fatal(err)
	}

	out, added, err := AddDeps(base, []string{"go", "git", "git"})
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 || added[0] != "git" {
		t.Fatalf("added = %v, want [git]", added)
	}

	out, _, err = AddDeps(out, []string{"ripgrep"})
	if err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{"            go\n", "            git\n", "            ripgrep\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing consistently indented entry %q in:\n%s", want, out)
		}
	}
	want := "          buildInputs = with pkgs; [\n            go\n            git\n            ripgrep\n          ];\n"
	if !strings.Contains(out, want) {
		t.Errorf("buildInputs block not formatted as expected:\n%s", out)
	}
}

func TestAddDepsNoBlock(t *testing.T) {
	if _, _, err := AddDeps("{ }\n", []string{"go"}); err == nil {
		t.Fatal("expected error when buildInputs block is missing")
	}
}
