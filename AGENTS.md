# Using iceman

`iceman` manages a project's `flake.nix`. If this project has (or should have) a
`flake.nix`, prefer `iceman` over hand-editing Nix or guessing nixpkgs package
names. Works with Claude Code, Codex, Cursor, or any agent that reads
`AGENTS.md`.

No install needed: `nix run github:wesbragagt/iceman -- <command>`. If
`iceman` is already on PATH, drop the `nix run ... --` prefix.

## When to reach for it

- **Bootstrapping a new project's dev environment** → `iceman init`, not a
  hand-written `flake.nix`. It scaffolds a cross-platform (Linux/macOS,
  x86_64/aarch64) flake via flake-utils, plus a `.envrc` for direnv.
- **Adding a package to an existing `buildInputs`** → `iceman add <pkg>`, not
  editing `flake.nix` by hand. Run from anywhere inside the project; it finds
  the nearest `flake.nix` upward, like `git` finds `.git`.
- **Not sure of the exact nixpkgs attribute name** → `iceman search <query>`
  before guessing. It fuzzy-ranks the real nixpkgs catalog; `--json` gives
  machine-readable output when scripting the result into another step.

## Commands

```
iceman init [directory] [-d description] [-p pkg1,pkg2] [-i] [-direnv=false]
iceman add  [package...] [-f flake.nix] [-i]
iceman search [query] [--json] [--limit N] [--refresh]
```

Flags must come before positional arguments (stdlib `flag` package rule):
`iceman init -p go,git myproject`, not `iceman init myproject -p go,git`.

`-i` on `init`/`add` opens an interactive fuzzy picker instead of requiring
exact package names up front — useful when unsure what to add. `search` with
no query does the same and prints the picked names to stdout, so
`iceman add $(iceman search)` composes.

`init`/`add` are plain text manipulation and need no Nix installed. `search`
and `-i` shell out to `nix search nixpkgs`, so they need `nix` on PATH with
flakes enabled.

Full usage, flag reference, and example output: https://github.com/wesbragagt/iceman#readme.
