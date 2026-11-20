# Packages

Every Go package in gorge with its purpose, size, importers and status: where something
lives, and whether a package is load-bearing. For the tour of the tree, the import rules and
"where does my change go?", see [Repository layout](../contributing/repo-layout.md).

The tables are generated from the source tree by
[`cmd/pkgmap`](https://github.com/adams-shaun/gorge/tree/main/cmd/pkgmap) at every site build
(`.github/workflows/docs.yml`). Tier and status come from one committed file,
[`packages.tiers.json`](https://github.com/adams-shaun/gorge/blob/main/docs/site/architecture/packages.tiers.json).
`go run ./cmd/pkgmap -check` (and `TestClassificationIsComplete`) fails when a package is missing
from it or an entry names a path that no longer exists. The git hooks in `.githooks/` run the check
on commit.

Columns:

- **Purpose**: the first sentence of the package doc comment, or the classification's `purpose`.
- **LOC / Test LOC**: non-test / test lines of Go in that directory (not recursive), every build tag counted.
- **Imp**: in-repo packages importing it from non-test code; `+N` adds those importing it only from tests.
- **Status**: **CORE** (the engine), **SUPP** (supports the product), **EXT** (optional extension:
  bots, search, ML, alternate wires), **TOOL** (tooling, tests, ratchets), **SUSP** (suspected obsolete),
  **RED** (redundant).

The tier-dependency graph at the end is built from the actual imports and flags any import from a
tier into one listed after it. To preview locally:

```sh
go run ./cmd/pkgmap > docs/site/architecture/_generated_packages.md
```

--8<-- "architecture/_generated_packages.md"

## Non-Go top-level paths

| Path | What | Status |
|---|---|---|
| `web/` | Svelte+Vite client | SUPP |
| `scripts/` | contributor and CI helpers: CI sharding, smokes, the wire matrix, enginebench, the docs renderers | TOOL |
| `docs/` | design docs, decisions, this site, and a guide for AI coding agents | SUPP |
| `.githooks/` | opt-in git hooks: the package-classification check, the `RACE=1` race gate on a push to main | TOOL |
| `.github/` | the workflows (`ci`, `oracle`, `coverage`, `docs`, `demo-image`, `queue-cancel`) and the PR template | TOOL |

Machine learning, private experiments and the compliance oracles are not in this repository:
[the three repositories](../repos/index.md).
