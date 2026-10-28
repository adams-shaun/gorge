# Packages

Every Go package in gorge, with its purpose, size, importers and status, for
a contributor looking for where something lives or whether a package is
load-bearing. The guided tour of the tree, the import rules and "where does
my change go?" are on [Repository layout](../contributing/repo-layout.md).

The tables below are generated from the source tree by
[`cmd/pkgmap`](https://github.com/adams-shaun/gorge/tree/main/cmd/pkgmap) each
time the site is built, so they never go stale. Tier and status come from one
committed classification file,
[`docs/site/architecture/packages.tiers.json`](https://github.com/adams-shaun/gorge/blob/main/docs/site/architecture/packages.tiers.json);
`go run ./cmd/pkgmap -check` (and the package's test) fails when a package is
missing from it or an entry names a path that no longer exists.

Columns:

- **Purpose**: the first sentence of the package doc comment (or the
  classification's `purpose` when the package has none).
- **LOC / Test LOC**: non-test / test lines of Go in that directory (not
  recursive), counting every build tag.
- **Imp**: in-repo packages importing it from non-test code; `+N` adds the
  packages that import it only from tests.
- **Status**: **CORE** (the engine), **SUPP** (supports the product),
  **EXT** (optional extension: bots, search, ML, alternate wires),
  **TOOL** (tooling, tests, ratchets), **SUSP** (suspected obsolete) or
  **RED** (redundant). SUSP and RED entries are discussed on the
  [cleanup roadmap](cleanup.md).

The tier-dependency graph at the end is built from the actual imports, and
flags any import from a tier into one listed after it.

For where a given kind of change belongs, see
[Repository layout](../contributing/repo-layout.md#where-does-my-change-go).

To preview locally:

```sh
go run ./cmd/pkgmap > docs/site/architecture/_generated_packages.md
```

--8<-- "architecture/_generated_packages.md"

## Non-Go top-level paths

| Path | What | Status |
|---|---|---|
| `web/` | Svelte+Vite client | SUPP |
| `scripts/` | contributor and CI helpers: CI sharding, smokes, enginebench, the docs sets renderer | TOOL |
| `docs/` | design docs (`docs/design/`), decisions (`docs/decisions/`), this site (`docs/site/`), an internal guide for AI coding agents (`docs/agents/`) | SUPP |
| `.githooks/` | opt-in git hooks: the package-classification check, the `RACE=1` race gate | TOOL |
| `.github/` | the `ci`, `docs` and `coverage` workflows, the PR template | TOOL |

Draft events, the maintainers' extensions and the oracle pipelines are not in this
repository and are not listed here; see
[The three repositories](../repos/index.md).
