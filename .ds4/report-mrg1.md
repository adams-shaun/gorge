# Merge-conflict resolution — task cli-20260922T225140Z-c661fa12

## Starting state

`git status` found the tree CLEAN — no rebase or merge in flight. The daemon's
earlier attempt (rebase, then a merge fallback, both conflicting per
`.ds4/merge-conflict-mrg1.md`) had been fully aborted before this seat
started; branch tip was `63c07260` ("fix(rules): carry layer-4 derived types
into the ordinary filter grammar") on top of merge-base `c472a88f`. Main had
advanced since (`0104252d` tip: the pc1 closure and the 6a16cd8c merge).

I redid the integration as a **merge of `main` into the branch** (`git merge
main` — `git rebase` is forbidden in a seat), which reproduced exactly the
conflict set the daemon saw: `AGENTS.md` content-conflicted;
`effects/filter.go`, `rules/clone.go`, `rules/engine.go`, `rules/layers.go`,
`rules/statics.go` auto-merged.

## Conflicted file: AGENTS.md (one hunk)

Both sides touched the same slot in the "Known approximations" table — each
had DELETED a different row that the other kept:

- **branch side** deleted the **"Layer-4 type grants reach only the layer
  walk"** row — that IS the reviewed fix `63c07260` itself (commit message:
  "Closes the AGENTS.md 'Layer-4 type grants reach only the layer walk' row.
  The row is deleted and knownApproximationRows is lowered to match."), with
  `rules/layer4types.go` + `SpecContext.DerivedTypes` + tests.
- **main side** deleted the **(pc1)** row ("A positive predicate word that is
  neither a type word nor `Colorless`/`MultiColor` fails closed…") — closed by
  `179a3de1` "feat(effects): implement pc1 context-bound object predicates"
  (`ExiledWithSource`, `wasDealtDamageThisTurn`, `IsImprinted`,
  `NotDefinedTargeted`, `DefenderCtrl`, `Opponent` predicates).

**Resolution:** delete BOTH rows — each deletion is a legitimate closure and
the register is delete-only. Merged table measured with the ratchet test's own
line logic (`## Known approximations` … next `## `, lines starting `| `, minus
header):

| ref | rows | constant |
|---|---|---|
| base `c472a88f` | 51 | 51 |
| main | 49 (deleted fx20, legend-rule, pc1; added the KReplacement-order row) | 50 (lags one — allowed) |
| branch `63c07260` | 50 (deleted layer-4) | 50 |
| merged | **48** | set to **48** |

## internal/testutil/agentsdoc_test.go (auto-merged, then adjusted)

Both sides changed the same line `51`→`50`, so git auto-merged it to 50 — but
that describes neither merged state: the merged table measures **48** rows
(51 − fx20 − legend − pc1 − layer-4 + KReplacement-order row). Set
`knownApproximationRows = 48`. `knownOversizeRows` (8) and `standInCellLimit`
(600) identical on all sides, untouched. Method mirrors main's own
integration precedent `4dcef2ea` ("lowered to 51 after ft1 closure — the
merged table measures 51 rows, confirmed by the ratchet's own count").

## Auto-merged code files — semantic sanity check

`git diff main -- effects/filter.go rules/layers.go rules/statics.go
rules/clone.go rules/engine.go` showed ONLY the branch's layer-4 additions
(the `SpecContext.DerivedTypes` slice field, the `hasTypeCtx` published-table
arm with the walk's `ExtraTypes`-authoritative guard, `matchesWithChars`
clearing `sc.DerivedTypes`, the `layer4InPool`/`layer4Types` engine fields
and Clone copy) — `git diff main --stat` over the whole tree equals the
branch commit's own 17-file stat. The reverse check confirmed main's pc1
predicates (`wordPredicate`'s context-bound kinds, `ExiledWithSource`,
`IsImprinted`, …) are present in the merged tree. The two sides' shared
surface (`effects/filter.go`'s compiled-predicate bypass now gated on
`!hasEffectiveName(o, sc) && !hasDerivedTypeEntry(o, sc)`) composed cleanly.

## Commands run and output

- `git merge main` → `CONFLICT (content): Merge conflict in AGENTS.md`; the
  five code files auto-merged (same set as the daemon's log).
- `git status --short` after staging → no UU left.
- `.cards` check: **present** (real symlink → `/home/sadams/projects/gorge/.cards`),
  so the corpus-backed runs below were real, not vacuous skips.
- `go test ./internal/testutil/ -run 'TestKnownApproximations' -v` →
  `--- PASS: TestKnownApproximationsOnlyShrinks (0.00s)` / `ok`.
- Post-merge ratchets + the branch's own layer-4 tests:
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestLayer4'`
  → `ok github.com/adams-shaun/gorge/rules 0.875s`. (0.875 s matches the
  verified non-vacuous 0.894 s run of the same set recorded in the prior
  round's report; `.cards` present.)
- `go test ./effects -run 'TestPlayerSpecFx20Grammar|TestUnknownPredicates'`
  → `ok github.com/adams-shaun/gorge/effects 0.006s` (main's fx20 closure
  still passes on the merged tree).
- Behaviour goldens (the branch's fix is an engine-behaviour change main has
  never gated, so both were checked cheaply before committing):
  - `go test ./internal/archtest/` → `ok 3.221s`
  - `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/` →
    `ok 1.017s` — the pinned split did NOT move.
- `gofmt -l` on the touched Go file (`internal/testutil/agentsdoc_test.go`)
  and the auto-merged ones → no output.

## Uncertainties / notes

- I set the register constant to the measured merged count (48) rather than
  either side's 50; main's own 50 already lagged its 49-row table by one.
  TestKnownApproximationsOnlyShrinks passes with 48 and prompts to lower, so
  48 is the value the ratchet itself asks for.
- The merge commit includes this report file. The version the daemon had
  staged in `.ds4/` was a STALE copy of ticket 6a16cd8c's round-1 report
  (different task); it is replaced by this record. Nothing of value is lost —
  that round's record is committed in its own history on main.
- No engine behaviour beyond the approved fix `63c07260` was introduced: the
  resolution itself touched only AGENTS.md, the register constant, and this
  report.

## Issues

None found during this round's resolution.
