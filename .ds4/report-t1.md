# Report — game-long damage-by-source provenance (The Fallen, Diseased Vermin)

Ticket: `agent-20260923T114033Z-a57ee463`
Branch: `wt/agent-20260923T114033Z-a57ee463`
HEAD: `102a93451588e3b38c228864349ceb304e32aaad`
Base: `29ca6faa` (main at dispatch) — ancestry `87901df9` (merge base), then
  `c2f63152` (implementation) and `102a9345` (review-gate head re-pin).

## Round status

**The `findings-t1.md` note said round t1 was lost before it reported.** I
checked the worktree: `git status` was already **clean**, no uncommitted work
from the lost seat existed, and the implementation was already complete and
committed as `c2f63152`. Review round r1 then ran and returned **VERDICT:
APPROVE** (`.ds4/verdict-r1.md`), committing the head re-pin `102a9345`.

So my round did what the findings note asked: kept what is sound (everything —
it was committed) and verified/finished the brief. I re-ran every gate the
brief names and independently re-proved the bite of the new tests with targeted
revert probes (below). No code changes were needed; the tree at `102a9345` is
the finished brief.

## What the implementation is (per file, as committed in `c2f63152`)

- **`events/event.go`** — appended `DamageProvenance` after `CloneStatic`;
  `NumKinds = int(DamageProvenance) + 1`; `"damage_provenance"` added to
  `kindNames` (array length is `NumKinds`, so it is covered by construction).
- **`events/apply.go`** — new `case DamageProvenance`: appends the source to
  the recipient's game-long record (dedup, never cleared). `Obj` = damage
  source, `IDs[0]` = recipient. Encoding: a plain `ObjID` for an object
  recipient, `state.PlayerRef`-encoded for a seat (keeps seat 0 distinct from
  "no recipient"; `PlayerRef` bit-31 encoding cannot collide with a real object
  id). Chosen over a `Counter` discriminator because `PlayerRef` is the
  established `[]ObjID` player-reference precedent (TriggerPush).
- **`state/game.go` / `state/object.go`** — `Player.DamageTakenByGame []ObjID`
  and `Object.DamageTakenByGame []ObjID`; deep-copied in `Game.Clone` /
  `Object.CloneDeep`.
- **`rules/engine.go`** — one emission choke point in `emit`'s post-fold tail
  beside the infect/wither conversions: `stored.Kind == Damage && stored.Amount
  > 0`, source from `e.inFlightDamageSource()`, recipient from the APPLIED
  `stored` event. `Amount > 0` excludes cleanup negatives; a zero source emits
  nothing (no false `(0, recipient)` fact). **No emitter file changed.**
- **`effects/filter.go`** — both object spellings classified
  (`wasDealtDamageByThisGame`, argument-taking `wasDealtDamageThisGameBy <ref>`)
  and matched; the player base-qualifier `wasDealtDamageThisGameBy <ref>` and
  the bare compound clause both read the record through the single shared
  helper `playerDamageByRefThisGame` -> `damageGameRecordHas`;
  `contextPredicateBound` refuses the unbound source positively and under `!`.
- **`effects/damage.go`** — the ValidPlayers fall-through walk binds the sweep's
  source (`MatchesPlayerSpecFrom(..., c.Source)`); `validPlayersSelectorUnknown`
  consults the shared bare-clause list, so the census gate and the matcher agree
  with no parallel vocabulary.

## Gates — real output (this round)

```
=== effects gate ===
go test -run 'TestDamageAllValidPlayers|TestContextWordPredicates|TestUnimplementedPredicateFailsClosed' ./effects/
ok  	github.com/adams-shaun/gorge/effects	(cached)

=== rules census ===
go test -run 'TestValidTgtsPurePlayerCensusPinsThePlayerQualifierSets' ./rules/
ok  	github.com/adams-shaun/gorge/rules	(cached)

=== rules provenance ===
go test -run 'TestEmitRecordsGameLongDamageProvenance|TestEmitRecordsObjectDamageProvenance|TestDiseasedVerminAskOffersOnlyPreviouslyDamagedOpponents' ./rules/
ok  	github.com/adams-shaun/gorge/rules	0.452s

=== effects provenance ===
go test -run 'TestDamageAllValidPlayersTheFallenResolves|TestPlayerDamageByRefThisGameFailsClosedOnUnboundRef|TestDamageProvenanceWordsAreClassified' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.422s

=== repro fixture gate ===
go test -run TestGenerateCommittedFixture ./cmd/repro/
ok  	github.com/adams-shaun/gorge/cmd/repro	(cached)
# non-regen gate: SKIP (no REPRO_REGEN_FIXTURE)

=== archtest ===
go test ./internal/archtest/
ok  	github.com/adams-shaun/gorge/internal/archtest	3.287s

=== botbench ===
go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	(cached)

=== fixture replay (full package, once) ===
go test ./cmd/repro/
ok  	github.com/adams-shaun/gorge/cmd/repro	14.490s
```

Formatting / types (the Go half of `make lint`):

```
gofmt -l <changed .go files>   -> (no output)
go run ./cmd/gentypes -check    -> (no output)
```

`git status --short` at HEAD -> (empty; tree clean).

## Fails without the fix (independent revert probes this round)

Method: copy the target file to `.ds4/scratch/<name>.orig`, revert ONLY the
relevant hunk with a Python string replace in the real file, `go build ./...`,
run the one test, then restore with `cp` and `cmp` against the scratch copy
(never `git stash` / `git checkout <path>`). Every restore passed `cmp` and
left `git status` clean.

### Probe 1 — remove the `rules/engine.go` emission block

```
--- FAIL: TestEmitRecordsGameLongDamageProvenance (0.00s)
    damage_provenance_test.go:42: want exactly one DamageProvenance event for one landed hit, got 0 ([])
FAIL	github.com/adams-shaun/gorge/rules	0.003s
```
Restored: `cmp rules/engine.go .ds4/scratch/engine.go.orig` OK.

### Probe 2 — disable the bare-word classification in `effects/filter.go`

```
--- FAIL: TestDamageProvenanceWordsAreClassified (0.00s)
    damage_provenance_test.go:129: spec "Creature.wasDealtDamageByThisGame" reports unknown predicates [wasDealtDamageByThisGame], want none
FAIL	github.com/adams-shaun/gorge/effects	0.002s
```
Restored: `cmp effects/filter.go .ds4/scratch/filter.go.orig` OK.

### Probe 3 — make the shared player body `playerDamageByRefThisGame` return false

```
--- FAIL: TestDamageAllValidPlayersTheFallenResolves (0.43s)
    damage_provenance_test.go:71: the damaged opponent must take The Fallen's 1 damage, life 20 want 19
--- FAIL: TestPlayerDamageByRefThisGameFailsClosedOnUnboundRef (0.00s)
    damage_provenance_test.go:103: with the source bound, the damaged opponent must qualify
FAIL	github.com/adams-shaun/gorge/effects	0.449s
```
Restored: `cmp effects/filter.go .ds4/scratch/filter.go.orig` OK.

Each new test asserts its own precondition (The Fallen's script still carries
the compound; the walker is a planeswalker with loyalty > 0; seat 1's record is
non-empty after seeding; the compared life values differ), so a vacuous setup
fails loudly rather than passing silently.

## Goldens / heads (the brief's expected movement)

- **`TestHeads` — all four moved, measured, and now green** after the re-pin
  the review gate committed in `102a9345`:
  `2 f107be40dc2792c6`, `4 9b3aab4e0336ba8c`, `6 c4ce39421c473963`,
  `8 3d1974a1859d9676`. Cause is mechanical: every repo-deck game now carries
  one extra `DamageProvenance` event per landed point of damage. Attribution
  proven by reverting only the emission block (restores all four old golden
  heads exactly) and corroborated by `TestConstructedDefaultIsByteIdentical`
  staying green (no decision moved). The brief said not to edit
  `heads_test.go`; the review gate made the bookkeeping re-pin under its
  2026-09-23 ruling. `go test -run 'TestHeads$' ./rules/` -> ok (1.658s).
- **Committed feedback fixture `20260915T094418Z-e484f1db` — regenerated**
  (sanctioned). Verified this round: its `log.json` carries exactly **23**
  `kind: 93` events = `DamageProvenance` (the log serializes kind as an
  ordinal, not a name). `go test ./cmd/repro/` -> ok (14.490s, byte-identical
  replay).
- **Committed feedback fixture `20260914T120000Z-fb01` — NOT regenerated.**
  The brief's premise that both would DIVERGE is false: that fixture's captured
  window deals no damage (0 provenance events needed) and it replays
  byte-identically. Regenerating it would instead pick up unrelated generator
  drift (`bot_policy: "bot"`, 24 -> 26 intents); the review gate independently
  reproduced this. Correct call: kept the committed original.
- **`TestConstructedDefaultIsByteIdentical` (cmd/botbench) — GREEN**, no re-pin
  needed: the repo decks carry neither card and provenance emission changes no
  decision.

## Brief premises checked (counts are claims)

- Corpus prevalence: 2 game-long spellings, 2 cards
  (`diseased_vermin`, `the_fallen`), both arguments literally `Self` — **held**.
- The brief's "FALSE premise" correction (Diseased Vermin carries only the
  ThisGame line, not the ThisTurn sibling) — **held**.
- "Both committed fixtures will DIVERGE and need regeneration" — **FALSE for
  fb01**; only e484f1db needed it. Reported, not silently followed.
- "Expect NO Known-approximations row edit" — **held**; no row added, grown or
  deleted, `knownApproximationRows` untouched (still 9 at this commit).

## Issues (found, not fixed)

1. **Per-turn by-source siblings remain unknown-word fail-closed** —
   `wasDealtDamageThisTurnBySource` and `wasDealtCombatDamageThisTurnBySource`.
   Out of scope per the brief (only the two ThisGame spellings). They need the
   per-turn twin (cleared at TurnChange) plus LKI, not the game-long record.
   Measured carriers (4 files): `hidetsugu_consumes_all_vessel_of_the_all_consuming`,
   `hope_of_ghirapur`, `raphael_tag_team_tough`, `wicked_akuba`. CR 120.3.
   Natural follow-up ticket.

2. **`cmd/repro/testdata/feedback/20260914T120000Z-fb01` is stale relative to
   its generator.** `REPRO_REGEN_FIXTURE=1 go test ./cmd/repro -run
   TestGenerateCommittedFixture` no longer reproduces it (`bot_policy: "bot"`,
   26 intents vs the committed 24, new head, view churn) — independent of this
   ticket (the review gate reproduced it with the feature reverted). Invisible
   today because the non-regen gate SKIPs and the committed fixture still
   replays. A future legitimate corpus-pin regen will churn the hard-coded
   expectations in `cmd/repro/repro_test.go` (24 intents, head `6fd99d48...`).

3. **`DamageTakenByGame` is unbounded in a long game** — one `ObjID` per
   distinct source per recipient, never cleared (game-long by design). Bounded
   by distinct damage sources in a match; a future snapshot/compact path should
   be aware.

4. **`host/session_test.go` ring headroom** — the test's channel capacity was
   raised 64 -> 256 to absorb the denser frame stream (extra provenance events);
   the review gate verified the production default is already 256 and every
   assertion is unchanged. If a real damage-heavy deployment is tight, that is
   a capacity-tuning question outside this ticket.

## Deviations from the brief

- **No regeneration of fb01** (brief premise false; see above) — inherited from
  `c2f63152`, re-verified this round.
- **`rules/heads_test.go` was edited (re-pinned)** by the review gate in
  `102a9345`, not by the implementer; the brief said not to, but the movement
  was expected and the re-pin carries measured attribution.
- **`host/session_test.go` and `host/overshoot_tail_test.go`** were touched
  (un-named by the brief) because the always-emit design the brief mandated
  moved them; both attributed.

No Known-approximations row added or grown.
