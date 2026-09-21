# iceman

Get a cross-platform Nix dev shell for a project in one command, and add packages to it later without editing Nix.

`iceman init` writes a `flake.nix` (built on `flake-utils`, so it works on Linux and macOS, x86_64 and aarch64) plus a `.envrc` for direnv. `iceman add` finds the nearest `flake.nix` and appends packages to its `buildInputs`. `iceman search` fuzzy-searches nixpkgs, in a terminal picker or as plain text.

## Why

Nix's language is a real barrier: correct flake syntax, `flake-utils` boilerplate, and exact nixpkgs attribute names are not things you want to learn just to get a shell with `go` and `git` in it. That's true whether you've never written a line of Nix, or you've written plenty and are just tired of retyping the same `mkShell` scaffolding.

`iceman` skips the language entirely. `init` and `add` are text operations over a known template, not a Nix evaluator, so they work without Nix installed at all. `search` handles the other hard part, not knowing the exact nixpkgs attribute name, by fuzzy-matching against the real package catalog instead of making you guess or browse search.nixos.org. The result is a normal, hand-editable `flake.nix` — iceman gets you there fast, then gets out of the way.

### Why Nix matters more in the AI age

Coding agents (Claude Code, Cursor, Devin, CI-run agents) live or die by whether "it works on my machine" also works on theirs. A `flake.nix` pins the *entire* toolchain — compilers, CLIs, system libraries — as a hash-locked dependency graph, not just the app's own packages. That closes the gap that `pip install`, `npm install`, or a README's "brew install these things" instructions leave open: an agent (or a human) starting from a clean checkout gets byte-for-byte the same environment every time, on Linux or macOS, without hallucinating a package name or version that half-installs and silently breaks something three steps later.

That determinism is also what makes an environment safe to hand to an agent unsupervised. `nix develop` activates a read-only, reproducible shell — an agent can run builds and tests inside it without mutating the base toolchain, and if a change to the environment goes wrong, `flake.nix` is a diffable, revertible manifest instead of an ad-hoc pile of shell history. The same flake that gives a human `direnv`-powered auto-loading gives CI and agent sandboxes the identical, one-command setup — no bespoke Dockerfile, no drift between "how I develop" and "how the agent develops."

`iceman` exists so getting that reproducibility doesn't require first learning Nix's language. One command gets a project to that state; `add` and `search` keep it there without hand-editing Nix syntax.

## Try it

No install needed:

```bash
nix run github:wesbragagt/iceman -- init -p go,gopls,git myproject
cd myproject
nix develop
```

Later, from anywhere inside the project:

```bash
iceman add ripgrep
```

## Install

```bash
nix profile install github:wesbragagt/iceman
```

Or with Go: `go build` in a clone, then put `./iceman` on PATH.

## Usage

```
iceman init [directory] [-d description] [-p pkg1,pkg2] [-i] [-direnv=false]
iceman add  [package...] [-f flake.nix] [-i]
iceman search [query] [--json] [--limit N] [--refresh]
iceman skill [directory] [-file AGENTS.md] [-force]
```

`init` creates `flake.nix` and a `.envrc` containing `use flake`, then runs `direnv allow` if direnv is on PATH. `-direnv=false` skips the `.envrc`. The description defaults to `<directory> dev environment`. Flags must come before the directory argument.

`add` searches upward from the current directory for `flake.nix`, like `git` finds `.git`, and skips packages already present. `-f` points at a specific file.

`search` prints matches best first, one per line, as `<pname>  (<attrpath>)  — <description>`. Package-name matches always outrank description-only hits. `--limit` defaults to 20.

### Interactive picker

`iceman search`, `iceman add`, and `iceman init -i` with no packages open a picker in the terminal: type to filter, arrows to move, space to multi-select, enter to confirm, esc to cancel. `add -i` and `init -i` write the selection into the flake; `search` prints it, so `iceman add $(iceman search)` also works. When stdout is not a terminal, `add` and `search` error instead of opening the picker, so scripts never hang. `NO_COLOR` disables colors.

`skill` writes iceman's own `AGENTS.md` into a project, so Claude Code, Codex, Cursor, or any agent that reads `AGENTS.md` knows to reach for `iceman` instead of hand-editing `flake.nix`. Refuses to overwrite an existing file unless `-force`.

## Requirements

`init` and `add` are plain text manipulation and work without Nix installed. `search` and the `-i` picker shell out to `nix search nixpkgs`, so they need `nix` on PATH with flakes enabled. The first fetch of the catalog takes a couple of minutes; it is cached at `$XDG_CACHE_HOME/iceman/nixpkgs-search.json` (or the OS user cache dir) for 24 hours. `--refresh` forces a re-fetch.

Don't have Nix yet? The [Determinate Nix Installer](https://github.com/DeterminateSystems/nix-installer) is the easiest way to get it, with flakes enabled by default:

```bash
curl --proto '=https' --tlsv1.2 -sSf -L https://install.determinate.systems/nix | sh -s -- install
```

## Generated flake

`iceman init -p go,gopls,git myproject` produces:

```nix
{
  description = "myproject dev environment";

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
            go
            gopls
            git
          ];
        };
      }
    );
}
```

## Contributing

Issues and PRs are welcome. Before opening one:

```bash
go build ./...
go test ./...
```

Bug reports and feature requests have templates under [`.github/ISSUE_TEMPLATE`](.github/ISSUE_TEMPLATE) — please search existing issues first. `init`/`add`/`skill` are covered by the integration tests in `test/`; `flake.nix` rendering and `buildInputs` editing are covered in `internal/flake`. Add or update tests alongside behavior changes.

## License

MIT — see [LICENSE](LICENSE).
