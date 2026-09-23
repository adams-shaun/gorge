# Report — r2, agent-20260918T232601Z-7cac94d8 — api:Animate.RemoveKeywords round 2

Ticket: api:Animate.RemoveKeywords is unread + the three coupled breaks
(A/B/C/D) that kept the Animate Dead reanimate chain from ever running.
Round 1's implementation landed as `d62010f2` (five homes, one chain — see its
commit message; the full per-item evidence is in `.ds4/report-r1.md`).
Round 2 was dispatched on one failing module gate; this report covers it.

Note on the shared path: this worktree's tracked `.ds4/report-r2.md` held the
Deep Spawn ticket's merge-resolution round report (ticket
agent-20260923T002719Z-c0b56143, merged via f65ac023 — the content remains in
git history). This round's report replaces it here and is additionally kept at
the ticket-unique path `.ds4/report-r2-animate-dead.md`, per the process note
the Deep Spawn round itself left in this file.

## What this round found and fixed

`findings-r2.md` pasted the module gate failure:
`TestNecromancyOffSorceryCastSacrificesItselfAtCleanup`
(rules/mayflashsac_test.go:329) died on

```
non-priority decision &{Seq:147 ... Kind:trigger_order ... Min:2 Max:2 ...}
encountered while driving to turn 2 seat 1 step main1
```

with the two options labelled `Necromancy` and the Necromancy ETB text.

**Diagnosis (measured, scratch probe against the corpus script).** I
replicated the test's exact drive in a scratch probe (since deleted) that
answers every ask and dumps `e.pendingTriggers` at the ask:

```
ASK seq=147 kind=trigger_order min=2 max=2 opts=[0:Necromancy 1:Necromancy: When CARDNAME enters, ...]
  pt[0] src=1 ctrl=0 delayed=false idx=1 api=Cleanup ... db=DB      <- printed T: line, Execute$ DBCleanup, Static$ True
  pt[1] src=1 ctrl=0 delayed=true  idx=0 api=SacrificeAll ... db=DB <- the registered DBDelay delayed trigger, Execute$ TrigSacrifice
```

Timeline from the event log: ETB trigger fires (seq 73) → bear reanimated
(85), remembered (86), Aura attached (89), `DBDelay` registered (90) → at
cleanup the MayFlashSac rider sacrifices Necromancy (144) → **two** triggers
fire in the same leave-battlefield window (149 printed DBCleanup, 150 delayed
TrigSacrifice) → CR 603.3 order ask (147) → the test's `driveToStep` driver,
which answers only priority passes and cleanup discards, dies on the ask.

Baseline check: the identical test at **unmodified main** (`git archive main`
into `.ds4/scratch/basewt`, `.cards` symlinked) **passes** — because at main
the ETB trigger was suppressed by the fail-closed `IsPresent$ Card.StrictlySelf`
gate (the defect the round-1 `effects/filter.go` registration fixed), so
`DBDelay` was never registered and the LTB fired a single trigger: the test
"passed" while the reanimation **never ran**. The gate failure is therefore
the card's real behaviour surfacing, not a regression in the chain fix.

**Why the engine's ask is correct under its established semantics.** A
`Static$ True` ChangesZone trigger is queued and pushed to the stack like any
other trigger in this build (that is pre-existing behaviour, visible on main
where DBCleanup alone fired and resolved through the stack). Two simultaneous
triggers of one controller legitimately pose the CR 603.3 order ask. Changing
Static$-True handling is a corpus-wide design question — 173 corpus files
carry `Static$ True` T: lines — and is out of this ticket's scope (filed under
Issues).

**Fix (test driver only, no engine change):** `rules/mayflashsac_test.go`
gains `driveToStepAnsweringOrder`, a local driver mirroring `driveToStep`
that additionally answers a `KTriggerOrder` with the queue's own order (an
identity permutation — a legal CR 603.3 answer), used by the Necromancy
off-sorcery leaf's final drive. The test also gains two precondition
assertions the old leaf lacked: the reanimated bear is on the battlefield
under seat 0, and Necromancy is `AttachedTo` the bear.

## Fails without the fix

The updated test (with its new precondition assertions) run against
unmodified main (`git archive main | tar -x` into `.ds4/scratch/basewt`,
`.cards` symlinked, test file copied in, then the real file `cmp`-checked
byte-identical after restoring):

```
    mayflashsac_test.go:326: bear zone graveyard controller 0, want battlefield under seat 0 (the reanimation never ran)
FAIL
FAIL	github.com/adams-shaun/gorge/rules	0.004s
```

— i.e. on main the leaf fails not on the driver but on the reanimation-never-
ran defect itself, so the leaf genuinely pins the chain. The driver-only
failing evidence on the fixed code is the findings paste above (reproduced
identically at the start of this round: `necro.log` shows the same
trigger_order death before the driver fix).

## Gates run (real output)

```
$ go test -run 'TestNecromancy' ./rules/            (after the fix)
ok  	github.com/adams-shaun/gorge/rules	0.007s

$ go test -run 'TestMayFlashSac|TestNecromancy' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.028s

$ go test -run 'TestAnimateDead|TestDanceOfTheDead|TestHeads' ./rules/
ok  	github.com/adams-shaun/gorge/rules	1.969s

$ go test -run 'TestAnimate' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.002s

$ go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	1.509s

$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.211s

$ gofmt -l rules/mayflashsac_test.go        (no output)
$ go vet ./rules ./effects ./state          (no output)
```

`.cards` was present (symlink to `/home/sadams/projects/gorge/.cards`) for
every run. TestHeads did not move (no repo deck carries Necromancy or the
three carriers — the brief's zero-golden-exposure measurement held).

## Round-1 gates re-verified

Round 1's own pins all still pass on the round-2 tree (the
`TestAnimateDead|TestDanceOfTheDead|TestHeads` and `TestAnimate` runs above).
The brief's Done-means items 1–8 were evidenced in `.ds4/report-r1.md` and are
unchanged by this round (this round touched only a test file's driver and
assertions).

## Issues

- **Delayed-trigger option labels are wrong in asks** (pre-existing, not
  fixed, cosmetic but user-visible): `triggerLabel` resolves a pending trigger
  via `triggerOf(pt)`, which indexes the source face's `Triggers` by `pt.Idx`;
  a delayed trigger queues with `Idx=0`, so the ordering ask above offered the
  delayed TrigSacrifice under the ETB trigger's description ("When CARDNAME
  enters…") instead of its own ("When CARDNAME leaves the battlefield, that
  creature's controller sacrifices it"). Any ask that includes a delayed
  trigger (order, optional) shows a sibling trigger's text. Fix shape: label a
  `pt.Delayed` entry from the registered `state.DelayedTrigger`'s own stored
  description. Measured exposure: every delayed-trigger registration whose
  source face has ≥1 T: line.
- **`Static$ True` triggers use the stack and join order asks** (pre-existing
  corpus-wide approximation, not fixed): Forge's `Static$ True` means the
  trigger executes without using the stack; this build queues it like any
  trigger (Necromancy's DBCleanup entered the CR 603.3 order ask). 173 corpus
  files carry `Static$ True` T: lines (`/usr/bin/grep -rlE '^T:.*Static\$ *True'
  .cards/cardsfolder | wc -l` = 173). A dedicated ticket should decide the
  design (execute-at-queue vs stack) — it also removes the ask this round's
  driver works around.
- Minor, benign: during the delayed TrigSacrifice resolution a Note prints
  "SacrificeAll target 1 is not on the battlefield (zone graveyard); skipped"
  — the delayed trigger's remembered set includes the already-departed source
  alongside the bear (the registration's `IDs:[1 2]`). The bear is still
  sacrificed (the card's real text); the note is noise from the LKI shape.
  No action taken.
