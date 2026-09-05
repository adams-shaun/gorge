# Final whole-branch review — gorge @ 5db64f2 (75 commits, root eaeca1e)

### Verdict — Ready to merge? **With fixes**

One Critical, one Important, and the final-wave comment sweep. The Critical is a
non-terminating match reachable from card data this build's own coverage gate
reports as fully playable (24 corpus cards, including Rest in Peace and Dryad
Militant); it is pre-existing, unchanged since c19097f, and Task 29 closed only
one of the four stack-exit paths that carry it. Everything the M1 milestone row
names is green and re-measured at HEAD: the four acceptance chain heads match to
the digit, `go test ./...` and `go test -race ./...` are clean, `make lint` is
clean, `make sim` is 20/20 with replay verification, `make report` prints
`cards: 33667 playable: 15265 (45.3%)` exactly, and the ratchet's 35 entries are
all genuine unimplemented primitives. Every global constraint holds: zero direct
state writes outside `state/` and `events/`, zero `time.Now`, zero global
`math/rand`, zero `effects → rules` imports, an append-only `Kind` block with a
byte-identical `Event`/`Append` since Task 9, and thirteen map ranges none of
which can reach an event, a decision's option order, view output or mtgsim
output. Hidden-information and replay fidelity measured zero real defects.

---

### What I read and ran

**Read (whole file, in dependency order).** `cards/`: ir.go, parse.go, link.go,
registry.go, validate.go, primitive.go, intrinsic.go, face.go, fetch.go (heads),
boundary_test.go. `state/`: game.go, object.go, ids.go, continuous.go,
pending.go. `decision/decision.go`. `events/`: event.go, log.go, apply.go.
`effects/`: registry.go, context.go, filter.go, count.go (tail), zone.go,
cardflow.go, misc.go, damage.go, life.go, counters.go, combatfx.go.
`rules/`: engine.go, turn.go, stack.go, legal.go, mana.go, sba.go, combat.go,
trigger.go (all 1062 lines), layers.go, statics.go, rng.go.
`view/`: view.go, redact.go. `seat/`: seat.go, bot.go. `replay/replay.go`.
`internal/testutil/`: invariants.go, decks.go, sampledecks.go.
`cmd/mtgsim/main.go`, `cmd/forgec/main.go` (report path).
Tests read for gate content: rules/acceptance_test.go, rules/coverage_test.go,
rules/sba_test.go (Task 22 pins), rules/replacement_updated_test.go,
rules/testbot_test.go, cards/parse_test.go:118-128, view/view_test.go:438-452,
rules/fix1_test.go (head + :390-400).
Ledger: `progress.md` at lines 460, 1336-1372, 1717, 1878-1935, 2620-2660,
3130-3160, 3215-3220, 3688-3702, 3765-3775, 3840-3852, 4110-4120; plus
`final-review-deferred.md`, `task-1-brief.md`.

**Ran (gates).**

| command | result | wall |
|---|---|---|
| `go test ./...` | ok, all 12 packages, exit 0 | **17.59 s** |
| `CGO_ENABLED=1 go test -race -count=1 ./...` | ok, all 12 packages, exit 0 | **166.75 s** |
| `make lint` | `gofmt -l .` empty, `go vet ./...` clean, exit 0 | 1 s |
| `mtgsim -seats 4 -games 20 -verify` | 20/20 terminated, **20/20 `replay OK`** | **1.80 s** |
| `forgec report -dir .cards` | `cards: 33667  playable: 15265 (45.3%)` — exact match | 3 s |
| `git ls-files \| grep -i '\.txt$'` | **0** | — |
| `grep -rn "go:embed"` | **1** hit, `internal/testutil/decks.go:25` → `decks/*.json` | — |
| `strings bin/mtgsim \| grep -c 'Oracle:'` / `'^Name:'` | **0 / 0** (no Forge script text in build outputs) | — |
| test/fuzz function count | **406** `func Test`, 0 `func Fuzz` | — |

Binaries were built to the scratchpad, not `bin/`; nothing in the checkout was
mutated (working tree, index and HEAD untouched; `git status --porcelain` empty
at start and end).

**Acceptance chain heads at 5db64f2** (`go test ./rules -run TestRepoDecksPlayAtEverySeatCount -v`) — all four confirmed identical to the controller's 4820330 measurement and the Task 29 re-review:

| seats | intents | events | turns | winner | chain |
|---|---|---|---|---|---|
| 2 | 345 | 1994 | 15 | death-n-taxes | `7705a6505954f6cd` ✓ |
| 4 | 1188 | 6210 | 35 | mono-black-aggro | `2d5589b31c4853cd` ✓ |
| 6 | 2410 | 11800 | 51 | mono-green-stompy | `bf4012092fdad38b` ✓ |
| 8 | 3800 | 17788 | 61 | mono-green-stompy | `01b9f48c1b6dc135` ✓ |

**Probes built outside the repo** — scratch module `probe` (`replace
github.com/adams-shaun/gorge => /home/sadams/projects/gorge`, `CGO_ENABLED=0`),
plus a type-checking AST tool run with cwd = the module root. `internal/testutil`
is not importable across the module boundary, so deck loading, invariants and the
bot policy were all re-implemented in the probe against the public API only.

| probe | measurement |
|---|---|
| **totality** (17 malformed Configs: 0/1/2..9 seats, zero-card decks, nil decks, nil cards, more decks than names, more names than decks, empty names) | **0 panics, 0 stalls.** 7 hostile intents per real decision (wrong player, stale Seq, index == len, −1, duplicate, PlayerID 200, 1<<30): **0 accepted, 0 panics.** Duplicate submits of an already-accepted intent: **300/300 rejected**, 0 accepted. Longest run 30 368 intents (9 seats), all reached `Over`. |
| **AST: map ranges** (go/types over 12 packages, non-test code) | **13**, all provably order-safe: `cards/primitive.go:36,52` and `cards/validate.go:70` sort before returning; `cmd/forgec/main.go:207` accumulates counts only, `:239` sorts with a name tiebreak; `effects/count.go:157` map→map copy; `effects/filter.go:270` sorts; `effects/registry.go:77,90,93,105,131,134` are copy-on-write map copies and `Supported()`'s map build. **0 reach an event, a decision's option order, view output or mtgsim output.** |
| **AST: direct state mutation** | **0** assignments or `++/--` whose LHS selector has receiver type `state.*` outside `state/` and `events/`. The only non-`events` writers are `rules/engine.go:95,97` (`AddObject`/`SetZone` in genesis, documented as pre-log). |
| **events.Kind append-only** | `git diff aeae6cf..HEAD` on the `Event` struct: **IDENTICAL**. On `func (e Event) Append`: **IDENTICAL**. Kind ordinals 0-22 unchanged, six appended (LandPlayed…EndCombatReset); `kindNames` has exactly 29 entries in const order. |
| **replay tamper** (18 mutations + 3 000 randomized Apply-fuzz trials, 8 mutations each) | Every tamper detected — flipped Kind, out-of-range Zone/Step/Player/Obj, truncation to half and to 0, reorder, duplicated Seq, duplicated event, changed seed → `*replay.Divergence` naming the first differing Seq; intent Seq/player/choice/order/truncation → plain wrapped rejection. **0 panics in replay, 0 in `ReplayTo` across n ∈ {−1000,−1,0,1,7,half,all,all+1,1<<20}, 0/3000 in `events.Apply` fuzz.** |
| **replay fidelity** (12 games: 2/4/6/8 seats × 3 seeds, real repo decks, `seat.Bot`) | **0 failures.** `Replay` head == live head, `RNGDraws` equal (118/236/354/472 by seat count), `HeadAt` equal, `ReplayTo` at 0/¼/½/¾/1 prefix-head equal, **the event log folded ALONE into a fresh `state.Game` diffed byte-for-byte against the live game — zero diffs** (scalars, every player, every object field, every zone list), JSON round-trip of the log still replays to the same head. |
| **view leak walk** (16 977 decision points, real repo decks, 2/4/6/8 seats × 2 seeds, every seat **plus a spectator index past the end**, `view.Project` and `view.RedactEvents` both marshalled to JSON and every id-shaped field cross-checked against the hidden zones the viewer does not own) | **0 genuine leaks.** 88 `view.stack[].source` and 24 `MoveZone battlefield→hand` hits are the two documented-public shapes (R3 makes the stack observable; redact.go rule 2 says "a move FROM a public zone stays public"), all naming a permanent that was public on the battlefield before it bounced. **0 wire-shape violations** (`Stack`/`Pending`/`Players`/`Targets` non-nil everywhere, Ruling T23-u). |
| **R1/R2/R3 as a user experiences them** (public API only, crafted 2-seat deck) | **R1:** 113 `KTriggerOrder` decisions; on all 97 where the first push was observable, the trigger chosen first was pushed first (Ruling U2's direction) — **0 violations**. **R2:** 352 `KTriggerOptional` decisions, 176 declined; declining pushed nothing and changed nothing — **176/176 honoured, 0 violations**. **R3:** at 2 436 points with a non-empty stack and 465 with a non-empty pending queue, the `Stack`+`Pending` JSON was **byte-identical for every seat and the spectator**; `StackView.Targets` populated at 4 111 seat-views. |
| **Task 22's four pins** | All green at HEAD: `TestDestroyLethalDamageDoesNotAmplify…`, `TestRemovePermanentsDoesNotAmplify…` (+2 life/Submit), `TestRemovalSweepFiringsDoNotScaleWithAnUnrelatedDeathChain` (**flat 2 across chains 0/1/5/20/60**), `TestNoPendingDecisionWithAZeroLifePlayerNotYetLost{,ViaRemovalSweep}`, and the N6 pin `TestLethalDamageIsRetriedWhenTheReplacementsControllerIsEliminatedMidCall` (rules/sba_test.go:606). **No movement.** |
| **independent SBA/invariant walk** (23 978 decision points, 12 repo-deck games) | CR 704.5a outstanding (life ≤ 0, not Lost): **0**. CR 704.5f/g outstanding (lethal creature still on the battlefield): **0**. Decisions asked of an eliminated seat: **0**. Zone/one-zone/negative-counter/one-survivor invariant violations (re-implemented externally): **0**. N9's residue is not observable from the repo decks. |
| **ratchet genuineness** (Ruling W2) | 136 distinct cards, 35 not fully supported — matches `knownUnsupported` exactly. All 22 distinct primitives named are genuinely absent from `effects.Supported()`: **0 entries name an already-registered primitive.** 2 APIs (`api:Token` ×4, `api:CopySpellAbility` ×1), 20 keywords. |
| **F7 bot mirror drift** | `seat/bot.go botDecide` vs `rules/testbot_test.go answer`, and both `clamp`s, diffed after stripping comments and normalising the rng receiver: **0 differing lines. No drift.** |
| **Task 29 `Updated` semantics** | Two `Updated` replacements matching one entering creature (7 life vs 101 life, 40 games, both cast orders): **exactly one applies, never both** (0 occurrences of +108), fully deterministic — every seed re-run in-process produced an identical chain head. Winner is `forEachObject` position, not cast order, not a map range. An `Updated` `ReplaceWith$` that itself relocates the object (battlefield → exile): 12 seeds, **0 panics, 0 stalls**, every Cub ends in exile, stack empty. A `Card`-matching `Updated` replacement evaluated against a **Face-less** object (a resolved ability's own stack object — the only token-shaped object this build produces, since `api:Token` is unimplemented): matches via `matchesBase("Card")`, original move applies, `ReplaceWith$` runs, **no panic, no stall**. The guard's own shape (`Replaced` ETB + broad graveyard replacement) is pinned in-repo at **passes = 2, exactly 1 `Note`, next decision reached** and is green. |

---

### Strengths

The single-mutation-path discipline is real and now machine-verifiable: a
type-checked walk of every package finds **zero** direct writes to `state`
fields outside `state/` and `events/`, and the log folded alone into a fresh
`Game` reproduces twelve full multi-seat games with **zero** diffs. Determinism
is enforced structurally rather than by convention — one PCG in `rules/rng.go`,
one in the bot, no clock, and thirteen map ranges every one of which sorts or
accumulates. The comments are an unusually good durable record of *why*, and the
Task 22 sba.go header in particular is the best example of "measured, not
argued" I have read in this codebase. Totality holds under genuinely hostile
input: 17 malformed configs, 7 bad intents per decision, 3 000 randomized
tampered logs — zero panics anywhere.

---

## Issues

### Critical

**C1 — `rules/stack.go:347` (and `:212`, `:250`, `:281`, `:136`): three of the
four ways an object leaves the stack are unguarded against a replacement
discarding the move, producing a non-terminating match.**

*What is wrong.* Task 29 added a totality guard for exactly one stack exit — a
permanent spell's `Stack → Battlefield` ETB (`stack.go:292`, guarded at
`:294-333`). The same hazard exists verbatim on every other exit and is
unguarded:

- `:347` a resolved **instant/sorcery** → graveyard  ← reproduced
- `:281` a **fizzled spell** → graveyard
- `:250` a resolved **ability object** → exile (Task 29 noted this as I-2)
- `:212` a **fizzled ability** → exile
- `:136` `askTarget`'s cast-time "countered: no legal targets" → graveyard

If a `ReplacementResult$`-absent (i.e. this build's *Replaced* path)
`R:Event$ Moved` replacement matches that move, `applyReplacements` discards it,
nothing else ever removes the object from `e.G.Stack`, and the next priority
round resolves the same object again — **re-running its effect every time**.

*How I measured it.* A probe outside the repo, driving `rules.New`/`Submit`
through the public API only, with a two-card deck: an instant, plus an
enchantment carrying the corpus's own Rest in Peace text (the exact string
`rules/replacement_updated_test.go` already ships as
`graveyardBlockingReplacementSrc`, narrowed to `ValidCard$ Instant,Sorcery`,
which is Dryad Militant's printed line):

```
intents=100000  over=false  turn=1  stack=[Zap]  events=500023
*** NON-TERMINATION ***
```

100 000 intents, half a million events, still on turn 1, the same spell on the
stack. With a *damaging* instant instead the loop is also a rules catastrophe: it
re-resolved ~20 times and killed a player from 20 life in 48 intents. The
ability-object variant (I-2) reproduces identically: `intents=300000 over=false
turn=2 stackDepth=1 events=1650016`.

*Reachability.* Not hypothetical. `grep` over `.cards/cardsfolder`: **91**
`R:Event$ Moved | Destination$ Graveyard` lines carry no `ReplacementResult$`,
and 45 of them are not narrowed to `Card.Self`. Cross-checking those against
`effects.Supported()` + `Registry.Unsupported`: **24 of them are cards this build
already reports as FULLY PLAYABLE**, so the coverage gate a deck-builder consumes
would bless them — including **Rest in Peace** (`ValidCard$ Card`), **Dryad
Militant** (`ValidCard$ Instant,Sorcery`), Leyline of the Void, Dauthi
Voidwalker, Anafenza the Foremost, Forbidden Crypt, Abandoned Sarcophagus,
Emet-Selch, Haakon, Void Maw, Rayami, Misery's Shadow, Liesa, Draugr Necromancer,
Ravenous Slime, Lorcan, Glimpse the Cosmos, Stone of Erech, The Darkness Crystal,
Yawgmoth's Agenda, Festival of Embers, Frostwielder, Gisa, Kumano's Pupils.
`repl:Moved` is a registered primitive, so nothing gates these out.

*Pre-existing, not a Task 29 regression.* `git show c19097f:rules/stack.go`
confirms the non-permanent branch is byte-identical before Task 29; Task 29
guarded one path and explicitly noted only the ability path as out of scope. The
instant/sorcery path is noted nowhere in the ledger.

*The fix.* Extract Task 29's existing guard body (`stack.go:333-346`) into a
method and call it at every stack exit:

```go
// ensureLeftTheStack is CR 608.2m housekeeping. Whatever route this object was
// meant to take off the stack, if a replacement discarded that move the object
// must still leave -- otherwise resolveTop finds it on top again and resolves
// it forever. Held under applyingReplacement (saved and restored) for the same
// reason the ETB guard is: "ceases to exist" is not a game event a card's own
// replacement may intercept (Task 29 review finding I-1).
func (e *Engine) ensureLeftTheStack(id state.ObjID, to state.Zone, why string) {
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZStack {
		return
	}
	saved := e.applyingReplacement
	e.applyingReplacement = true
	e.emit(events.Event{Kind: events.Note, Obj: id, Text: why})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: to})
	e.applyingReplacement = saved
}
```

Call it immediately after the emits at `:136` (→ `ZGraveyard`), `:212`
(→ `ZExile`), `:250` (→ `ZExile`), `:281` (→ `ZGraveyard`), `:347`
(→ `ZGraveyard`), and use it in place of the inline block at `:333-346` so there
is one implementation. Keep the existing ETB `Note` wording at the ETB site so
`TestPermanentSpellWhoseEntryIsFullyReplacedDoesNotStickOnTheStack` and
`TestTotalityGuardSurvivesABroadGraveyardReplacement` (which assert exactly one
`Note` and `passes == 2`) stay green.

*Regression tests to add* (both reproduce above, both fail before the fix): an
instant resolving under a `ValidCard$ Instant,Sorcery` graveyard replacement, and
a triggered ability resolving under a `Destination$ Exile | ValidCard$ Card`
replacement. Assert the stack drains within a bounded pass count and exactly one
`Note` per swept object.

*Chain-head safety.* The guard is a no-op unless the object is still on the stack
after its own move, which never happens in the 12 repo decks — so the four
acceptance chain heads must be unchanged. **Re-verify them after the fix; any
movement means the guard fired somewhere it should not have.**

---

### Important

**I2 — `view/redact.go:93,97,122`: `RedactEvents` returns events whose `IDs`/`Pairs`
alias the engine's live log, and its doc promises the opposite. A read path can
silently destroy the match's audit chain.**

*What is wrong.* The loop is `for _, e := range evs` — `e` is a struct copy, but
`e.IDs`/`e.Pairs` still point at the caller's backing arrays. Three branches
append `e` without touching them: the owner's own `Secret` event (`:93`), the
`g == nil` degrade (`:97`), and — via the `case events.Note:` no-op — the
non-`Secret` `Note` path falling through to `:122`. The package doc at
`redact.go:8-11` and `:31` says "building a copy of each — the log … must never
be touched by a read path" and "applied to a COPY of each event — the input
slice, and every event in it, is never mutated". That is false today. This is the
exact defect class `events/log.go`'s `Append` copy was added to close (6334c71).

*How I measured it.* Redacting a real 2-seat repo-deck game's whole log for seat
0 returns **50** events whose `IDs[0]` is the same address as the engine's own
logged event — all of them that seat's `Shuffle`, i.e. their entire library
order. Mutating the *client's* copy of one:

```
mutating the CLIENT's copy of event 1 (shuffle) IDs[0] from 3 to 0
engine log event 1 IDs[0] is now 0
Head()   before=87e09ae4bf846697 after=87e09ae4bf846697
HeadAt() before=87e09ae4bf846697 after=4d9a716f7a056fe0
RESULT: Head() and HeadAt() now DISAGREE
```

`Head()` (the already-folded rolling chain) and `HeadAt()` (recomputed from the
stored events) permanently disagree, so `replay.Replay` of that match fails for
the rest of time — caused entirely by a caller reading its own view.

*The fix.* Deep-copy on every path. Simplest and uniform: at the top of the loop
body, before any branch,

```go
e.IDs = append([]state.ObjID(nil), e.IDs...)
e.Pairs = append([][2]state.ObjID(nil), e.Pairs...)
```

(the existing `filterVisible`/`filterVisiblePairs` already return fresh slices, so
the rule-3 branch is unaffected and the doc becomes true). Then add a test that
mutates a returned event's `IDs` and asserts `l.Head() == l.HeadAt(len(l.Events))`
still holds. Do this in the same edit as **T23-z** below, which touches the same
switch.

---

### Minor

- **M1 `rules/trigger.go:817-822`** — "M1 has no combat-damage implementation
  (`dealCombatDamage` is a no-op stub, Task 21/22's territory)". Stale since
  9de31df: `rules/combat.go:224` implements it fully. The *behaviour*
  (`CombatDamage$ True` never fires) is still correct, but only because
  `events.Event` carries no combat/noncombat discriminator — keep that half,
  delete the stub claim.
- **M2 `rules/statics.go:167-172`** — `blockRestricted`: "Nothing calls this yet
  … `askBlockers` and `handleBlockers` are still `stubs.go`'s no-ops". Stale:
  `rules/combat.go:76` (`canBlock`) calls it, and `stubs.go` was deleted by
  Task 22.
- **M3 `rules/layers.go:80-83`** — `EndOfTurnCleanup`: "Nothing in this package
  calls it". Stale: `rules/combat.go:436` (`cleanupStep`) calls it.
- **M4 `rules/statics_test.go:183`** — same staleness ("askBlockers/handleBlockers
  is still Task 21/22's stub").
- **M5 `rules/fix1_test.go:396-397`** — "resolveAbility is still the empty stub
  this task's brief adds (replaced in Task 14)". Stale; resolveAbility is fully
  implemented. The test is still correct (it asserts no pool change), only the
  reason is wrong.
- **M6 (T21-i) — wrong ruling numbers, 10 citations in 6 files.** Canonical map
  (progress.md:1910-1918): `T21-a` = First Strike, `T21-b` = multi-block
  assignment, `T21-c` = askBlockers nil panic, `T21-d` = CR 509.1h, `T21-e` =
  EndCombatReset/replay, `T21-f` = onBoard SummonSick. Exact edits:
  | file:line | says | should say |
  |---|---|---|
  | `events/event.go:93` | T21-a | **T21-e** |
  | `rules/turn.go:33` | T21-a | **T21-e** |
  | `events/apply_test.go:803` | T21-a | **T21-e** |
  | `rules/combat_test.go:521` | T21-a | **T21-e** |
  | `rules/combat.go:156` | T21-b | **T21-c** |
  | `rules/combat_test.go:443` | T21-b | **T21-c** |
  | `rules/combat.go:303` | T21-c | **T21-a** |
  | `rules/combat.go:384` | T21-c | **T21-a** |
  | `rules/combat_test.go:373` | T21-c | **T21-a** |
  | `rules/combat.go:346` | T21-e | **T21-b** |
  | `rules/combat_test.go:408` | T21-e | **T21-b** |
  (`T21-d` ×2 and `T21-f` ×2 are already correct — do not touch them.) I
  cross-checked every one of the 79 distinct `Ruling X` ids cited anywhere in Go
  source against `progress.md`: only `T22-c` has no `Ruling T22-c` line in the
  ledger (it is described there without the "Ruling" prefix); the rest resolve.
- **M7 `rules/trigger.go:968`** — comment headed "Review finding I-3 (Task 29 fix
  round 1)" carries **M-6**'s content (the case-sensitive `"Updated"` compare).
  The real I-3 block is at `:989`. Relabel `:968` to M-6.
- **M8 `rules/turn.go:254-255`** — the T28-b worked example calls `drainerSrc` a
  "whenever you draw a card, lose life equal to your life total" trigger;
  `rules/trigger_order_test.go:27-32` defines it as `Mode$ Phase | Phase$ Upkeep`.
  Argument correct, quote wrong.
- **M9 `view/view_test.go:444-446`** — orphaned doc comment for a deleted test
  (`TestRedactEventsStripsNoteCarryingAnotherSeatsLibraryIDs`) sitting directly
  above `TestRedactEventsPassesANonSecretNoteThroughUnchanged`, which proves the
  opposite. Delete it.
- **M10 `cards/parse_test.go:123`** — 130 characters (measured). Reflow.
- **M11 `rules/fix1_test.go`** — filename carries no meaning. Rename to what it
  actually holds (genesis/replay-through-Submit regressions), e.g.
  `genesis_replay_test.go`.
- **M12 `replay/replay.go:111`** — the runs-short path returns a plain
  `fmt.Errorf` with no `Seq`, unlike its mirror-image `Divergence{Missing:true}`.
  `cmd/mtgsim/printReplayOutcome` therefore prints "replay error:" instead of a
  located divergence. Cosmetic but it degrades mtgsim's diagnosis.
- **M13 `rules/legal.go:66-67`** — "Task 18 widens this once activated abilities
  with real costs land". Task 18 landed; the widening did not. This is T19c-b
  item 3; restate it as the parked limitation it is rather than as a pending task.
- **M14 file sizes** — `rules/trigger.go` **1062** lines (next largest
  `rules/sba.go` 484, `rules/combat.go` 455, `view/view.go` 423,
  `rules/stack.go` 417). `effects/*` are all ≤ 275. Only trigger.go is past
  reason: it holds three separable concerns (the trigger queue/drain, the seven
  `*Matches` predicates, and `applyReplacements`/`replacementMatches` — which
  Task 20 already booked for a split at progress.md:1523). Not blocking; split
  after merge. Test-side, `rules/trigger_order_test.go` 1233 and
  `view/view_test.go` 1376 lead; `effects/primitives_test.go` is 1010 (the
  ledger's "780 lines" note at :1092 is out of date).

---

## Deferred / parked triage

Exactly one fix wave follows, so this is the whole worklist.

| ledger line | item | must-fix before merge? | reason | fix if must-fix |
|---|---|---|---|---|
| — (new, this review) | **C1** resolveTop's other three stack exits unguarded → non-terminating match | **YES — Critical** | Reproduced: 100 000 intents / 500 023 events / turn 1, never ends. 24 corpus cards this build calls fully playable trigger it. "Any reachable unbounded loop is Critical." | `ensureLeftTheStack` helper called at stack.go:136/212/250/281/347, replacing the inline block at :333-346. Two regression tests. Re-verify the four chain heads. |
| 3693 | **I2** `view/redact.go` doc claims a copy; pass-through paths alias `IDs`/`Pairs` | **YES — Important** | Not just a doc bug: measured 50 aliasing events per game; one mutation from a pure read path desyncs `Head()` from `HeadAt()` and permanently breaks replay of that match. | Deep-copy `IDs`/`Pairs` at the top of `RedactEvents`' loop body; add the Head/HeadAt mutation test. |
| 3697-3699 | **T23-z** rule 2 filters only `Obj` on zone-move kinds; `IDs`/`Pairs` filtered by nothing | **YES — plan-mandated** | Ruling T23-z explicitly assigns it to the final wave ("allowlist shape everywhere"). Behaviour-neutral today: I measured **0** non-test emitters of `MoveZone`/`Draw`/`PutOnStack` carrying `IDs` or `Pairs`, so no chain head can move. Two lines, same switch as I2. | In `redact.go`'s `case events.MoveZone, events.Draw, events.PutOnStack:`, add `e.IDs = filterVisible(g, e.IDs, viewer)` and `e.Pairs = filterVisiblePairs(g, e.Pairs, viewer)`. |
| 1910-1918 | **T21-i** code cites wrong ruling numbers | **YES — assigned to this wave** | The ledger's own ruling: "this codebase's convention is that comments are the durable record of WHY, so a wrong cross-reference is a real defect. Fix in the final wave; do not let a fixer guess the mapping." | The 11-row table in M6 above. Comment-only. |
| Task 29 re-review (added after the digest) | `rules/trigger.go:968` "Review finding I-3" carries M-6's content | **YES** | Same class as T21-i, same wave, one word. | Relabel `:968` to M-6; real I-3 stays at `:989`. |
| 3690-3692 | `view/view_test.go:444-446` orphaned doc | **YES** | Named for the final wave; the comment states the opposite of what the test below it proves. | Delete the three lines. |
| 3846-3849 | `rules/turn.go:253-255` I-4 comment misquotes `drainerSrc` | **YES** | Named for the final wave; a wrong worked example in the one comment that tells a future reader not to delete a live guard. | Replace "whenever you draw a card, lose life equal to your life total" with "an upkeep self-drain (`Mode$ Phase | Phase$ Upkeep`)". |
| 3218-3219 (N3) | `cards/parse_test.go:123` 130-char comment | **YES** | Named for the final wave ("sweep in the final wave"); measured 130 chars. | Reflow to ≤ 100. |
| 671 | `rules/fix1_test.go` meaningless name | **YES** | Cheap, named, and the file is 463 lines of real regressions nobody can find by name. | `git mv` to `genesis_replay_test.go` (package-internal; no import changes). |
| — (this review) | M1-M5, M13 stale comments (`trigger.go:817`, `statics.go:167`, `layers.go:80`, `statics_test.go:183`, `fix1_test.go:396`, `legal.go:66`) | **YES** | Every one asserts a thing is unimplemented or uncalled that this branch implements and calls. Same class as T21-i, and the wave is already touching four of these files. | Rewrite each to state today's truth; keep the still-valid reason where there is one (see M1). |
| 2627 / 3139 | **N9** attempted-set predicate-flip sub-case | **No — defer** | Independently re-measured at HEAD: **0** outstanding CR 704.5a and **0** outstanding CR 704.5f/g across 23 978 repo-deck decision points, 0 decisions to an eliminated seat. The ledger's own figure is 0.18% on a hand-built board, self-healing on the next `checkStateBased`, never touching the life invariant. The four pins are the gate and touching the loop a fifth time against a 0/23 978 signal is the worse trade. | — |
| 1340-1352 | **T19c-b** Equip/Attach, `TriggeredCard$`, non-mana activated-ability enumeration | **No — defer** | Confirmed: `rules/legal.go:68` offers mana abilities only, so Equip and every non-mana activated ability is inert. All four affected cards are in `knownUnsupported` (`kw:Equip` ×3, plus Sword of Fire and Ice) and are therefore already declared unplayable by the ratchet. M2/M4 coverage work. Fix only M13's stale framing. | — |
| 1882-1888 | **T21-h** multi-block last-blocker-absorbs | **No — defer** | A documented CR 510.1a approximation; the ledger's own cost note is "legal-but-suboptimal play, never an illegal state". Turning it into a decision is combat-UI scope. | — |
| 3215-3217 (N2) | `TargetMin$ 0` vs the `spec != ""` fizzle gate | **No — defer** | Real but bounded: 1 339 corpus cards carry `TargetMin$ 0`, and `askTarget` (stack.go:105) ignores `TargetMin`/`TargetMax` entirely — it is always Min 1/Max 1. Closing the fizzle gate alone would be incoherent without real `TargetMin$`/`TargetMax$` support, which is a milestone, not a wave. Exactly one such card (Karn, the Great Creator) is in a repo deck, and its abilities are never enumerated anyway (T19c-b). | — |
| 3215-3217 (N4) | optional trigger whose decider departed is silently declined | **No — defer** | `trigger.go` `putTriggersOnStack` declines rather than assuming yes, which still satisfies R2 (no outcome is assumed for a player who could answer). Belongs with the CR 800.4a family. | — |
| 3146 | **CR 800.4a general case** (a departed player's stack objects still resolve) | **No — defer** | Explicitly M2 scope in the ledger. The reachable half — a departed player's *pending triggers* and *pending decision* — is already closed (`dropDepartedTriggers`, `pushTrigger`'s own guard, `releasePendingDecisionOfDepartedPlayer`), and my 23 978-point walk found 0 decisions asked of an eliminated seat. | — |
| 3147 | Task 27's `ValidTgts$` fizzle consequence (**U13**, Sword of Fire and Ice's draw rider) | **No — defer** | The card is in `knownUnsupported` for `kw:Equip`, so it is already declared unplayable and its rider is unreachable. | — |
| Task 29 report (I-2) | ability object stuck on the stack under an exile replacement | **Folded into C1 — YES** | Reproduced end-to-end from card data (300 000 intents, never ends). It is the same defect as C1 at `stack.go:250`/`:212`, so it is fixed by the same helper. Do **not** leave it "noted only". | See C1. |
| Task 29 report (I-3) | ETB-trigger matching sees pre-replacement state | **No — defer** | Genuinely narrow: 8 corpus cards carry a tapped/untapped `ChangesZone` predicate, none in a repo deck. Fixing it means restructuring how replacement and trigger evaluation thread state — a milestone, not a wave. The comment at `trigger.go:989` states it accurately; only its neighbour's label (M7) is wrong. | — |
| 4114-4116 | Task 24 M-1 replay-runs-short plain error; M-4 param name | **No — defer** (M11 above) | Cosmetic diagnosis quality; `mtgsim` still reports failure and exits non-zero. Do it if the wave has room. | — |
| 3850-3852 | Task 28 M-1/M-2/M-3 (raw `PlayerLost` forge, `SetZone` truncation + budget 10, missing 4-seat `t.Logf`) | **No — defer** | Test-hygiene only; none affects a gate or a measured number. | — |
| 130-1525 (Tasks 2-20 "minor (deferred)" block, ~30 items) | int32 overflow clamps, dead guards, fixture duplication, `primitives_test.go` size, `copyTargets` shape, unused `p` param, etc. | **No — defer** | Each is documented with a stated cost; none is reachable from the corpus and none moves a measured number. Reviewed as a set, not individually re-litigated. | — |
| 460-465 | T11-f genesis per-player loop | **No — defer** | Superseded in practice: `Engine.New` now breaks on excess decks (T22-m) and my probe drove 17 malformed configs including that one with 0 panics. | — |
| 1717 | T20-g `state.Step` has no `Valid()` | **Already done** | `state/ids.go:58` has `Step.Valid()` and `String()` returns "unknown" out of range; my tampered-log fuzz drove Step 0-255 with 0 panics. Nothing to do. | — |

---

### Recommendations for after merge

1. **Add a stack-progress assertion to the fuzz gate.** The whole C1 family is
   "an object is on the stack and nothing removed it". One invariant —
   *`resolveTop` never returns with `id` still in `e.G.Stack`* — would have
   caught all four instances, and is cheaper than four guards' worth of tests.
2. **Make the coverage gate aware of replacement *shape*, not just
   `repl:Moved`.** `Supported()` currently blesses Rest in Peace and Dryad
   Militant. A `Destination$ Graveyard|Exile` replacement with no
   `ReplacementResult$` is a distinct primitive from an `Updated` one; splitting
   the registration key (`repl:Moved:Updated` vs `repl:Moved:Replaced`) would
   route the dangerous shape out of decks until CR 616 is properly modelled.
3. **Split `rules/trigger.go`** (1062 lines) along the seam Task 20 already
   identified at progress.md:1523: queue/drain, `*Matches` predicates,
   replacements.
4. **CR 616 multi-replacement.** I measured that exactly one of two matching
   `Updated` replacements applies, deterministically, by battlefield position.
   That is a documented M1 simplification, but it is the next thing a real
   Legacy board will notice, and the "which one" is currently an accident of zone
   order rather than a player's choice.
5. **Keep the four acceptance chain heads as a merge gate**, and add the two new
   stall regressions to it — they are the cheapest possible detector for anyone
   widening `applyReplacements` again.
