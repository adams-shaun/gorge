# feat(cardc): CC-33a the mtgish fork register, its lock check and the fork's regeneration workflow

Kind: feature

Target branch: `cardc`. Complexity: low. Implementer: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, section 5 ("Fork rules",
"Toolchain"). Independent of CC-30 and CC-32; runs in parallel with CC-30. CC-33b (fallback rows) follows CC-32.

## Goal

1. The fork patch register, `engine/cardc/testdata/mtgish-fork.tsv`, and `TestMtgishForkRegister`, which holds its
   `carried` rows equal to `MTGISH.lock`'s `patch` lines, offline.
2. A GitHub Actions workflow **for the fork** (`github.com/adams-shaun/mtgish`, branch `gorge`) that regenerates
   `data/mtgish.lines.json` from the grammar and fails unless the result is byte-identical to the committed file.
   It is committed in gorge as a file to install; this ticket never pushes to the fork.

## Workspace facts (verified 11-24 at `de85384ff1`)

- `engine/cardc/MTGISH.lock`: `repo https://github.com/adams-shaun/mtgish`, `upstream https://github.com/i5jb/mtgish`,
  `base` = `ref` = `1553384c9a970a02514b337db9c86697313530e8`, three `sha256` lines, no `patch` lines.
- `mtgish.ReadLock` (`engine/cardc/mtgish/lock.go:28`) parses it, `patch <sha> <issue>` into `Lock.Patches`.
- `git ls-remote https://github.com/adams-shaun/mtgish` (no credentials): `refs/heads/gorge` and `main` both at
  `1553384c…`; the fork has no `.github/workflows` yet.
- mtgish's `justfile` at `1553384`: `parser_generate_mtgish` (depends on `plurals_generate` and `parser_compile`)
  writes `data/mtgish.lines.ron` with the Go parser (`go_mtg_parser/go.mod`: `go 1.18`); `rust_build_ronl_to_bincode`
  and `rust_build_bincode_to_jsonl` build two `rust_syntax` binaries with `cargo`; `convert_to_json` runs them to write
  `data/mtgish.lines.json`. `plurals_generate` runs `grammars/generate_plurals` and `jq`. Read the justfile and
  `grammars/generate_plurals` from a shallow clone in the user cache directory (never `/tmp`) before writing the workflow; the clone is read-only reference and is never pushed.
- Neither `cargo` nor `rustc` is on this box: the workflow cannot be run here, and this ticket does not try.

## Design

Decided. The seat makes no design calls.

### 1. Register (`engine/cardc/testdata/mtgish-fork.tsv`)

```
# mtgish fork patches (docs/design/2026-11-23-typed-script-values/PLAN.md section 5).
sha	upstream_issue	class	status	cards
```

No rows yet: the fork carries no patch. Columns: `sha` 40 lower-case hex; `upstream_issue` an i5jb/mtgish issue
number, > 0, required; `class` a short token (`up-to-exact`, `morph-face-up`, `haste-dropped`, `class-level-cost`,
`mdfc-back-cost`, or a new lower-case-hyphen token); `status` `carried`, `fixed-upstream` or `dropped`; `cards`
comma-separated card names, or `-`.

### 2. Reader and test (`engine/cardc/mtgish/fork.go`, `fork_test.go`)

```go
type ForkPatch struct{ SHA string; Issue int; Class, Status string; Cards []string }
func ReadForkRegister(path string) ([]ForkPatch, error) // header required; a bad row is an error naming its line
```

- `TestMtgishForkRegister` (offline): the register parses; no duplicate `sha`; the multiset of (sha, issue) over
  `carried` rows equals the lock's `patch` lines, both ways; a lock with no `patch` line has `ref == base`.
- `TestReadForkRegister`: one good row of each status, and one bad row per column rule.

### 3. The workflow (`engine/cardc/fork/regenerate.yml`)

Not under `.github/`: gorge's own CI must never run it. It is the fork's `.github/workflows/regenerate.yml`, verbatim.
- Triggers: `push` to `gorge`, and `workflow_dispatch`.
- One job on `ubuntu-24.04`: `actions/checkout@v4`; `actions/setup-go@v5` with `go-version-file:
  go_mtg_parser/go.mod`; `dtolnay/rust-toolchain@stable`; `extractions/setup-just@v2`; `jq` from apt if the image
  lacks it; then `just parser_generate_mtgish`, `just rust_build_ronl_to_bincode`, `just rust_build_bincode_to_jsonl`,
  `just convert_to_json`.
- Last step: `git diff --exit-code --stat -- data/mtgish.lines.json`. On failure, upload the regenerated file as an
  artifact (`actions/upload-artifact@v4`, 7-day retention) so the difference can be inspected.
- No secrets, no `pull_request` trigger, no step that pushes, tags or opens anything.
- A header comment says what it proves (the committed data is reproducible from the grammar at that commit) and that
  each fork patch commit carries its own regenerated data, checked by this run.

`TestForkWorkflow` (offline, in `engine/cardc/mtgish`): the file exists, names each of the four `just` recipes, ends
with the `git diff --exit-code` check, and contains none of `pull_request`, `secrets.`, `git push`.

## Done means

- [ ] `make test-one PKG=./engine/cardc/mtgish RUN='^(TestMtgishForkRegister|TestReadForkRegister|TestForkWorkflow|TestReadLock)$'` passes.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestPublicBoundary|TestNoVerbatimForgeLines)$'` passes.
- [ ] The hand-back lists anything in mtgish's recipes the workflow could not express (a tool not on the runner,
  a network fetch, a non-deterministic step), with the file and line.
- [ ] No `make test-targeted` or `make test-smart` per ticket: one `make test-smart` runs at the wave-4 head.

## Out of scope

- Pushing anything to the fork, opening issues or pull requests anywhere outside gorge. Never open a pull request to
  i5jb/mtgish. Installing the workflow is the operator's, by hand.
- Fork patch commits themselves (grammar fixes and their regenerated data), and fallback rows (CC-33b).
- Installing Rust on this box.

## Collisions

New `engine/cardc/mtgish/fork.go`, `fork_test.go`, `engine/cardc/fork/regenerate.yml`,
`engine/cardc/testdata/mtgish-fork.tsv`. Touches no CC-30 file.
