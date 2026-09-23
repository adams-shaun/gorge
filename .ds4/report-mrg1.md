# Merge-conflict resolution report — mrg1 (ticket cli-20260922T225141Z-8d166e9c)

## Starting state

`git status` showed a CLEAN tree on `wt/cli-20260922T225141Z-8d166e9c` at the branch
commit 0679b1cd ("feat(rules): foretell predicates, Cosmos Charger any-turn grant,
effect-delivered free may-play"), 1 commit ahead of merge-base 270448df. The failed
rebase/merge attempts from the dispatch log had already been rolled back — no
`.git/rebase-merge`, `.git/rebase-apply` or `MERGE_HEAD` present. So there was no
in-flight operation to finish; I performed the integration myself as a **merge of
main** (merge, not rebase — this repo's standing rule forbids `git rebase`, and
main's history shows this is the established integration shape for wt branches).

## Conflicted file

One file: `internal/testutil/agentsdoc_test.go`, hunk on the `knownApproximationRows`
constant:

- **HEAD (branch 0679b1cd):** `knownApproximationRows = 87` — the branch's ticket
  deleted one Known-approximations row (the foretell/Cosmos Charger/MayPlay row) from
  AGENTS.md (auto-merged cleanly) and lowered the constant accordingly (88 → 87).
- **main (e53c80a2):** `knownApproximationRows = 76` — main removed rows via its own
  landed tickets since the split.

## Resolution

The auto-merged AGENTS.md contains BOTH sides' row deletions. Measured with awk over
the merged file: **74 data rows** (base 88 − 1 branch deletion − 13 main deletions).
Set the constant to the measured merged value: `knownApproximationRows = 74`. This is
correct in both directions of the ratchet: no row was added, and the constant exactly
matches the table, so `TestKnownApproximationsOnlyShrinks` passes with no slack and no
false failure. `knownOversizeRows = 8` needed no change (main's side of the conflict
region only). No test logic or other file was touched.

Note: the `knownApproximationRows` values in the main-side history are stale relative
to the table (main's own table measured 75 data rows against a constant of 76 — a
prior resolver's report-mrg1.md in main's tree says the same); the test only fails on
growth so this never bites, but worth knowing.

The merge also auto-merged the branch fix's real code files against main's later
changes to the same files: `effects/filter.go`, `effects/misc.go`, `rules/legal.go`,
`rules/paramcensus_test.go` — git merged these cleanly and the branch's own tests
still pass (below).

## Commands run

- `git merge main` → `CONFLICT (content): Merge conflict in internal/testutil/agentsdoc_test.go`, all other files auto-merged.
- Row count: `awk` over merged/HEAD/main AGENTS.md → merged 74 data rows, branch 87, main 75.
- Edit constant → `git add internal/testutil/agentsdoc_test.go` → `git commit --no-edit`
  → merge commit **98dcf594** ("Merge branch 'main' into wt/cli-20260922T225141Z-8d166e9c").
- `go test ./internal/testutil/ -run 'TestKnownApproximations'` → `ok 0.001s`.
- Post-merge ratchets (per brief):
  `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeckIsFullySupported|TestEveryRepoDeckParamsAreRead|CountHeadRatchet' -v`
  → exit 0, 0 SKIPs, all PASS (TestEveryRepoDeckIsFullySupported 0.57s,
  TestEveryRepoDeckParamsAreRead 0.11s, both trigger-mode registry tests PASS).
  `go test ./rules -run 'TestEveryRepoDeckCountHeadResolves'` (the actual name of the
  CountHead ratchet) → `ok 0.600s`.
- Branch-fix sanity: `go test ./rules -run 'Foretell|MayPlay'` → `ok` (the branch's
  `rules/foretell_grant_test.go` suite still passes on the merged tree).
- `gofmt -l internal/testutil/agentsdoc_test.go` → empty.
- `git status` → clean.

`.cards` was present (symlink to the real corpus, `ir.gob.gz` resolves), so these runs
are real, not vacuous skips.

## Uncertainties

- This report file path itself is a TRACKED file in main (a previous pipeline's
  resolver committed their mrg1 report there for ticket cli-20260922T225141Z-d9f9a1a7).
  Per the report contract I wrote to exactly this path; the old content remains in git
  history. The report is committed as a separate `docs` commit so the branch ends
  clean.
- Commit messages in this repo carry no `Ref:` trailer and no attribution (gorge
  rules); the merge used the default generated message, the report commit is
  `docs(merge): mrg1 conflict-resolution report`.

STATUS=DONE
COMMITS=98dcf594 (merge) + report commit ce340ec9 (docs)
TESTS=internal/testutil KnownApproximations ratchet ok; rules trigger-mode registry + repo-deck acceptance + params + CountHead ratchets all ok, 0 skips; branch Foretell|MayPlay suite ok; gofmt clean
