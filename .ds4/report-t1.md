# Report — api:Phases (CR 702.25 phasing), task agent-20260919T181525Z-3cba0676

Status: DONE.

This round rebased the existing phasing work onto current `main` and finished
verifying it. The prior run's work was already committed sound; no production
change was needed. Everything below is a measurement on the rebased branch.

## Rebase (controller directive)

The tree was clean before rebasing (nothing for a peer to lose), so:

```
$ git rebase main
Successfully rebased and updated refs/heads/wt/agent-20260919T181525Z-3cba0676.
```

Main moved twice during this round (fuzz-b10, then fuzz-b11), so the rebase
was run twice to keep the branch on the current tip. Final base: `374a568b`
(`merge(fuzz-b11)`), no divergence (`git log HEAD..main` empty). The two
ticket commits are now **`416be8e8`** and **`202151d1`** (report:
`875a67d5`). No conflict either time; both landed byte-for-byte. All gates
below were re-run on the final base.

## What the committed work does

Two commits, 20 files, ~978 insertions (see `git diff --stat main..HEAD`).

- **`state/object.go`** — `Object.PhasedOut bool`, the CR 702.25b/d/e
  phased-out status (not a zone change; a value copy in `CloneDeep` carries it).
- **`events/event.go` / `events/apply.go`** — new append-only `PhaseOut` Kind
  (Amount 1 out, -1 in). The fold is battlefield-gated; on a phase-OUT it
  removes the permanent from combat (CR 702.25c) with the exact
  `EndCombatReset{Obj}` shape, leaving the CR 509.1h zero tombstone in each
  attacker's `BlockedBy`; the `move` fold clears `PhasedOut` on a real
  battlefield departure (CR 702.25e).
- **`effects/phases.go`** — `Register("Phases", effPhases)`. Reads the chosen
  targets / `Defined$` source, `AllValid$` (deterministic arena sweep, the same
  `MatchesObjectCtx` evaluator `effGainControl` uses), Forge's `PhaseInOrOut$`
  toggle (8 corpus lines; Time and Tide), and an explicit `Phaseout$ False`
  phase-in. Each affected permanent gets its own event (CR 702.25a), and an
  already-phased-out permanent is skipped on a plain phase-out (Forge's
  `!isPhasedOut()` guard).
- **Readers gating on the status** — `rules/stack.go` `candidatesFor` (not
  targetable), `rules/layers.go` `matchesWithChars` (no continuous effect
  applies) and the own-statics gate, `rules/statics.go` `activeStatics`,
  `rules/legal.go` `existsOnBattlefield` (one predicate: no activation/mana/
  granted/Station/Room/max-speed ability offered), `rules/sba.go` (5 scans),
  `rules/attach.go`, `rules/combat.go` `attackableCreature`/`canBlock`,
  `view/view.go` `cardViews` (absent from every projection),
  `view/describe.go` (renders the event), `rules/turn.go` `finishUntapStep`
  (phases in at the controller's untap step, CR 702.25d / CR 502.4).
- **Ratchet** — `rules/acceptance_test.go` `knownUnsupported` loses
  `"Vision, Synthezoid Avenger": {"api:Phases"}` (see below).
- **Tests** — `rules/phases_test.go`, `rules/phasedout_nonexistence_test.go`,
  `rules/cr702_phasing_conformance_test.go`, `view/phasedout_test.go`,
  `effects/phases_allvalid_test.go`.

## Gates run (real output)

Targeted engine suite (brief's carriers + CR lane + ratchets):

```
$ go build ./...
(no output)
$ go test -run 'TestPhases|TestTalonGates|TestGuardianOfFaith|TestPhaseoutFalse|TestPhasedOutPermanent|TestCR702|TestEveryRepoDeckIsFullySupported|TestEveryRepoDeckParamsAreRead' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.646s
```

CR conformance lane (`make conformance`'s exact command):

```
$ go test -count=1 ./rules -run TestCR -v          # exit 0
--- PASS: TestCR702PhasedOutPermanentIsNotATarget (0.00s)
--- PASS: TestCR702PhasedOutPermanentStaticAbilityIsOff (0.00s)
--- PASS: TestCR702PhasedOutPermanentDoesNotAttack (0.00s)
--- PASS: TestCR702PhasedOutPermanentPhasesInAtUntap (0.00s)
PASS
ok  	github.com/adams-shaun/gorge/rules	1.465s
```

Edited packages with no `-run` target (run once each):

```
$ go test ./events/
ok  	github.com/adams-shaun/gorge/events	(cached)
$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	15.581s
$ go test ./view/
ok  	github.com/adams-shaun/gorge/view	0.529s
```

Mandatory goldens (dispatch's "Behaviour goldens outside `rules/`"):

```
$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	2.852s
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	0.630s
```

The botbench split did **not** move — no re-pin needed.

Format / generated-types prerequisites:

```
$ gofmt -l effects/ events/ rules/ state/ view/
(no output)
$ go run ./cmd/gentypes -check          # exit 0
```

Daemon-only gates not run in the seat, per dispatch/system notes: `make sim`,
TestHeads, `make report`, `go vet ./...`. The full `make conformance` target
(the `-v` lane) was run at its exact command above, exit 0.

## Corpus measurements (brief's counts re-checked)

The brief's `71 lines / 56 files` held exactly:

```
$ /usr/bin/grep -rE '\$ Phases\b' .cards/cardsfolder | wc -l
71
$ /usr/bin/grep -rlE '\$ Phases\b' .cards/cardsfolder | wc -l
56
$ /usr/bin/grep -rlE 'PhaseInOrOut\$' .cards/cardsfolder | wc -l
8
$ /usr/bin/grep -rE '\$ Phases\b' .cards/cardsfolder | grep -c 'AllValid'
11
```

## Ratchet movement, with measured cause

The brief predicted `knownUnsupported` would "lose the entry for the deck
card once the deck file is added". Measured: **no deck file was added and no
Talon Gates entry existed.**

```
$ /usr/bin/grep -rl "Talon Gates" internal/testutil/decks/      # (nothing)
$ /usr/bin/grep -rl "Vision, Synthezoid" internal/testutil/decks/
internal/testutil/decks/avengers-assemble.json
```

What actually moved is `"Vision, Synthezoid Avenger": {"api:Phases"}`
(`rules/acceptance_test.go`), whose only gap was `api:Phases` (its Charm
branch reaches `DB$ Phases | Defined$ Self`). Registering the primitive made
that entry stale and the ratchet's bidirectional assertion (`Ruling R-20`)
requires deleting a stale entry. Deletion is licensed by the real card tests.
`TestEveryRepoDeckIsFullySupported` passes in both directions.
`TestEveryRepoDeckParamsAreRead` passes — no new param-census entry is needed
for `AllValid$`/`PhaseInOrOut$`/`Phaseout$` because no repo-deck carrier holds
them unread.

`make report`'s playable-count rise was not measured (daemon gate).

## Fails without the fix (mandatory proof, per dispatch)

Each non-test hunk was reverted in place, the targeted test run, then the file
restored from a byte-identical `.ds4/scratch/*.bak` copy (verified with `cmp`).

1. **`effects/phases.go` registration removed** (`Register("Phases", ...)`):

```
--- FAIL: TestPhasesPrimitiveSupported (0.00s)
    phases_test.go:25: effects.Supported() is missing api:Phases
--- FAIL: TestTalonGatesOfMadaraPhasesOutAndIn (0.40s)
    phases_test.go:84: api:Phases is not registered (unimplemented fallback Note present)
--- FAIL: TestGuardianOfFaithPhasesOutOtherCreatures (0.00s)
    phases_test.go:122: Bears were not phased out by Guardian of Faith
```

2. **`rules/turn.go` phase-in block disabled** (`if next == 0` → `if false`):

```
--- FAIL: TestCR702PhasedOutPermanentPhasesInAtUntap (0.40s)
    cr702_phasing_conformance_test.go:100: CR 702.25d: a phased-out permanent did not phase in at its controller's untap step
--- FAIL: TestTalonGatesOfMadaraPhasesOutAndIn (0.00s)
    phases_test.go:94: Bears did not phase in at its controller's next untap step
```

3. **`events/apply.go` PhaseOut combat-removal block removed**:

```
--- FAIL: TestCR702PhasedOutAttackerIsRemovedFromCombat (0.39s)
    phasedout_nonexistence_test.go:85: CR 702.25c: a phased-out attacker was not removed from combat
--- FAIL: TestCR702PhasedOutBlockerLeavesZeroTombstone (0.00s)
    phasedout_nonexistence_test.go:112: CR 509.1h: attacker BlockedBy = [2], want [0] (a zero tombstone keeping it blocked)
```

4. **`rules/legal.go` `existsOnBattlefield` blind to `PhasedOut`**:

```
--- FAIL: TestCR702PhasedOutPermanentActivatedAbilityNotOffered (0.00s)
    phasedout_nonexistence_test.go:57: CR 702.25b: a phased-out permanent's activated ability was offered
```

5. **`rules/layers.go` `matchesWithChars` gate disabled**:

```
--- FAIL: TestCR702PhasedOutPermanentStaticAbilityIsOff (0.41s)
    cr702_phasing_conformance_test.go:64: CR 702.25b: phased-out Bears power = 3, want 2 (the lord's static must be off)
```

6. **`rules/stack.go` `candidatesFor` `!o.PhasedOut` dropped**:

```
--- FAIL: TestCR702PhasedOutPermanentIsNotATarget (0.44s)
    cr702_phasing_conformance_test.go:43: CR 702.25b: a phased-out permanent was offered as a target
```

Every test asserts its precondition first (the object is a battlefield
permanent and phased-in; the census/lord/offer actually reaches it), and the
Talon Gates test asserts the `unimplemented API Phases` Note is absent, so a
"nothing happened" pass is impossible with the primitive unregistered.

## Workspace state

```
$ git status --short
(nothing to commit, working tree clean)
$ git merge-base HEAD main
374a568b13d87060dd0398bc705809e4ad8673c9
$ git log --oneline HEAD..main
(empty)
```

`.cards` was present (symlink to the real corpus) before the first test run —
this is a real green run, not a skipped one.

## Deviations from the brief

- The brief's ratchet prediction (deck card entry) did not match the tree; the
  actual stale entry (Vision) was removed. See above. Reporting the brief's
  premise as false, per the dispatch's own instruction.
- The brief's "make conformance currently RED for that leaf" is the pre-fix
  condition; the lane is GREEN now, and the pre-fix RED is proven by the
  revert proofs above.
- No deck file was added, exactly as the brief states.

## Issues

Defects found and deliberately NOT fixed in this ticket; the two follow-ups
below were already written to `.ds4/new-tickets/` by the prior (lost) run and
picked up:

- **`api:Phases` riders remain unread** — `RememberAffected$ True` (Out of
  Time, Unyaro), `WontPhaseInNormal$ True` (Out of Time, Unyaro, Oubliette),
  `Tapped$`/`Untapped$` on a phase-in body (Oubliette), and the
  `T:Mode$ PhaseOutAll` trigger (The War Doctor). `RememberAffected$` +
  `WontPhaseInNormal$` are a pair: without them Out of Time's Remembered set
  is empty and its phase-in body sweeps nothing, so the creatures stay phased
  out forever — a visible board-state bug. File: `effects/phases.go`
  (`effPhases`); trigger mode in `rules/trigmatch_registry_test.go`.
  Filed: `.ds4/new-tickets/phases-riders-rememberaffected-phaseoutall.md`.
- **`K:Phasing` (CR 702.26), the phasing KEYWORD, is unregistered** — 13
  corpus files carry `^K:Phasing`; a bare keyword line produces no unsupported
  primitive, so `make report` counts these cards playable and
  `knownUnsupported` does not list them. They are wrong in silence (Katabatic
  Winds never phases itself out). File: `cards/keywords.go` +
  `rules/turn.go`. Filed:
  `.ds4/new-tickets/phasing-keyword-cr702-26.md`.
- **Nested phasing of attachments** — CR 702.25b says "anything attached to
  it" treats the bearer as nonexistent. The build relies on the bearer being
  phased out via `matchesWithChars`; a phased-out Aura/Equipment with a
  phased-in bearer was not specially probed. No corpus carrier measured for the
  reverse direction. File: `rules/layers.go`, `rules/attach.go`.
- No CR-lane test exists for `WontPhaseInNormal$`/attachment nesting; both
  would cite CR 702.25d/702.25b. Named here so they stop being invisible to
  the ledger.

Nothing that was in the ledger was closed by this ticket.
