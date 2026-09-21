package flake

import (
	"bytes"
	"strings"
	"text/template"
)

const flakeTemplate = `{
  description = "{{escapeNixString .Description}}";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
{{range .Deps}}            {{.}}
{{end}}          ];
        };
      }
    );
}
`

// escapeNixString escapes a value so it can be embedded safely inside a
// double-quoted Nix string literal. Nix treats `\` as an escape character,
// `"` as the string terminator, and `${` as the start of an antiquotation, so
// all three must be neutralized. `$` is escaped unconditionally (`\$` is a
// literal `$` in Nix), which is simpler and safer than only escaping `${`.
func escapeNixString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\', '"', '$':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Data holds the values used to render a new flake.nix.
type Data struct {
	Description string
	Deps        []string
}

// Render produces the contents of a new flake.nix file.
func Render(d Data) (string, error) {
	tmpl, err := template.New("flake").Funcs(template.FuncMap{"escapeNixString": escapeNixString}).Parse(flakeTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, d); err != nil {
		return "", err
	}
	return buf.String(), nil
}
