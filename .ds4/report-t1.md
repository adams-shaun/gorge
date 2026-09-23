# Report — Mill<N> cost verification

No code changes were needed. The existing regression and composed/short-library coverage already establish the requested behavior; the report's implementation commits are present in current history. `.cards/` was present as a symlink resolving to `/home/sadams/projects/gorge/.cards`, so corpus-backed tests were not skipped.

`rules/mill_cost_test.go:TestMillikinMillCostMovesTopLibraryCardBeforeMana` uses the real corpus Millikin and checks the ability parses as API `Mana` with exactly `Mill<1>`, confirms the top library card is in the library, and proves it moves to the graveyard and mana is produced. It also rejects the unimplemented-Mill Note path. `rules/mill_cost_composed_test.go` covers composed and ordinary spell/ability payments; `rules/mill_cost_short_library_test.go` covers the empty-library case. The repo-deck parameter census is clean for the current `knownUnsupportedParams` table; no allowlist change is warranted.

## Verification

- Targeted regression and census:

  ```text
  $ go test -run 'TestMillikinMillCostMovesTopLibraryCardBeforeMana|TestEveryRepoDeckParamsAreRead' ./rules/ > .ds4/scratch/mill-targeted.log 2>&1; rc=$?; tail -30 .ds4/scratch/mill-targeted.log; exit $rc
  ok   github.com/adams-shaun/gorge/rules  0.770s
  ```

- Architecture gate:

  ```text
  $ go test ./internal/archtest/ 2>&1 | tail -15
  ok   github.com/adams-shaun/gorge/internal/archtest  4.141s
  ```

- Constructed default golden:

  ```text
  $ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/ 2>&1 | tail -5
  ok   github.com/adams-shaun/gorge/cmd/botbench  1.350s
  ```

- Generated types:

  ```text
  $ go run ./cmd/gentypes -check
  [no output; exit 0]
  ```

- `gofmt -l <changed files>`: not applicable; there are no changed files.
- No new test was added, so no failure-with-fix-removed demonstration is applicable.

## Issues

None found within scope. The reported behavior is already fixed and pinned by `33eed7fa`, `74043a77`, and `4480d00e`. No Known approximations row or census entry was changed. No commit was created because the working tree has no code changes.
