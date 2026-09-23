# Report — cli-20260923T060000Z-trig-attackerblocked

## Summary

Closed the ticket's one sub-shape: `Mode$ AttackerBlocked` (and its sibling
`Mode$ AttackerBlockedByCreature`) now fire `AddTrigger$`-granted instances, not
only printed face triggers. Because the row's other three sub-shapes
(AttackersDeclared, Cycled, CounterAdded) had all landed on this base, this
commit also deletes the "Four trigger modes carry limits" row from AGENTS.md and
lowers `knownApproximationRows` 21 → 20.

Commit: `76014dd4`

## Workspace facts used

- `.cards` was already a symlink in the worktree (`ls -la .cards` →
  `.cards -> /home/sadams/projects/gorge/.cards`); corpus-backed tests really
  ran (the new tests parse real corpus cards, and a missing corpus would have
  `t.Fatalf`'d on the parse). No skips.
- Sibling landing check on the branch: `git log --oneline` shows
  `b5f1a0d9 merge(...trig-attackersdeclared)`, `c37f9f03 merge(...trig-cycled)`,
  `db800365 merge(...trig-counteradded)`. All three sub-shapes are on the base,
  so the row deletion is authorised.

## What changed, per file

### `rules/trigmatch_combat.go`

The root cause: `AttackerBlocked` and `AttackerBlockedByCreature` are dedicated
hooks, not `trigMatchers` entries (`rules/trigmatch_combat.go`'s `init()` does
not register them). The ordinary granted-trigger walk
(`checkGrantedStaticTriggersUsing`) queues a grant only after
`triggerMatches(...)` returns true, and `triggerMatches` returns false when
`trigMatchers[t.Mode] == nil`. So every `AddTrigger$` grant of these two modes
was rejected and never fired.

- Extracted the per-trigger queue body of `checkAttackerBlockedTriggers` into a
  new shared helper `queueAttackerBlockedTrigger(t, source, controller, idx,
  granted, grantor, ev)`. Printed triggers and granted instances now share the
  zone/phase gates, the read-only-then-reserve limit discipline, the two
  `ValidCard$`/`ValidBlocker$` candidate walks and the reference capture. The
  helper carries `Granted`/`Grantor`/`Execute` onto the `pendingTrigger` so
  `pushTrigger` routes it through `events.GrantTriggerPush` (the replayable-grant
  path) and `events.Apply` rebuilds the `Execute$` body from the grantor's SVar
  table. This is exactly the `queueAttackerUnblockedTrigger` /
  `checkGrantedAttackerUnblockedTriggers` shape already used for the sibling
  `AttackerUnblocked` mode.
- Added `checkGrantedAttackerBlockedTriggers(ev)`, called from
  `checkAttackerBlockedTriggers` after the printed walk. It iterates the
  deterministic `e.active()` slice, selects live grants whose
  `ce.AddTrigger.Mode` is one of the two modes, resolves the grantor
  (`ce.Source`, or `ce.TriggerGrantor` for the Animate route), links the body
  with `grantedTriggerExecute`, and for each object matching `ce.Affects` queues
  through the same helper with `idx = -1` and the grant provenance. A grant
  whose body cannot be resolved queues nothing (the live==replay gate).
- The extracted helper adds an early `t.Effect == nil` return. The printed path
  previously broke out of the candidate walk on a nil effect; behaviour is
  identical (nothing queued, no limit consumed).

`attackerBlockedCandidates` itself was already correct (it reads `ValidCard$`
against each attacker) and needed no change; the fix is that granted instances
now reach it.

### `rules/attacker_blocked_grants_test.go` (new)

Two tests, both driven by real corpus cards:

- `TestGrantedAttackerBlockedByCreaturePumps` — Retaliation
  (`AddTrigger$ TrigBlocked`, `Mode$ AttackerBlockedByCreature | ValidCard$
  Card.Self | ValidBlocker$ Creature`). A granted 2/2 becomes blocked and pumps
  to 3/3.
- `TestGrantedAttackerBlockedDraws` — Stormsurge Kraken
  (`AddTrigger$ TrigBlocked`, `Mode$ AttackerBlocked | ValidCard$ Card.Self`,
  `OptionalDecider$ You` draw two). The Kraken, with a commander in play so the
  Lieutenant static is live, becomes blocked and draws two.

Each asserts its own preconditions: the recipient is on the battlefield; the
compared values differ (2/2 → 3/3; hand 0 → 2); the recipient prints NO
become-blocked trigger (so the path under test is the granted one, not a
printed line); and `GrantTriggerPush == 1` (the granted handler actually ran).
The Kraken test also asserts the static is live via 7/7 (printed 5/5 + granted
2/2), so the `IsPresent$`-gated grant is proven before the block. Both build a
`Clone()` and compare in the Retaliation case (the granted body is an SVar
resolved from the grantor's table during Apply).

### `AGENTS.md`

Deleted the row (found by its text):

> Four trigger modes carry limits. ... **AttackerBlocked** misses
> `AddTrigger$`-granted instances. | `rules/trigger_match.go` (...) | M4 (...)

No new row, no other row touched.

### `internal/testutil/agentsdoc_test.go`

`knownApproximationRows` 21 → 20, with a comment noting this ticket's deletion.

## Gates run (real output pasted)

Environment: `.cards` symlink present; `GOFLAGS=-p=2` and `GOMEMLIMIT` left at
their defaults; no `-p`/`-parallel` override.

Targeted tests (the brief's one gated command plus the fix's siblings):

```
$ go test -count=1 -run 'TestGrantedAttackerBlocked' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.007s
```

```
$ go test -run 'Afflict|Flanking|AttackerBlocked|AttackerUnblocked|AttackerUnblockedOnce|BlocksTrigger|BlockerDeclaration|MinMaxBlocker|MustBlock|Menace' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.793s
```

Ratchets that a merge with main newly enforces:

```
$ go test -run 'TestParamCensus|TestEveryRepoDeckParamsAreRead|TestNoTriggerModeIsRegisteredThatTheSwitchNeverDispatched|TestEveryDispatchedTriggerModeHasAMatcher' ./rules/
ok  	github.com/adams-shaun/gorge/rules	1.035s
```

```
$ go test -run 'TestKnownApproximation|TestKnownApproximationsOnlyShrinks|TestKnownApproximationRowsAreShort' ./internal/testutil/
ok  	github.com/adams-shaun/gorge/internal/testutil	0.001s
```

Behaviour goldens outside `rules/` (run once, before DONE):

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.637s
```

```
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.305s
```

```
$ gofmt -l rules/trigmatch_combat.go rules/attacker_blocked_grants_test.go internal/testutil/agentsdoc_test.go
(no output)
$ go build ./...
(no output)
$ go run ./cmd/gentypes -check
(no output)
```

## Fails without the fix

Restored the pre-fix `rules/trigmatch_combat.go` (saved to
`.ds4/scratch/trigmatch_combat.go.orig`), ran the new tests, then restored the
fixed file byte-identically (`cmp` against `.ds4/scratch/trigmatch_combat.go.fixed`
printed `RESTORED_BYTE_IDENTICAL`):

```
$ go test -run 'TestGrantedAttackerBlocked' ./rules/
--- FAIL: TestGrantedAttackerBlockedByCreaturePumps (0.00s)
    attacker_blocked_grants_test.go:58: Retaliation granted AttackerBlockedByCreature GrantTriggerPush events = 0, want 1
--- FAIL: TestGrantedAttackerBlockedDraws (0.00s)
    attacker_blocked_grants_test.go:117: Stormsurge Kraken granted AttackerBlocked GrantTriggerPush events = 0, want 1
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.006s
FAIL
```

Both fail at the granted-handler assertion, i.e. with the fix reverted the
granted walk does not exist and no `GrantTriggerPush` is emitted.

## Head / ratchet movement

None measured:

- `TestConstructedDefaultIsByteIdentical` (`cmd/botbench`) passed unchanged, so
  the 20-game bot win split did not move → **no botbench re-pin**.
- Confirmed no repo-deck card carries any of the three affected corpus carriers:
  `grep -rl 'Stormsurge Kraken'|'Retaliation'|'Mirror Shield'` over
  `internal/testutil/decks/` returns nothing, so `TestHeads` and the deck
  acceptance/replay goldens cannot move from this change.
- No `knownUnsupported`, `knownUnsupportedParams`, `knownUnmodelledCountHeads`
  or `registeredModes` entry changed; the four ratchet scans above pass.
- No new `Mode$` was registered (the two modes stay dedicated hooks), so the
  registry ratchet is untouched.
- No `events.Kind` change; the granted instance uses the existing
  `GrantTriggerPush` path.

## Structural fix (not the instance)

The brief names "expand the granted triggers … so a granted instance is a
candidate like a printed one". I chose the shared-helper shape: one
`queueAttackerBlockedTrigger` that both the printed walk and the granted walk
call, and one granted walk that selects grants by MODE (not by a hard-coded
list of cards or by parsing printed faces). The next sibling carrier of either
mode is covered automatically — any live `AddTrigger$` whose `Mode$` is
`AttackerBlocked`/`AttackerBlockedByCreature` flows through the same path, and
the two modes cannot drift in gates/referents because they share the helper.
A corpus carrier list is deliberately not encoded.

I did NOT touch the AttackersDeclared, Cycled or CounterAdded code (sibling
sub-shapes, out of scope).

## Deviations from the brief

None. The brief's `## Workspace facts`/row quote differ slightly from the actual
AGENTS.md text ("per declare step" vs actual wording); I deleted the row by its
actual text.

## Open concerns / caveats

1. `rules/paramcensus_test.go`'s comment above the `trig:` read-root list still
   says a granted trigger of ANY mode "matches through triggerMatches' own
   dispatch". That was already false for these two dedicated-hook modes and is
   the very defect fixed here; the granted half now lives in
   `checkGrantedAttackerBlockedTriggers`. Comment-only drift; the census tests
   pass. I left it unchanged to stay inside the brief.
2. Mirror Shield's `AddTrigger$ TrigBlocks & TrigBecomeBlocked` still does not
   fire — but for a DIFFERENT, pre-existing reason (the `&`-joined multi-name
   value is never split, so no grant registers at all). Filed as a new ticket;
   see `## Issues`.

## Issues

- **`AddTrigger$` with `&`-joined SVar names never registers** (out of scope;
  filed as `.ds4/new-tickets/addtrigger-multiname-ampersand.md`,
  Priority 2). `rules/layers.go` (`staticEffects`, AddTrigger branch) looks up
  the WHOLE `st.Params["AddTrigger"]` string in `fc.SVars`; `cards/parse.go`
  does not split a parameter value on `&`. Measured: 5 corpus files use the
  shape (`mirror_shield`, `veterans_armaments`, `astrologians_planisphere`,
  `candlekeep_sage`, `noble_heritage`); a throwaway test confirmed Mirror Shield
  registers **0** AddTrigger grants. This is the multi-name grammar shared by
  every granted mode, not the AttackerBlocked sub-shape this ticket closed
  (which is about making a REGISTERED grant fire). Fixing it changes granted
  behaviour for all modes, so it is a separate ticket.
- **`paramcensus_test.go` stale comment** (comment-only): the `trig:` read-root
  preamble claims all granted triggers dispatch through `triggerMatches`; the
  two become-blocked modes now have a dedicated granted walk. See concern 1.
- No CR-lane test was added or is proposed. The defect is a trigger-dispatch
  gap, not a CR-rule conformance gap, and the ticket brief asked for a card-level
  test only.
