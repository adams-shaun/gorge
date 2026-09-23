# Merge conflict resolution: mrg1 (round 4 — branch wt/cli-20260922T225138Z-7a41baa0, main at db82645a)

## Starting state

The worktree was clean, HEAD = `f58f1440` (this branch's round-1 merge of
main@c2596487). `main` had since advanced to `db82645a` (the dynamic unless-cost
grammar, its round-3 report, and the 636f892f merge). The branch's approved fix
commits (`cc65f39f` infer stack zone from ValidTgts spell, `070f673d` keep
origin-implied zone over ValidTgts stack inference, `c0a5a86c` preselected
counter targets offered) are NOT on main; main independently landed
`ffae51a54` (origin-implied target zone + a new `ValidTgts$ Spell` latent-footgun
approximation row) AFTER the branch's fix.

Re-ran the integration as `git merge main` from the branch (rebase is forbidden
here; prior rounds of this same integration were landed as merges on main too).

## Conflicted files and resolution

1. **`AGENTS.md`** (one conflict block at the ValidTgts/UnlessCost/TargetType
   table region):
   - HEAD side: the `UnlessCost$` mana-window row and the old `TargetType$
     qualifiers` row (both as of main@c2596487, which this branch had already
     merged in round 1).
   - main side: the new `ValidTgts$ Spell ... latent footgun` row — main's
     `ffae51a54` deleted the UnlessCost row (the dynamic unless-cost grammar
     landed: e7d5bcec/e3372a23) and the TargetType row (b89e7869), then re-added
     the ValidTgts row recording the footgun the branch's reviewed fix CLOSES.
   - Resolution: took main's side for the UnlessCost/TargetType deletions (main
     carries later, deliberate closures of both), then DELETED the `ValidTgts$
     Spell` row as well: in the merged tree `targetZones` DOES infer the stack
     from a bare `ValidTgts$ Spell` (branch fix `cc65f39f`/`070f673d`, kept by
     the auto-merge), so main's row was false against the merged behaviour.
     Merged `AGENTS.md` now differs from main by exactly that one row deletion.
2. **`internal/testutil/agentsdoc_test.go`**: auto-merge took main's constant 78
   (= main's measured 78 data rows). After the row deletion above, lowered to
   **77** = the merged table's measured row count (verified: 78 data rows at
   main, minus 1). The register never rises.
3. **`.ds4/report-mrg1.md`**: both sides were prior rounds' reports (round 1 on
   this branch, round 3 on main). Replaced with THIS report per the
   report-path contract (prior rounds were each superseded in place).
4. **`rules/stack.go`**: auto-merged with NO textual conflict. The merged result
   keeps the branch's reviewed implementation (origin-implied zone outranks;
   bare `ValidTgts$ Spell` targets the stack; `targetsStackObjects` via
   `state.StackKindTokenOf`, which still exists in the merged
   `state/stackkind.go`) plus main's later unless-cost changes. Verified
   `go build ./state ./rules` clean. `rules/stack_target_zones_test.go`
   (branch's regression, added by cc65f39f) survives main's independent
   deletion of the same-named file at its base.

All other main-side changes (`effects/unless.go`, `effects/registry.go`,
`rules/unless_payment.go`, `rules/mana*.go`, `rules/resolution.go`, the
`unless_*_test.go` suites) auto-merged and were retained unmodified.

## Commands run and output

```
git merge main
  -> Auto-merging .ds4/report-mrg1.md CONFLICT; AGENTS.md CONFLICT; rules/stack.go auto-merged
go build ./state ./rules                      -> clean
go test ./internal/testutil -run TestKnownApproximations   (see below)
go test ./rules -run '<ratchets + targeted suites>'        (see below)
go test ./internal/archtest/                  (see below)
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/  (see below)
git commit (default merge message)
```

Full pasted outputs (corpus present via the `.cards` symlink, so none of these
are corpus-missing skips):

```
go test ./internal/testutil -run 'TestKnownApproximations|TestKnownOversize'
  -> ok  github.com/adams-shaun/gorge/internal/testutil 0.001s

go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|\
  TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|TestValidTgtsSpellWithoutTargetTypeSearchesStack|\
  TestTargetTypeSpellOffersOnlyStackObjectsAndCounters|TestCounterspellWithOnlyItselfOnStackFizzles|\
  TestTgtZoneGraveyardTargetOfferedAndResolves|TestOriginGraveyardAbilityTargetsGraveyardLand|\
  TestOriginGraveyardInstantSorceryTargetsGraveyard|TestTargetZonesChangeZoneOriginTable|\
  TestTargetTypeQualifiersReadAllNamedQualifiers|TestTriggeredTargetTypeKeepsKindAfterSourceLeaves|\
  Unless|TestPreselected'
  -> ok  github.com/adams-shaun/gorge/rules 1.015s   (all ratchets + both sides' targeted suites)

go test ./effects -run 'Unless|Charm|Compare|ChooseEach'
  -> ok  github.com/adams-shaun/gorge/effects 0.682s

go test ./internal/archtest/
  -> ok  github.com/adams-shaun/gorge/internal/archtest 3.142s

go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
  -> ok  github.com/adams-shaun/gorge/cmd/botbench 1.072s   (byte-identical, unmoved)

gofmt -l <changed .go files>  -> clean
```

After the commit: `git merge-base --is-ancestor main HEAD` -> MAIN-CONTAINED;
`git diff main --stat` shows exactly the branch's fix (rules/stack.go +31,
its two test files, agentsdoc constant 77, the .ds4 report).

## Notes / uncertainties

- The ValidTgts row deletion is the one judgement call: main recorded the row at
  `ffae51a54` (21:40), three minutes after this branch's last fix commit
  (21:37), without containing that fix. The row's own text calls the behaviour a
  "latent footgun"; the reviewed fix removes exactly that behaviour while
  PRESERVING main's newer origin-implied-zone route (it outranks the inference,
  per `070f673d`). If the gate disagrees, the alternative is restoring the row
  and reverting the inference — a behaviour call for the controller, flagged
  here.
- Main tracks `.ds4/report-mrg1.md`; this file replaces the round-3 report in
  the merge commit, as prior rounds did.

## Issues

None found. No new defects surfaced during integration.

---

# Merge conflict resolution mrg1 (branch wt/cli-20260922T225138Z-e21c29e8)

## State found

`git status` at start: clean, nothing in flight — the daemon's failed rebase and
merge-fallback had both been aborted, leaving the branch at `7821d4f0`
(fix(rules): enforce cast target minima at offer time). I redid the integration
with `git merge main` and resolved it there. Merge base: `83c421d0`.

## Conflicted file: internal/testutil/agentsdoc_test.go

One conflict, the `knownApproximationRows` constant:

- **Branch (HEAD, 7821d4f0):** `87` — its fix deleted one stale row from
  AGENTS.md's Known-approximations table and lowered the constant 88 → 87.
- **Main:** `82` — main's lineage independently deleted several rows and lowered
  the same constant 88 → 82.

The merged `AGENTS.md` auto-merged and contains BOTH sides' deletions; measured
with the test's own counting rule (`^| ` lines in the section minus the header
row): branch base 87, main 80, merged **79** data rows. Resolution: took **main's
value `82`** — it is the tighter constant, still above the merged 79 (the test
fails only on growth, and logs a hint when below), and it honours both sides'
deletions. Taking the branch's 87 instead would have left main's five extra
deletions unreflected in the constant. No other hunk in the file conflicted
(`knownOversizeRows` merged cleanly).

## Post-merge ratchet failure and its fix (second commit)

The brief's post-merge ratchet run failed:

```
--- FAIL: TestEveryRepoDeckParamsAreRead (0.12s)
    paramcensus rot guard: 1 findings:
        paramcensus: 1 unclassified Params reads (the census cannot rot):
        legal.go:969:54: Engine.targetSAAvailable: dynamic Params key "key"
        that is not a function parameter -- resolve it via a parameter or
        classify it
```

`main`'s param census rot guard (present at the merge base too) rejects any
`.Params[key]` read whose key is not a string function parameter. The branch's
`Engine.targetSAAvailable` (rules/legal.go, from 7821d4f0) indexed
`sa.Params[key]` through a `for _, key := range []string{"TargetMin",
"TargetMax"}` loop variable. I rewrote it to read the two bounds by literal key
— a semantic no-op (same `EqualFold(TrimSpace(...), "X")` tests on the same two
params, same `xPending` gate) that satisfies the census without touching the
offer-time minimum enforcement the branch fix introduced. No census allowlist
was touched. Fixed in `3cd4dd24 fix(rules): read cast bound params by literal
key for the param census`.

## Commands and output

- `git merge main` — conflict in `internal/testutil/agentsdoc_test.go` only;
  `AGENTS.md`, `rules/cast.go`, `rules/legal.go` auto-merged.
- `[ -e .cards ]` — present (real corpus, not a skip-run).
- `gofmt -l` on both edited files — clean.
- `go test -run 'TestKnownApproximation' ./internal/testutil/`
  → `ok github.com/adams-shaun/gorge/internal/testutil 0.001s`
- `go test ./rules -run 'TestNoTriggerModeIsRegistered|TestEveryDispatchedTriggerMode|TestEveryRepoDeck|TestEveryRepoDeckParams|CountHead|CastTarget'`
  → FAILED with the rot-guard finding above (first run); after the legal.go fix
  → `ok github.com/adams-shaun/gorge/rules 0.787s`
- `go test ./internal/archtest/` → `ok ... 3.281s`
- `go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/`
  → `ok ... 1.119s` (no split movement)
- `git status` after both commits — working tree clean.

## Commits

- `29121ab2` Merge branch 'main' into wt/cli-20260922T225138Z-e21c29e8
- `3cd4dd24` fix(rules): read cast bound params by literal key for the param census

## Notes / uncertainty

- The tracked `.ds4/report-mrg1.md` that arrived via the merge was a STALE
  report from a previous round on the `wt/...-97ad08b6` lineage (its numbers —
  84/83 rows — belong to that round, not this one). This file replaces it.
- Why the branch's own gate did not flag the census rot guard before the merge
  is unclear (the census scan logic is identical at 7821d4f0 and post-merge); an
  attempt to re-run the census on the branch's tree mixed-version and did not
  build, so I did not pursue it. The merged tree is what must pass, and it does.
- Chain heads and the full daemon gate were not run (daemon-only gates).

## Issues

- None new. The census rot guard did its job: a dynamic Params key that the
  census cannot attribute is now a literal-key read.