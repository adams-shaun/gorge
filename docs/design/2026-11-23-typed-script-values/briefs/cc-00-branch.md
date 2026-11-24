# feat(cardc): CC-00 branch hooks, the mtgish fork, its pin and fetch, the boundary and import guards

Kind: feature

Target branch: `cardc` (never main). Complexity: medium. Seat: Sonnet.
Design of record: `docs/design/2026-11-23-typed-script-values/PLAN.md` on `cardc`, sections 3 and 5.

## Goal

Make the `cardc` side branch ready for the card compiler. Afterwards:
- the mtgish data is fetched from our own fork, `github.com/adams-shaun/mtgish`, pinned by commit;
- `make mtgish-fetch` fetches the pinned data into gitignored `.cards/mtgish/<ref>/`, checked by sha256;
- the public-boundary test refuses any committed mtgish data;
- a new archtest proves nothing outside `engine/cardc/**` and `cmd/cardc` imports cardc, so main's behaviour cannot
  change through this branch.

## Behaviour

- `make mtgish-fetch` downloads every file the lock lists (`data/mtgish.lines.json`, `LICENSE`,
  `typescript_types/oracle_cards.d.ts`) at `MTGISH_REF` into
  `$(CARDS_DIR)/mtgish/<ref>/`, from the fork, and checks each file's sha256 against `engine/cardc/MTGISH.lock`.
  - A file already present with the locked sha256 is not fetched again.
  - A mismatch exits 1.
  - A ref that differs from the lock exits 2.
- `make cardc-check` runs `$(MAKE) test-targeted BASE=origin/main`: every package the `cardc` branch changes against
  main, plus their test reverse deps. (`test-one` refuses `./...`.)
- `TestPublicBoundary` fails on a tracked file that:
  - has a path component `.mtgish`;
  - has a base name starting `mtgish.lines.`;
  - or starts with an mtgish card record (`{"_OracleCard":`).
- `TestNothingImportsCardc` fails when any package outside `engine/cardc/...` and `cmd/cardc` imports a package under
  `engine/cardc`.
- Nothing on main changes behaviour. No engine package is edited.

## Design

Decided; the seat makes no design calls.

1. **The fork exists before this ticket starts.** The operator creates `adams-shaun/mtgish` and its branch `gorge` at
   upstream `1553384c9a970a02514b337db9c86697313530e8` (PLAN.md section 5, "Prerequisite"). The seat creates nothing
   on GitHub; it only checks the branch with `git ls-remote`.
   - **Never open a pull request to i5jb/mtgish, and never run `gh pr create` in or against it.** The upstream author
     accepts only hand-written PRs. Upstream contribution is issues only, and the operator files those.
2. **Data location: `$(CARDS_DIR)/mtgish/<ref>/`, not a top-level `.mtgish/`.** agentctl links `.cards` into every seat
   worktree; a top-level directory would be missing in seats. `.cards/` is already gitignored (`.gitignore:4`).
3. **The lock**, `engine/cardc/MTGISH.lock`. Comment lines start with `#`. The data lines, in order:

   ```
   repo https://github.com/adams-shaun/mtgish
   upstream https://github.com/i5jb/mtgish
   base 1553384c9a970a02514b337db9c86697313530e8
   ref 1553384c9a970a02514b337db9c86697313530e8
   sha256 b1d67570bfa51956ed34b1df021a701f01c170e68085005cf4f200080d80a005 data/mtgish.lines.json
   sha256 529d8c04b1a74b127edcc37e14e6ee99a3a9068ace45f5623a3422ff22bb4c2d LICENSE
   sha256 97b7760c61c063b262c12704a324f9298cb70913c2b1696f858097557d8a4eb0 typescript_types/oracle_cards.d.ts
   ```

   - The first two sums match gorge-compliance PR #373's lock for upstream `1553384`.
   - The third, mtgish's ts-rs-generated JSON schema, was measured from a checkout at `1553384`. CC-02 reads it as
     the variant catalogue.
   - The fork serves the same bytes.
   - Later fork patches add one `patch <sha> <upstream-issue-number>` line each. CC-33 owns that; CC-00 writes none.
4. **The fetch script**, `scripts/cardc-mtgish-fetch.sh`, a bash adaptation of gorge-compliance's
   `scripts/mtgish-fetch.sh` (same `set -euo pipefail`, same sha256 loop and `.part` rename). It differs in three ways:
   - the lock path is `engine/cardc/MTGISH.lock`;
   - the URL base is `https://raw.githubusercontent.com/adams-shaun/mtgish` (env `MTGISH_BASE_URL` overrides);
   - the destination is `${CARDS_DIR:-.cards}/mtgish/<ref>`.

   `MTGISH_REF` comes from the environment, else from `cardc.mk`.
5. **`cardc.mk`** at the repo root holds `MTGISH_REF ?= 1553384c9a970a02514b337db9c86697313530e8` and the phony
   targets `mtgish-fetch` and `cardc-check`. The Makefile gets one line, `include cardc.mk`, as its last line. No other
   Makefile edit.
6. **`.gitignore`** gets `.mtgish/` (a guard against stray copies).
7. **Boundary rule.** Append one entry to `boundaryRules` (`internal/archtest/public_boundary_test.go:120`) with id
   `mtgish-data`, implementing the three checks in Behaviour. Add three violating and two clean cases to
   `TestPublicBoundaryDetection` (`internal/archtest/public_boundary_test.go:608`).
8. **Import guards.**
   - Add `TestNothingImportsCardc` to `internal/archtest/arch_test.go`. It uses `packages(t)`
     (`internal/archtest/arch_test.go:42`) and checks every package's imports. So it cannot pass vacuously, it also
     requires `module + "/engine/cardc"` to exist in the package set.
   - Put the check in a pure function, `cardcImportViolations(module string, imports map[string][]string) []string`
     (importer → its imports; returns one `importer -> imported` line per violation, sorted). The test calls it on
     the real package set and on a synthetic map.
   - Add rows to the `forbidden` list of `TestDependencyOrderHolds` (`internal/archtest/arch_test.go:204`):
     - `engine/cardc/ir` → `engine/cards`;
     - `engine/cardc/ir` → `engine/state`;
     - `engine/cardc/mtgish` → `engine/cards`.

     The IR is a leaf, and the front end never reads gorge's card structs.
9. **Package stub.** `engine/cardc/doc.go`: `package cardc`, with a doc comment that points at the plan file.

## Implementation steps

1. Check the fork: `git ls-remote https://github.com/adams-shaun/mtgish refs/heads/gorge` must print
   `1553384c9a970a02514b337db9c86697313530e8`.
   - If it prints nothing or another sha, stop and report BLOCKED: "fork prerequisite missing (PLAN.md section 5)".
     Do not create the fork, do not run `gh`, and do not fetch from upstream instead.
2. Write `engine/cardc/MTGISH.lock`, `scripts/cardc-mtgish-fetch.sh` (executable), `cardc.mk`, the Makefile `include`
   line and the `.gitignore` line.
3. Run `make mtgish-fetch`. Confirm all three files land under `.cards/mtgish/<ref>/` with the locked sums.
4. Write `engine/cardc/doc.go`, the boundary rule, its detection cases, `TestNothingImportsCardc` and the three
   dependency rows.

## Tests

- `TestPublicBoundary`: passes on the branch.
- `TestPublicBoundaryDetection`: with the new `mtgish-data` cases.
- `TestNothingImportsCardc`: new. It includes a table-driven half over a synthetic import map (one allowed importer
  under `engine/cardc/mtgish`, one allowed `cmd/cardc`, one forbidden `engine/rules`).
- `TestDependencyOrderHolds`: with the three new rows.

## Done means

- [ ] `make mtgish-fetch` exits 0 and prints all three files as `ok` or `fetched`.
- [ ] `make test-one PKG=./internal/archtest RUN='^(TestPublicBoundary|TestPublicBoundaryDetection|TestNothingImportsCardc|TestDependencyOrderHolds)$'` passes.
- [ ] `make test-targeted BASE=origin/cardc` passes.
- [ ] `make test-smart BASE=origin/cardc` passes.

## Out of scope

- Any fork patch commit or upstream issue (CC-33).
- Any Go code under `engine/cardc/` beyond `doc.go`.
- `cmd/cardc`.
- A Rust toolchain. Neither `cargo` nor `rustc` is on the box, and CC-00 needs neither.
- Creating the fork or any GitHub object, and opening any pull request or issue upstream.

## Collisions

First ticket of the program; nothing else runs on `cardc` until it lands. Its shared files are the Makefile `include`
line, `.gitignore`, and the two archtest files. Later tickets do not edit them; only the rolling `cardc-follow` merge
touches them again.
