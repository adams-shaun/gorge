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
