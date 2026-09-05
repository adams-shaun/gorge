# M0/M1 rulings digest

Every orchestrator ruling recorded in the M0/M1 SDD ledger (`progress.md`, 4552
lines, archived outside the repo), one bullet each: decision, why, cost if
wrong as the ledger stated it ("not stated" where it did not), and status. Two
IDs the code or ledger cites were never defined as rulings: `T14-c` (cited once
in the ledger) and `T22-c` (cited by `rules/engine.go`'s small-deck genesis
guard; that code comment is its only record). Compiled 2026-09-04 by a
transcription pass over the ledger; wording is the ledger's, condensed.

**Total distinct rulings: 194** (193 explicitly-IDed rulings across the checked families, plus one entry below flagged for an ID collision in the source — see T14 section).

**Missing IDs from the requested families (verified absent from the text):**
- `T14-c` — cited once (line 947, "Ruling T14-c's nil-`Face()` requirement") but never independently defined anywhere in the ledger with its own decision text.
- `T22-c` — absent. Task 22's lettered rulings jump directly from T22-b to T22-d; no T22-c exists anywhere in the file.

All other requested families (F1–F8; Task 1/3/4×2/5/6; T8-a..b; T9-a..d; T10-a..e; T11-a..g; T12-a..d; T13-a..b; T14-a,b,d,e,f; T15-a..d; T16-a..b; T17-a..d; T18-a..c; T19-a..d; T19b-a..c; T19c-a..b; T20-a..g; U-place, U1..U13; W1, W2; T21-a..i; D2-a; T22-a,b,d..q; T23-a..z; P1..P14; T24-a..c; T25-a..g; T26-a..c; T28-a..c; T29-a..c; M2-a..l; F-1..F-8) were located in full.

---

## Pre-flight (F1–F8)

- **F1** — Execute Tasks 15, 16, 17 before Task 14 — *why:* Task 14's tests call `effects.Resolve`/`MatchesSpec`, which those tasks produce — *cost if wrong:* "Costs nothing if wrong; the dependency is mechanical" — *status:* implemented
- **F2** — `view.Chars` interface renames the `Derived` accessor to `Keywords`: `{Power; Toughness; Keywords}` — *why:* `Engine.Derived` already returns a `Derived` struct; renaming the interface method is the smaller change — *cost if wrong:* a mechanical rename in one interface and one method — *status:* implemented
- **F3** — Any code reading `Object.Face()` must handle nil (resolveTop, view.cardViews fall back) — *why:* triggered abilities (T20) and tokens (T18) have no printed card — *cost if wrong:* nil dereference at the first trigger; T25's fuzz would catch it anyway — *status:* implemented
- **F4** — `setStep(StepCleanup)` removes marked damage and clears Deathtouched counters via negative Damage/CounterChange events — *why:* CR 514.2, damage wears off during cleanup — *cost if wrong:* not stated — *status:* parked→T21 (implemented there)
- **F5** — `setStep(StepCleanup)` also calls `EndOfTurnCleanup()` — *why:* Task 19 defines it but nothing calls it; UEOT effects would be permanent — *cost if wrong:* not stated — *status:* parked→T21 (implemented there)
- **F6** — `effects.parseCMC` and `rules.ParseCost` may both compute CMC, duplicated rather than shared — *why:* `effects` cannot import `rules` without a cycle — *cost if wrong:* extract a shared manacost leaf package later; no caller changes — *status:* accepted-as-is
- **F7** — `rules.newTestBot` may duplicate `seat.Bot`'s policy — *why:* the rules fuzz test cannot import `seat` without a cycle — *cost if wrong:* move the policy into a leaf package; mechanical — *status:* accepted-as-is
- **F8** — `stubs.go` must not survive Task 22 — *why:* Tasks 11/13 add placeholders whose lifetimes the plan tracks — *cost if wrong:* "No cost." — *status:* implemented (stubs.go deleted in Task 22)

## Task 1 / 3 / 4 (two) / 5 / 6

- **T1-a** — The boundary guard must also inspect staged and working-tree content, not only `HEAD:path` — *why:* a `git add`ed-but-uncommitted Forge script passed the test; plan mandated that shape — *cost if wrong:* test merely stricter; a few lines, no behaviour change — *status:* implemented
- **T3-a** — Replace the plan's goroutine/channel cycle test with a plain synchronous call asserting Link() returns and reports a diagnostic — *why:* the plan's version cannot fail the way it claims; `go test`'s own timeout is the real hang detector — *cost if wrong:* a cyclic SVar hangs the suite until timeout, same as the plan's version — *status:* implemented
- **T4-a** — LoadRegistry must check the error from `gzip.Reader.Close()` — *why:* truncated caches loaded with nil error via a bare `defer zr.Close()` — *cost if wrong:* one-line revert; no behaviour change for intact caches — *status:* superseded by T4-b
- **T4-b** — (supersedes T4-a) LoadRegistry must DRAIN the gzip reader to EOF and check that error, not merely check Close() — *why:* Close() alone never forces the CRC/ISIZE trailer check; corruption at several sizes went undetected — *cost if wrong:* one extra full read of a 6.7MB cache at load, once per process — *status:* implemented
- **T5-a** — Fix both reviewer Importants now: length-prefix DigestDir's path/content framing (closes a hash collision), and clean up Fetch's leftover work directory on failure — *why:* reviewer produced an actual digest collision; the leftover dir is cheap tidiness — *cost if wrong:* both changes inert for every successful fetch — *status:* implemented
- **T6-a** — Emit a diagnostic when a card ends up with NO named face, and only then (per-card, not per-face) — *why:* exactly 2 corpus files produce nameless cards; per-face would blow the 20-diagnostic budget — *cost if wrong:* two diagnostics of noise, budget still half free — *status:* implemented

## Task 8

- **T8-a** — `Option.Player` must lose `json:",omitempty"` (`Obj` keeps it) — *why:* `state.PlayerID` is 0-indexed; omitempty made a player-0 option wire-identical to no-player — *cost if wrong:* a slightly larger JSON envelope — *status:* implemented
- **T8-b** — `Chosen` must not panic on an out-of-range index; any index outside `[0,len(Options))` returns nil — *why:* Intent is network-facing under D6's one-goroutine-per-match model — *cost if wrong:* a caller that skipped Validate gets nil instead of a crash, acceptable — *status:* implemented

## Task 9

- **T9-a** — (blocker) `Log.Append` must copy IDs and Pairs before storing — *why:* a caller mutating its own slice after Append retroactively changed the logged event — *cost if wrong:* two small allocations per event carrying a slice — *status:* implemented
- **T9-b** — (major) `NoHash` becomes immutable after the first Append — *why:* toggling mid-stream desyncs incremental Head() from HeadAt(len) with no signal — *cost if wrong:* a benchmark wanting to toggle must build two logs — *status:* implemented
- **T9-c** — (minor, brief-mandated) The Seed IS folded into the hash chain — *why:* seed is the other half of the (seed, intents, events) triple D4 ties together — *cost if wrong:* chain heads change value; nothing yet consumes them, so now is cheapest — *status:* implemented
- **T9-d** — Fix-1 brief's claim "the encoding distinguishes nil from empty" is FALSE — Append writes only a length prefix, so nil/empty already hash identically; kept as-is and documented — *why:* reviewer proved the claim false; the copy idiom's normalisation is harmless — *cost if wrong:* a future consumer distinguishing them must add a discriminator, changing every chain head — *status:* accepted-as-is

## Task 10

- **T10-a** — (blocker) Move derives the removal zone from `o.Zone`, NOT from the event's `From` — *why:* trusting caller-supplied From let one event put an object in two zones or duplicate it — *cost if wrong:* an engine bug emitting a wrong From is silently absorbed rather than corrupting state — *status:* implemented
- **T10-b** — (blocker) Every `g.Obj()` dereference nil-guarded and every `g.Players[e.Player]` index bounds-checked; a failing guard makes the event a no-op — *why:* Tap/Untap/CounterChange/DeclareAttackers/DeclareBlockers/LifeChange/ManaAdd/ManaClear/PlayerLost/TurnChange lacked this; one goroutine per match — *cost if wrong:* not stated — *status:* implemented
- **T10-c** — (major) Zone values range-checked before reaching state's zone-index arithmetic — *why:* an out-of-range Zone panicked the same way as T10-b's cases — *cost if wrong:* not stated — *status:* implemented
- **T10-d** — (minor) Damage clamps at zero, matching AddCounter's existing clamp — *why:* not stated beyond matching the existing convention — *cost if wrong:* a "negative damage as healing" idiom would need its own event kind, the clearer design anyway — *status:* implemented
- **T10-e** — Partial application is per-item for list-shaped kinds (IDs/Pairs); scalar kinds stay all-or-nothing — *why:* self-consistent per reviewer's check — *cost if wrong:* a malformed DeclareBlockers applies its valid pairs instead of dropping whole, which combat wants anyway — *status:* accepted-as-is

## Task 11

- **T11-a** — (blocker) Game.Passes/Game.Priority stop being written directly; the Priority event carries the holder in Player, pass count in Amount; events.Apply sets both — *why:* live vs. reconstructed Passes diverged (2 vs 0), breaking playback-to/resume-from — *cost if wrong:* one extra event field's worth of log size per priority pass — *status:* implemented
- **T11-b** — (major) `priorityRound` returns immediately if G.Over, and Submit rejects any intent once G.Over — *why:* mid-round elimination let Pending() serve a decision after match end — *cost if wrong:* "nothing — a finished game refusing input is the only defensible behaviour" — *status:* implemented
- **T11-c** — (minor) The draw-step trigger checks the active player is not Lost — *why:* becomes reachable the moment SBAs land in Task 21 — *cost if wrong:* not stated — *status:* implemented
- **T11-d** — (info, accepted) Genesis legitimately bypasses events; must be documented, since the log alone is not a complete match description — *why:* Task 24 (replay) depends on knowing this — *cost if wrong:* Task 24 would rediscover it the expensive way — *status:* accepted-as-is
- **T11-e** — (deferred) stubs.go's zero-survivors fallback (seat 0 wins unconditionally) dies with stubs.go's deletion in Task 22 rather than being patched — *why:* unreachable while eliminations are strictly sequential — *cost if wrong:* not stated — *status:* parked→T22 (F8)
- **T11-f** — (minor, PARKED) Engine.New's per-player genesis loop does not check e.G.Over between seats — *why:* Task 21 rewrites elimination; fixing now targets soon-to-be-dead semantics — *cost if wrong:* a malformed-deck match logs a few unreachable events — *status:* parked→T21 (later resolved in Task 22)
- **T11-g** — (info, ACCEPTED AS IS) Priority.Amount clamped from below but not above — *why:* Passes only ever compared against AliveCount; a large value just ends the round early, replay still exact — *cost if wrong:* not stated — *status:* accepted-as-is

## Task 12

- **T12-a** — Do NOT build hybrid alternative-payment modelling in Task 12; document the generic-fold approximation and pin it with a test covering `GW`/`2B`/`W/U` — *why:* the brief's spec contradicted itself; zero of 148 M1-deck cards use hybrid spelling — *cost if wrong:* M4 builds the alternative-payment path anyway; nothing in M0/M1 changes — *status:* implemented
- **T12-b** — Phyrexian mana accepted as the same generic approximation for M1, documented and pinned by test — *why:* `Cost.Pay(pool)` cannot express "or pay 2 life" without touching Tasks 13/19b's surface for 2 cards — *cost if wrong:* Gitaxian Probe/Dismember misprice; no crash, no replay divergence — *status:* accepted-as-is
- **T12-c** — Fix reviewer's Minor 1: reject negative Generic and int32 overflow in ParseCost into the existing "+1 generic" fallback — *why:* both reachable from corpus text — *cost if wrong:* "nothing; no real mana cost is negative or >= 2^31" — *status:* implemented
- **T12-d** — Fix reviewer's Minor 2 by documenting the X contract in Cost's doc comment (flag contributing 0 to CMC/Pay) — *why:* matches CR 202.3b off-stack — *cost if wrong:* "nothing; a comment" — *status:* implemented

## Task 13

- **T13-a** — `handlePriority` must NOT write `e.G.Passes`/`e.G.Priority` directly; route through `events.Priority` per T11-a — *why:* brief written before T11-a existed and re-breaks it — *cost if wrong:* "nothing; the emit path is already proven by Task 11's re-review" — *status:* implemented
- **T13-b** — `LandsPlayed++` gets the same treatment via a new appended event kind `LandPlayed` — *why:* every state mutation must go through events.Apply; reusing CounterChange or MoveZone both had worse failure modes — *cost if wrong:* one unused event kind, and a replay that already diverged — *status:* implemented

## Task 14

- **T14-a** — Three `e.G.Priority = e.G.Active` writes in Task 14's brief become `events.Priority` emits with Amount 0 — *why:* same class as T13-a; written before T11-a existed — *cost if wrong:* "none; proven path" — *status:* superseded by T14-e (wrong Player value)
- **T14-b** — `handleTarget`'s live-only `o.Targets = [...]` write gets a new `TargetsChosen` event, appended at the end of the Kind block, encoded via existing Obj/Amount/IDs/Player fields — *why:* events/apply.go clears Targets on zone change but nothing ever sets it; replay would resolve with no targets — *cost if wrong:* one extra event per targeted spell in the log — *status:* implemented
- **T14-b (2nd use — ID reused in source)** — CR 608.2b target rechecking at resolution reassigned from Task 21 to Task 22, since `resolveTop` lives in rules/stack.go, which Task 21 never touches — *why:* Task 22 owns legality/lose-conditions; grafting into Task 21 edits outside its scope — *cost if wrong:* one small feature lands one task later — *status:* superseded (implemented in Task 22 as item 4)
- **T14-d** — `mtgcore/effects/primitives.go` (46 lines, registers only DealDamage and Mana) ratified as an out-of-file-list addition — *why:* Task 14's brief-mandated tests needed a working DealDamage primitive no task had shipped yet — *cost if wrong:* Task 18 rewrites 46 lines it was going to write anyway — *status:* implemented (superseded by Task 18)
- **T14-e** — (corrects T14-a, "this defect is mine") The three Priority emit sites must name the ACTOR, not `e.G.Active` — *why:* CR 117.3c: the caster of an instant keeps priority; T14-a copied the brief's wrong value — *cost if wrong:* priority wrongly snaps to the active player instead of the caster — *status:* implemented
- **T14-f** — Clamp `dealDamage`'s NumDmg at zero, and `addMana`'s Amount the same way — *why:* `NumDmg$ -5` currently HEALS via Apply's Damage case — *cost if wrong:* one comparison in a file Task 18 rewrites anyway — *status:* implemented

## Task 15

- **T15-a** — `Defined()` must return a defensive copy, not `c.Targets` itself — *why:* mutating a returned slice element mutated `Ctx.Targets` since Ctx is threaded by pointer — *cost if wrong:* one slice copy per effect resolution, "which is nothing" — *status:* implemented
- **T15-b** — Close the registry data race with an atomic copy-on-write snapshot (mutex on writes, `atomic.Pointer` load on reads), not a mutex on the read path — *why:* `go test -race` fired immediately; Register's own doc already claims post-init re-registration is supported — *cost if wrong:* ~15 lines and one atomic load per lookup — *status:* implemented
- **T15-c** — Bundled minors: document Host.Game()'s "never write directly" invariant; add the missing Defined$Player test case — *why:* free to fix in the same round, not loop-extending — *cost if wrong:* not stated — *status:* implemented
- **T15-d** — (process) Dispatch this fix round to a FRESH implementer on sonnet rather than resuming the haiku implementer — *why:* copy-on-write is subtle shared-state work; model policy puts that tier on sonnet — *cost if wrong:* one extra context rebuild — *status:* process

## Task 16

- **T16-a** — Reviewer's first Important (`nonBogusType` matches everything; UnknownPredicates can't see base tokens) is NOT a defect — parked, not fixed — *why:* Forge base types are an open vocabulary (subtypes used as base types); no closed set exists to validate against — *cost if wrong:* a typo'd negated base type over-matches until coverage catches it — *status:* parked→T26
- **T16-b** — Reviewer's second Important IS real: `matchesBase`'s "Permanent" case must check `o.Zone == state.ZBattlefield`, not match unconditionally — *why:* a card in hand/library/graveyard/exile was being treated as a permanent — *cost if wrong:* "nothing; this is what the rules say" — *status:* implemented

## Task 17

- **T17-a** — Guard totality holes: nil `*Ctx` substituted with an empty Ctx; nil Host returns the caller's default — *why:* Tasks 18-21 are the callers; a reachable panic is a remote kill of the match goroutine — *cost if wrong:* two cheap guards on a hot-ish path — *status:* implemented
- **T17-b** — Bounds-check `Ctx.Controller` before `g.Players[c.Controller]` in YourLifeTotal — *why:* panics at the boundary value `len(Players)`, an off-by-one — *cost if wrong:* one comparison — *status:* implemented
- **T17-c** — `SetSVars` must copy the map it is handed, not alias it — *why:* Task 15 already established defensive-copy convention (T15-a); Ctx is threaded by pointer — *cost if wrong:* one map copy per resolution — *status:* implemented
- **T17-d** — Bundled minors: add tests for an actual self-referencing/cyclic SVar and for SetSVars, both brief-mandated but uncovered — *why:* free in the same round — *cost if wrong:* not stated — *status:* implemented

## Task 18

- **T18-a** — Bounds-check 7 of 9 cardflow.go APIs via a shared guarded accessor `zoneOf(g,z,p)` — *why:* a target Player of 250 panicked; violates the brief's own totality contract — *cost if wrong:* seven bounds checks on a path that cannot currently reach them — *status:* implemented
- **T18-b** — `Token` and `CopySpellAbility` stay UNREGISTERED; gap documented and pinned by a regression test — *why:* no event kind can mint new game objects; a Note-only stub would falsely mark such cards playable — *cost if wrong:* cards needing tokens stay correctly reported unplayable until a later milestone — *status:* accepted-as-is
- **T18-c** — Four pieces of extra infrastructure ratified as necessary, not scope creep: `events.FlipFace`, `cards.ResolveSVar`, the SVar threading fix, wiring `effects.Supported()` into forgec report — *why:* each is required by the brief's own table/Step 4 verification — *cost if wrong:* not stated — *status:* implemented

## Task 19

- **T19-a** — Route the continuous-effect timestamp through a logged event (Clock++ inside events.Apply, AddContinuous emits it) — *why:* the bare `e.G.Clock++` outside Apply is the same bug class T11-a fixed for Passes/Priority — *cost if wrong:* one event per continuous effect registered — *status:* implemented
- **T19-b** — `Engine.HasKeyword` must be case-insensitive (`strings.EqualFold`) — *why:* every current call site is case-insensitive already; silent trap for corpus-cased input — *cost if wrong:* "one EqualFold" — *status:* implemented
- **T19-c** — `rules/legal.go`'s `f.HasKeyword("Flash")` must become `e.HasKeyword(id, "Flash")` — *why:* brief's Step 3 said replace every direct printed-characteristic read; legal.go was missed only by omission — *cost if wrong:* "none; it is what the accessor exists for" — *status:* implemented
- **T19-d** — (NEW TASK 19c) Move ContinuousEffect down into `state`, extend `effects.Host`, replace 5 Note-only stubs (Pump/PumpAll/Animate/Protection/Effect) — *why:* `effects` cannot import `rules`; 13 M1-deck cards including Dismember silently do nothing without this — *cost if wrong:* one type moves package, one interface method added — *status:* implemented (as Task 19c)

## Task 19b

- **T19b-a** — Ratify implementer's out-of-scope fix: `castSpell` pays `adjustedCost`, not the printed cost — *why:* legalActions gated on adjustedCost while castSpell paid the printed cost, letting a ReduceCost static resolve a spell paid for wrong — *cost if wrong:* "none; verified" — *status:* implemented
- **T19b-b** — (CRITICAL) Alternative-cost casting must pay the alternative cost; failed payment must abort the cast — `Option` gains a cost-variant field, `payMana` must report failure — *why:* a spell could be cast for ZERO MANA when the base cost was unaffordable and the alt cost was the whole point — *cost if wrong:* "one field on an internal option struct" — *status:* implemented
- **T19b-c** — (Important) `parseAmount` must bounds-check like `ParseCost` already does — *why:* `RaiseCost | Amount$ 3000000000` wraps negative, making a spell castable free — *cost if wrong:* "one range check" — *status:* implemented

## Task 19c

- **T19c-a** — `Effect` stays an unwired Note-only stub — verified as a correct scope call, not a dropped job — *why:* its Triggers$/StaticAbilities$ params aren't representable in ContinuousEffect and need Task 20's trigger queue — *cost if wrong:* one primitive lands in Task 20 instead — *status:* accepted-as-is
- **T19c-b** — (PARKED, carried to Tasks 20/21) Three residual gaps: effPump/effProtection's battlefield-only zone guard blocks Snapcaster Mage; Sword of Fire and Ice needs a whole Equip/Attach mechanic; Eldrazi Mimic needs TriggeredCard$ — *why:* all pre-existing and unreachable in today's build — *cost if wrong:* not stated — *status:* parked→T20/T21 (split further: zone guard settled not-the-problem by T20 review; remainder parked to Task 26/follow-up)

## Task 20

- **T20-a** — (CRITICAL) `putTriggersOnStack` must create the ability's stack object through a new appended event kind `TriggerPush`, not a direct AddObject — *why:* folding the log alone diverged permanently at the trigger's Move; not genesis-recoverable — *cost if wrong:* one appended event kind — *status:* implemented
- **T20-b** — (Important) `resolveTop`'s ability branch must build `Ctx{Source: id}` from `o.Source`, not the transient stack object's id — *why:* `Defined$ Self` resolved to the wrong object, silently no-opping the effect (root cause behind Piledriver/Eldrazi Mimic beyond count.go) — *cost if wrong:* "one field" — *status:* implemented
- **T20-c** — (Important) `Ctx.Remembered` does not survive to resolution because `events.Move` resets non-battlefield destinations including ZStack — *why:* contradicts the brief's own interface text — *cost if wrong:* triggered abilities cannot refer to what triggered them — *status:* implemented
- **T20-d** — (Important) No shipped test verifies a triggered ability's effect landed on the CORRECT object, only stack length — *why:* 246 passing tests gave false confidence, missing T20-b/T20-c — *cost if wrong:* not stated — *status:* implemented
- **T20-e** — (fix round 2) `events.Apply`'s TriggerPush case never validates Player, unlike sibling cases; audit ALL 28 Apply cases in the same pass — *why:* `TriggerPush{Player:200}` panics on replay of a tampered log — *cost if wrong:* a bounded one-file audit; alternative is a remote-kill primitive in the headline feature — *status:* implemented (4 of 28 cases fixed)
- **T20-f** — NO third review round for Task 20 — *why:* the residual risk is exactly what the 253 unchanged tests measure; the bound was verified directly instead — *cost if wrong:* a dropped event class surfaces in Task 24's replay exercise — *status:* process
- **T20-g** — (parked, assigned to Task 24) `state.Step` needs a `Valid()` method; `Step.String()` can panic on a tampered StepChange outside events.Apply — *why:* T20-e's audit only covers events.Apply, not downstream String() calls — *cost if wrong:* one remaining remote-kill primitive on the untrusted-log path — *status:* parked→T24 (String() half pulled forward by T23-i)

## U-place, U1–U13

- **U-place** — Task 27 executes AFTER Task 22 and BEFORE Task 23 — *why:* Tasks 23/24/25 must each handle new decision kinds; sequencing after avoids reopening all three — *cost if wrong:* Task 27 dispatched against a trigger.go that 21/22 then edit; small merge — *status:* implemented
- **U1** — (ordering) Only the INTRA-controller order is the player's; APNAP order across controllers stays engine-determined — *why:* CR 603.3b gives each player choice only for triggers they control — *cost if wrong:* the wrong player is asked about somebody else's triggers — *status:* implemented
- **U2** — (protocol) NO change to the decision wire format; ordering is Min=Max=N, optional is Min=Max=1; only new `decision.Kind` constants added — *why:* Decision.Validate already enforces this shape natively — *cost if wrong:* a protocol change rippling into 23/24/25 and every client — *status:* implemented
- **U3** — (resumability) `Engine.ask` is ASYNC; putTriggersOnStack must become resumable across a Submit via `e.pendingTriggers` — *why:* `Advance` loops while pending is nil, so a synchronous ask is impossible — *cost if wrong:* the engine deadlocks or drops triggers — *status:* implemented
- **U4** — (determinism) The order reaches replay as an Intent plus a DecisionMade event, carried by the ORDER of `events.TriggerPush` events — *why:* keeps replay log-driven, preserving T20-a — *cost if wrong:* silent replay divergence, the exact defect T20-a was — *status:* implemented
- **U5** — (R3 placement) Requirement R3 (stack observability) is a VIEW concern, riding on Task 23's brief, not Task 27's — *why:* not stated beyond scope fit — *cost if wrong:* R3 gets built twice or not at all — *status:* implemented (delivered in Task 23)
- **U6** — (new, mine) Never ask a player with Lost==true to order anything; drop pending triggers controlled by a departed player (CR 800.4a) — *why:* a controller eliminated between ask and answer must not strand the engine — *cost if wrong:* not stated — *status:* implemented (later found violated by N2, fixed under U11)
- **U7** — Review Task 27 on its own branch (`task-27-triggers`), fast-forward main only after it passes — *why:* keeps main at a known-good commit while a five-round fix loop is possible — *cost if wrong:* one extra fast-forward — *status:* implemented (discharged, main fast-forwarded to 0ea0aea)
- **U8** — F1 is a BLOCKING regression, not a should-fix (overrules the reviewer's own downgrade) — *why:* reproduced a CR 117.5 stack-order violation and a permanent hang, impossible before Task 27 — *cost if wrong:* a wedged match in production with no log tail explaining it — *status:* implemented
- **U9** — F2 invalidates the implementer's reason for a narrow rescue hook; widen the release hook to ANY `d.Kind` — *why:* KTarget is also asked from inside `handle`, not only trigger decisions — *cost if wrong:* not stated — *status:* implemented (widened further to no Kind filter at all)
- **U10** — The corpus `Optional$` count is 1143 (SVar 884/A 225/R 19/S 15), counted directly rather than adjudicated between three differing agent counts — *why:* settling by direct grep rather than trusting any one report — *cost if wrong:* not stated — *status:* implemented (the "corrected" number was itself later found wrong once, fixed in fix round 2)
- **U11** — Fix N1 (a released KTarget resolves with untargeted riders running) and N2 (a departed-controller's trigger still gets pushed, violating U6) — *why:* both are rules bugs; N2 is a spec violation of U6, not a legacy wart — *cost if wrong:* not stated — *status:* implemented
- **U12** — Make the no-recursion property of `resumeTriggerDrain` STRUCTURAL (`if e.pending != nil { return }`), not merely argumentative — *why:* the implementer called its own safety "an argument, not a structure," one reorder from unbounded recursion — *cost if wrong:* a redundant guard on a path that never trips it — *status:* implemented
- **U13** — "Zero blast radius" for Task 27's ValidTgts$ gate change was FALSE; the real number (one card, one rider — Sword of Fire and Ice's draw stops firing) goes to Task 26 — *why:* the Go suite's claim held but 6 of 12 real decks contradicted it — *cost if wrong:* not stated — *status:* implemented (carried to Task 26 as a note)

## W1, W2

- **W1** — `cmd/forgec/main.go` must import `rules` (blank import) so RegisterNonAPI inits run; add a regression test that Supported() contains a kw:/stat:/trig: family — *why:* coverage report understated true support (20.8% vs actual 36.8%, a 5,366-card gap) — *cost if wrong:* the report lies about coverage and misdirects which primitives to build next — *status:* parked→T26 (implemented there)
- **W2** — NO `kw:` primitive registered anywhere; Task 21 must RegisterNonAPI ONLY the keywords it genuinely implements — *why:* `cards/primitive.go` emits kw: requirements for every keyword but none was ever registered; `kw:Flying` alone covers 3,274 cards — *cost if wrong:* either understates (ok) or, far worse, overstates coverage — *status:* parked→T21 (implemented there, honestly, 8 keywords)

## Task 21

- **T21-a** — (CRITICAL) A plain First Strike attacker never takes a surviving blocker's regular-step damage; decouple "blockers hit back" from the attacker's per-step gate — *why:* would make the `kw:FirstStrike` registration (W2) an overstatement — *cost if wrong:* every first-strike creature in the corpus fights wrong — *status:* implemented
- **T21-b** — (HIGH) Non-Trample multi-block must assign lethal-then-spill in declaration order (CR 510.1c), not dump all damage on the first blocker — *why:* the shipped multi-blocker test only covered the Trample path — *cost if wrong:* multi-block combat math is wrong in the most common case — *status:* implemented (attacker's ordering choice parked as T21-h)
- **T21-c** — (MEDIUM) `askBlockers` nil-pointer panic on a Face()-less attacker; require Face() != nil — *why:* remote-kill reachable from tampered/untrusted input, downstream of events.Apply's audit — *cost if wrong:* a remote-kill primitive reachable from any tampered log — *status:* implemented
- **T21-d** — (MEDIUM) A non-Trample attacker whose blocker is removed before damage must deal NO damage to the player (CR 509.1h) — *why:* dormant today but reachable from a tampered log or future removal-during-combat — *cost if wrong:* a latent bug activates silently once instant-speed removal ships — *status:* implemented
- **T21-e** — (MEDIUM) Route end-of-combat/cleanup IsAttacking/BlockedBy reset through a new appended event kind `EndCombatReset` instead of a direct write — *why:* violates the project's most load-bearing invariant (all mutation via events.Apply) — *cost if wrong:* replay silently wrong for every game containing combat — *status:* implemented
- **T21-f** — (MINOR, in scope) Fix the `onBoard` test helper to set SummonSick, matching real battlefield entry — *why:* tests built on it did not reproduce live conditions — *cost if wrong:* not stated — *status:* implemented
- **T21-g** — (MINOR) The EndCombatReset comment wrongly cites "Ruling T21-a" instead of T21-e — *why:* comments are this codebase's durable record of WHY — *cost if wrong:* a future reader chases the wrong ruling — *status:* implemented
- **T21-h** — (parked, NOT this milestone) The non-Trample multi-block "last blocker absorbs remainder" is a documented approximation of the attacker's CR 510.1a ordering choice — *why:* separate from the user's R1-R3 requirement; would balloon this fix round — *cost if wrong:* attacker cannot choose damage-assignment order; legal-but-suboptimal, never illegal — *status:* parked→future combat-UI milestone
- **T21-i** — (parked for the final whole-branch fix wave) Code's ruling-number citations disagree with the ledger's canonical mapping (event.go/combat.go cite the wrong letters) — *why:* comments are the durable record; a wrong cross-reference is a real defect — *cost if wrong:* a future reader chases the wrong ruling — *status:* parked→final wave (implemented in F-3)

## D2-a

- **D2-a** — Task 26's `TestEveryRepoDeckIsFullySupported` will FAIL on first run; that failure is the gate doing its job, not something to silence by registering unhonoured keywords — *why:* only 8 of 27 M1 keywords registered and W1 (rules import) still open — *cost if wrong:* someone silences the gate and ships an overstating coverage report — *status:* implemented (became Task 26's ratchet, P12)

## Task 22

- **T22-a** — The implementer was RIGHT not to transcribe the brief's own Step 3 sample verbatim (it crowned seat 0 on simultaneous elimination) — *why:* the dispatch instruction was later and more specific than a plan sample that contradicted it — *cost if wrong:* a draw reported where a seat-0 win was intended; cheap to reverse — *status:* accepted-as-is
- **T22-b** — Dispatch Task 22's reviewer on opus rather than the mid tier — *why:* the diff changes an existing event kind's wire meaning and touches the untrusted-log replay entry point — *cost if wrong:* spent tokens on a review a cheaper model could have done — *status:* process
- **T22-d** — Fix the SBA loop's termination condition by measuring state, not emission (re-read the object after emitting; count only what actually left) — *why:* `destroyLethalDamage`'s `changed` was keyed on emitted events, causing 32x life/log amplification under a replacement — *cost if wrong:* an SBA needing a genuine second pass gets only one; bounded — *status:* implemented (later superseded in nuance by T22-j/T22-p)
- **T22-e** — Emit a Note event on maxSBAPasses exhaustion so a cap trip is observable in the log — *why:* a silent cap trip turns an engine bug into an unattributable wrong game state — *cost if wrong:* one extra event in a path that should never execute — *status:* implemented
- **T22-f** — Genesis must begin the turn with the first ALIVE seat, not seat 0 — *why:* the unconditional `beginTurn(0)` was never guarded against an already-eliminated seat 0 — *cost if wrong:* turn order starts one seat later in a scenario no shipping deck can reach — *status:* implemented
- **T22-g** — Tighten events.Apply's GameOver handling now: first GameOver wins, later ones no-op; Amount==0 validated win, Amount==1 draw, any other Amount changes nothing — *why:* the replay entry point for untrusted logs must pin down every shape of a reused field — *cost if wrong:* a hand-written log meaning something by a stray Amount stops meaning it — *status:* implemented (missed the emitter-audit obligation, corrected by T22-k)
- **T22-h** — Fold findings 10 (zero-seat New panics) and 11 (destroyed-vs-toughness-0 share Text) into this round rather than parking — *why:* both cheap now, costly after Task 24 mints golden replays — *cost if wrong:* not stated — *status:* implemented
- **T22-i** — Re-review round 1's fix on opus, not the cheap tier, and hunt the OPPOSITE (under-firing) error — *why:* replacing one termination condition with another under the same replacement-interaction that caused the original bug — *cost if wrong:* opus tokens on a diff a mid-tier model could have cleared — *status:* process (found N1 as predicted)
- **T22-j** — N1 is real, but the reviewer's proposed fix (key `changed` on any logged movement) is WRONG — fix with an ATTEMPTED-SET tracking already-tried objects instead — *why:* the log-delta fix reintroduces item 1's 32x amplification verbatim — *cost if wrong:* an object whose destruction is replaced early and later becomes destroyable waits until next checkStateBased; bounded — *status:* implemented
- **T22-k** — Audit the OTHER GameOver emitter (`effRestartGame`, N3): fix `Amount:0`→`Amount:1` — *why:* tightening a reused field's meaning obliges auditing every emitter, which T22-g should have said and didn't — *cost if wrong:* not stated — *status:* implemented
- **T22-l** — Close N4 by IGNORING an invalid Player with Amount==0 (changes nothing), not by inventing a draw result — *why:* refusing to end the game on an untrusted log is safe/detectable; manufacturing a draw fabricates a result the log never carried — *cost if wrong:* not stated — *status:* implemented
- **T22-m** — Fix the symmetric gap in checkLoseConditions's removal sweep in round 3 rather than park it; then run ONE combined re-review over rounds 2+3 together — *why:* symmetry is the point — two loops with different termination discipline invites reintroducing the bug — *cost if wrong:* a round-2 defect survives one extra round; cheap — *status:* implemented
- **T22-n** — Fix N6 (one-attempt-per-call can under-COMPLETE when a blocker evaporates mid-call) now rather than park it — *why:* exactly what Task 23's view and Task 26's acceptance games would surface and re-debug from scratch — *cost if wrong:* a fourth touch of the loop invariant reintroduces the spin — *status:* implemented (fix direction later corrected by T22-p)
- **T22-o** — Escalate round 4 to a FRESH implementer on opus, per the loop's own rule, though not for the rule's usual reason — *why:* N6 is a consequence of the mental model shaping three rounds by one agent; a fresh reader weighs the whole invariant at once — *cost if wrong:* a fresh agent re-derives context the incumbent already had — *status:* process (vindicated by T22-p)
- **T22-p** — The fix direction endorsed in T22-n was WRONG — "re-arm when some other SBA succeeded" reintroduces pre-round-3 amplification; key the re-arm on ALIVE-PLAYER COUNT instead — *why:* the fresh implementer BUILT and MEASURED the endorsed variant and found it broke pin 2; prose has been wrong four times out of four — *cost if wrong:* not stated — *status:* implemented (vindicated independently by the final re-review)
- **T22-q** — N9's comment half must be fixed NOW; its code half (predicate-flip sub-case) is BOOKED, not patched a fifth time — *why:* the honest code fix reintroduces pin 2's amplification failure mode at 0.18% of decision points — *cost if wrong:* an SBA stays outstanding for one extra decision in 0.18% of points — *status:* implemented (comment); parked→final wave (code, closed by F-1)

## Task 23

- **T23-a** — Files land at `view/view.go`, `view/redact.go`, `view/view_test.go` at repo root; module `github.com/adams-shaun/gorge` — *why:* the repo move made every `mtgcore/` path stale — *cost if wrong:* "none" — *status:* implemented
- **T23-b** — `view.Chars` = `{Power; Toughness; Keywords; PendingTriggers}`, brief's `Derived` renamed `Keywords` (F2), all four methods REQUIRED not optional — *why:* an optional interface would silently drop pending triggers if the engine's signature drifts — *cost if wrong:* one stub method per fake — *status:* implemented
- **T23-c** — `Face()` returns nil on ability objects and tokens; every read is nil-guarded, falling back to the source's name or "Ability" — *why:* F3 applied — *cost if wrong:* nil deref on the first trigger, reachable from every client — *status:* implemented
- **T23-d** — R3 delivery: new `state.PendingTrigger`, `Engine.PendingTriggers()`, `View.Stack` becomes `[]StackView`, View gains `Pending []PendingView`, both shown to every seat — *why:* Ruling U5 placed R3 here — *cost if wrong:* view types change before any client exists — *status:* implemented
- **T23-e** — (Task 22 finding 5) View gains `Draw bool`; `Winner` becomes `*state.PlayerID`, nil unless `Over && !Draw` — *why:* JSON null cannot be mistaken for seat 0 — *cost if wrong:* "one pointer field" — *status:* implemented
- **T23-f** — Every view type carries explicit lowercase snake_case json tags; leak test needs a POSITIVE CONTROL — *why:* Go's default "ID:" key would make the brief's leak test pass VACUOUSLY — *cost if wrong:* "none" — *status:* implemented
- **T23-g** — Phase derived in view from Step: beginning/main1/combat/main2/ending — *why:* not stated — *cost if wrong:* not stated — *status:* implemented
- **T23-h** — Totality: Project must be reachable from every client (nil g/ch, out-of-range viewer/Winner, dangling ids all degrade safely) — *why:* Project is reachable from every client — *cost if wrong:* not stated — *status:* implemented
- **T23-i** — (pulls T20-g's String() half forward) `state.Step` gains `Valid()`; `Step.String()` returns "unknown" instead of panicking — *why:* apply.go stores e.Step unvalidated, reaching Project's `g.Step.String()` with a tampered value — *cost if wrong:* "6 lines" — *status:* implemented
- **T23-j** — CardView adds Controller, Owner, SummonSick — *why:* battlefield lists are keyed by controller, hidden zones by owner — *cost if wrong:* not stated — *status:* implemented
- **T23-k** — Nothing in a View aliases game or engine state (fields built fresh; attached Decision is a copy) — *why:* a Seat (Task 25) receives View in-process and must not corrupt pending state through it — *cost if wrong:* not stated — *status:* implemented
- **T23-l** — (PRE-EXISTING LEAK, fixed here) `effects/cardflow.go`'s Secret MoveZone (tutor/dig to hand) must carry `Player: p`, not the zero value — *why:* Player==0 meant seat 0 saw every other seat's tutor payload — *cost if wrong:* "one field on one emit" — *status:* implemented
- **T23-m** — R3 integration test at engine level: simultaneous triggers, Project shown to every seat during KTriggerOrder, verified through the public API — *why:* not stated — *cost if wrong:* not stated — *status:* implemented
- **T23-n** — (models) Implementer sonnet; reviewer OPUS — hidden-information leaks are the highest-stakes property, view is network-facing to every client — *why:* the brief's own leak test was vacuous — *cost if wrong:* opus tokens on a review sonnet might have cleared — *status:* process
- **T23-o** — (scope) File list grows beyond the brief's three: state/pending.go, state/ids.go, rules/trigger.go, effects/cardflow.go — *why:* R3 is user-mandated scope the plan never had — *cost if wrong:* not stated — *status:* implemented
- **T23-p** — Concern (1) (PendingTriggers tests outside the file list) is not a widening; accepted — *why:* a new exported method with no test would itself be a finding — *cost if wrong:* not stated — *status:* accepted-as-is
- **T23-q** — Concern (2), `PlayerView.ID` tagged json "seat" not "id", accepted — *why:* two id spaces under one JSON key is a wire ambiguity this project keeps paying for — *cost if wrong:* "one tag rename before any client exists" — *status:* implemented
- **T23-r** — Concern (3) (an ability's SVars read from the source's current face, not snapshotted) parked, pre-existing (Task 20) — *why:* inert until something flips faces mid-trigger (nothing in M1) — *cost if wrong:* not stated — *status:* parked→final review
- **T23-s** — (C-1 + I-2 together) Redaction becomes STATE-AWARE and ALLOWLIST-SHAPED: `RedactEvents(g, evs, viewer)` with 3 rules — *why:* two non-Secret emitters (TriggerPush.IDs, RearrangeTopOfLibrary's Note) leaked hidden ids; TriggerPush's Player is the controller not the owner — *cost if wrong:* RedactEvents carries a `*state.Game` the brief didn't give it; nothing ripples yet — *status:* implemented
- **T23-t** — (I-1, I-3) Fix mutation-proof test gaps; leak test decodes JSON and walks every id-bearing key — *why:* "tests that would not fail on deletion are not tests" — *cost if wrong:* not stated — *status:* implemented
- **T23-u** — (M-4) Every public list marshals `[]`, never null; hand/pool present-and-non-nil for the viewer, ABSENT for everyone else — *why:* omitempty cannot express "present but empty" — *cost if wrong:* not stated — *status:* implemented
- **T23-v** — (M-1,M-2,M-3,M-5) Deferred minors recorded; M-2/M-3 reflect the engine's queue truthfully — *why:* not stated — *cost if wrong:* not stated — *status:* parked (M-1→T24, M-5 documented)
- **T23-w** — (concern 2) Rule 3 EXEMPTS `events.Note` — *why:* rule 3 as specified also redacted deliberately-public Reveal/RevealHand/PeekAndReveal Notes — *cost if wrong:* a future Note emitter forgetting Secret leaks — *status:* implemented
- **T23-x** — (concern 1) NOT Task 23's, NOT parked: filed as new TASK 28 — the draw-step turn-based action must run once per step entry — *why:* a totality violation reachable from ordinary card text; Tasks 25/26 build directly on the draw step — *cost if wrong:* one small task; alternative is rediscovering it in Task 25's fuzz with no context — *status:* implemented (as Task 28)
- **T23-y** — (process) ONE combined scoped re-review over rounds 1+2 rather than two — *why:* T22-m precedent; the final state is what matters — *cost if wrong:* not stated — *status:* process
- **T23-z** — The final wave must apply rule 3's per-id owner filter to IDs/Pairs on zone-move kinds too — *why:* allowlist shape everywhere, plan-mandated by T23-s — *cost if wrong:* two extra filter calls per move event — *status:* parked→final wave (implemented in F-2)

## Task 24/25/26 pre-flight (P1–P14)

- **P1** — (ORDER: 25 before 24) Reorder remaining tasks to 23 → 25 → 24 → 26 — *why:* Task 24's brief test 1 names Task 25's bot; proves the headline replay feature over real play — *cost if wrong:* "none; both land before 26 either way" — *status:* implemented
- **P2** — (Task 24, plan defect) `run` must NOT rewrite `in.Seq = d.Seq`; submit the recorded intent verbatim — *why:* the brief's test 5 requires a Seq mismatch to be REFUSED but its own code silently repairs it — *cost if wrong:* a log recorded by a changed engine fails one intent earlier than it would have — *status:* implemented
- **P3** — (Task 24, plan defect) Compare INCREMENTALLY after each Submit, returning `*Divergence` at the first mismatch — *why:* comparing after running everything can surface an altered intent as a plain rejection instead of the demanded Divergence — *cost if wrong:* one comparison per Submit instead of one per replay — *status:* implemented
- **P4** — (Task 24) T20-g's log-level validation is NOT needed inside Replay, since Replay only compares logged events, never applies them — *why:* T23-i already made Step.String() total — *cost if wrong:* not stated — *status:* accepted-as-is
- **P5** — (Task 24) Replay overrides `cfg.Seed` with `l.Seed` — the log wins; document it — *why:* genesis is Config-recoverable only (T11-d) — *cost if wrong:* not stated — *status:* implemented
- **P6** — (Task 25, plan defect) DROP `e.L.NoHash = true` from the fuzz test — *why:* it panics per T9-b since GameStart is already appended — *cost if wrong:* not stated — *status:* implemented
- **P7** — (Task 25, plan defect) The brief's bot must answer KTriggerOrder with a seeded permutation, KTriggerOptional with a seeded coin, and fall back to the first Min distinct indices for any other kind — *why:* the old bot's empty intents get rejected by Validate, fatalling the fuzz on the first simultaneous trigger — *cost if wrong:* not stated — *status:* implemented
- **P8** — (Task 25, spec over plan) Seat is the spec's D8 signature: `Decide(ctx, view.View, decision.Decision) (decision.Intent, error)` — *why:* the WebSocket human seat (M2) needs ctx, and D8 records the interface as the design decision — *cost if wrong:* one param and one return on two implementations — *status:* implemented
- **P9** — (Task 25, plan defect) `testutil.SampleDecks(t, n)` is undefined in the brief; author it inline via `cards.ParseBytes`+Link — *why:* the fuzz test needs it with no corpus dependency — *cost if wrong:* not stated — *status:* implemented
- **P10** — (Task 25, plan defect) Invariant checker takes `(t, g *state.Game, d *decision.Decision, where string)`; add invariant 7 (no decision pending for a Life<=0 player) — *why:* invariants 3/5 need the pending decision, and testutil can't import rules (cycle) — *cost if wrong:* not stated — *status:* implemented
- **P11** — (Task 26) Deck JSONs via `//go:embed decks/*.json` in testutil, with non-testing LoadRepoDeck/OpenCorpusRegistry variants for mtgsim — *why:* no path arithmetic; works from any package dir — *cost if wrong:* not stated — *status:* implemented
- **P12** — (Task 26, D2-a made concrete) `TestEveryRepoDeckIsFullySupported` is a RATCHET against a checked-in known-gap table, asserted EXACTLY — *why:* the suite stays green while the gap stays explicit and shrinks only through real primitives — *cost if wrong:* "a table to maintain, which is the point" — *status:* implemented
- **P13** — (Task 26, W1 + T16-a) `cmd/forgec` adds a blank rules import plus a regression test that Supported() contains a kw:/stat:/trig: family; coverage report flags orphan base-type tokens — *why:* not stated — *cost if wrong:* not stated — *status:* implemented
- **P14** — (Task 26, small) Winner logging checks Draw before Winner; fix stale lint invocation; AGENTS.md status section; Makefile gains sim, build depends on mtgsim — *why:* not stated — *cost if wrong:* not stated — *status:* implemented

## Task 24

- **T24-a** — (I-1) Compare replay's chain against `l.HeadAt(len(l.Events))`, not `l.Head()` — *why:* a JSON-round-tripped or NoHash log has a zero Head(), causing a false FAILURE on a byte-identical replay — *cost if wrong:* one recomputed chain per Replay — *status:* implemented
- **T24-b** — (I-3) Fix the DOC ("a returned engine is never nil"), not the code — *why:* a nil log is a programming error; manufacturing an engine for it would hide the bug — *cost if wrong:* "one sentence" — *status:* implemented
- **T24-c** — (M-2, M-3 bundled free-to-fix) Assert Pending() positioning in the prefix test; make the resume test actually resume from a midpoint with a different bot seed and reach Over — *why:* D4's headline resume-from behaviour deserves a shipped test — *cost if wrong:* not stated — *status:* implemented

## Task 25

- **T25-a** — Review Task 25 on OPUS; measure the fuzz gate's actual coverage rather than trust 3.9M events — *why:* a fuzz gate passing because the bot passes everything is the worst, undetectable-by-reading outcome — *cost if wrong:* opus tokens — *status:* process
- **T25-b** — (I-1, deeper than reviewer's fix) Fix the sample deck's mana cost AND make the bot activate mana only during a MAIN phase — *why:* the bot tapped out at upkeep priority, so it could never pay 2 mana at sorcery speed — *cost if wrong:* a slightly smarter bot; determinism/mirror re-verified — *status:* implemented (partial regression found and fixed under T25-g)
- **T25-c** — (I-4) Clamp-and-top-up bot answers against [Min,Max] on every return path, both bot copies — *why:* the bot ignores Min/Max on non-engine-shaped decisions — *cost if wrong:* not stated — *status:* implemented
- **T25-d** — (I-3) Hoist invariant 6 above the zone-agreement walk so it can actually fire — *why:* a check in name only is worse than none — *cost if wrong:* not stated — *status:* implemented
- **T25-e** — (I-2) Add a violation test per invariant check, each independently mutation-killed — *why:* not stated — *cost if wrong:* not stated — *status:* implemented
- **T25-f** — Bundle M-1..M-5 as free-to-fix in the same round (T15-c precedent), not loop-extending — *why:* not stated — *cost if wrong:* not stated — *status:* implemented
- **T25-g** — (round 2) At KPriority outside a main phase, the bot must return "pass" EXPLICITLY, and the top-up clamp must prefer "pass" — *why:* the I-4 clamp fix reintroduced I-1(b): 94% of "activate" choices fired outside main via the clamp's fallback — *cost if wrong:* "none; a bot that passes outside main is the stated policy" — *status:* implemented

## Task 26

- **T26-a** — TASK 29: replacement effects must honour `ReplacementResult$ Updated`, plus a resolveTop totality guard for a fully-replaced-without-moving permanent spell — *why:* applyReplacements treats every match as Replaced, causing an infinite resolve loop for 2 of 12 acceptance decks — *cost if wrong:* one task; the alternative is an M1 gate that can never pass — *status:* implemented (as Task 29)
- **T26-b** — (process) Task 26's review runs NOW in parallel against a git-archive snapshot, decoupled from Task 29's implementer editing rules/ mid-flight — *why:* not stated — *cost if wrong:* one confirmation run of the acceptance after 29 — *status:* process
- **T26-c** — Fix round 1 = I-1 (add ChangeType/ChangeValid to validCardFilterKeys, correct the audit comment) bundled with M-2/M-3/M-6; M-4/M-5 deferred — *why:* not stated — *cost if wrong:* "none; comment and key-set changes" — *status:* implemented

## Task 28

- **T28-a** — Fix placement (draw as a turn-based action in advanceStep) stands; REWRITE the two tests that forge draw-step state via raw emits — *why:* the tests pinned a premise the fix removes on purpose; a test passing vacuously is a finding, not a pass — *cost if wrong:* two test rewrites; properties unchanged either way — *status:* implemented
- **T28-b** — Fix round 1 = I-1 (two assertions), I-2 (comment-only fixes), I-3 (correct the report), I-4 (state the reachable !Lost-guard case) — *why:* not stated — *cost if wrong:* "none; assertions and comments" — *status:* implemented
- **T28-c** — Re-review on SONNET — two assertions plus comment edits, no non-comment change on any sba.go/checkStateBased path — *why:* the measurement-heavy verification is already done and the fix cannot move a pin — *cost if wrong:* not stated — *status:* implemented

## Task 29

- **T29-a** — (I-1) Take the reviewer's measured fix: emit the totality guard's Note and graveyard MoveZone under `applyingReplacement` so they aren't themselves replaceable — *why:* a broad "would go to a graveyard" replacement (RIP shape) kept the resolve loop alive — *cost if wrong:* a replacement that legitimately wanted to intercept the guard's move cannot; alternative is a loop — *status:* implemented
- **T29-b** — (I-2) Reword the stack.go comment to the truth and PARK the ability-object stuck-on-stack case with the CR 800.4a/N4 family for M2 — *why:* the comment falsely claimed no equivalent zone to get stuck in — *cost if wrong:* not stated — *status:* parked→M2
- **T29-c** — (I-3) Document the M1 approximation that ETB-trigger matching runs before ReplaceWith — *why:* 8 corpus cards affected, none in a deck — *cost if wrong:* not stated — *status:* implemented

## M2 (post-M1 roadmap, user decisions)

- **M2-a** — (user) The existing mtgplay/mtgserve client contract is EVIDENCE OF NEEDS, not an API to copy; M2's interface is designed fresh — *why:* not stated — *cost if wrong:* "none in M1; shapes the next spec" — *status:* user decision
- **M2-b** — (user) Next milestone plays LEGACY 4-player on the 12 repo decks; Commander is its own follow-on milestone — *why:* not stated — *cost if wrong:* Commander format work starts one milestone later — *status:* user decision
- **M2-c** — (user) Match host = BOTH an embeddable library API (mtgserve imports in-process) plus a thin standalone `gorged` binary — *why:* not stated — *cost if wrong:* one extra package boundary to keep honest — *status:* user decision
- **M2-d** — (user) Spectator pause is a DVR CURSOR — the match keeps running; each client holds a cursor into the event log — *why:* not stated — *cost if wrong:* a pace control per table added later, ~one message type — *status:* user decision
- **M2-e** — (user) Spectator visibility is a PER-TABLE policy, `spectator: public | omniscient` — *why:* not stated — *cost if wrong:* "one flag" — *status:* user decision
- **M2-f** — (user) Client = TypeScript SPA, Svelte + Vite, in gorge under `web/`, go:embed'ed into gorged; TS envelope types generated from Go types — *why:* not stated — *cost if wrong:* framework swap on a young codebase — *status:* user decision
- **M2-g** — (user) Sequencing = spectator-first client, the 35-card ratchet as a parallel engine-only plan in its own worktree, player seat third — *why:* not stated — *cost if wrong:* one rebase between the two worktrees — *status:* user decision
- **M2-h** — (user) M2a done-when = gorged runs 4 tables x 4 bots on repo decks with overview/focus/omniscient views, DVR, late-join backfill, replay-after-restart, perpetual tables, card images — *why:* not stated — *cost if wrong:* two extra tasks in the M2a plan — *status:* user decision
- **M2-i** — (user) Transport = SSE + POST, stdlib only; WebSocket only on measured need (amends spec D5) — *why:* not stated — *cost if wrong:* a WS adapter beside the SSE one, envelope unchanged — *status:* user decision
- **M2-j** — Roadmap Section 2 (M2a architecture: host/protocol/view additions, cmd/gorged, web/ Svelte, SSE+POST) approved by user as written ("ok", 2026-09-04) — *why:* not stated — *cost if wrong:* spec sections rewritten before any plan is cut; no code yet — *status:* user decision
- **M2-k** — Roadmap Section 3 (M2a data flow/lifecycle: host match loop, perpetual tables, append-only events file, SSE stream, DVR view-at-seq, crash-halts-no-auto-restart) approved by user ("ok", 2026-09-04) — *why:* not stated — *cost if wrong:* spec rewrite before any plan — *status:* user decision
- **M2-l** — Roadmap Section 4 (testing: leak walks, view-at-seq property, protocol goldens, host determinism/crash/restart tests, httptest SSE resume, Vitest, Playwright, lint gains svelte-check+eslint) approved by user ("yes", 2026-09-04) — *why:* not stated — *cost if wrong:* spec edits — *status:* user decision

## F- (final review / merge-gate, hyphenated)

- **F-1** — (C1) Accept the reviewer's fix verbatim: extract the totality guard into `ensureLeftTheStack(id, to, why)` and call it after every stack-exit emit, not just the ETB one — *why:* only the permanent-ETB exit had Task 29's guard; 5 other exits left a Replaced-path stack object stuck forever (24 corpus cards, e.g. Rest in Peace) — *cost if wrong:* RIP-class cards misbehave but terminate; a backstop the later semantic fix makes unreachable — *status:* implemented
- **F-2** — (I2 + T23-z) Deep-copy IDs/Pairs at the top of RedactEvents' loop before any branch; add filterVisible to IDs/Pairs on the zone-move case — *why:* pass-through paths aliased the live log's IDs/Pairs, corrupting HeadAt() on a read-path mutation — *cost if wrong:* one allocation per redacted event — *status:* implemented
- **F-3** — (minors) All comment/label/rename items in the final review's must-fix table land in this wave (T21-i's mapping, I-3 label, orphan comments, renames) — *why:* not stated — *cost if wrong:* "comments" — *status:* implemented
- **F-4** — (structure) The rules/trigger.go split (1062 lines, three concerns) is NOT this wave; booked for the next milestone's first task — *why:* a structural refactor at the merge gate buys nothing the tests can measure and risks the chain heads — *cost if wrong:* one large file for a few more weeks — *status:* parked→next milestone
- **F-5** — Comment-only diff in events/ (two T21-i relabels) is accepted; code bytes unchanged — *why:* the brief's own file-list gate ("events/ empty") contradicted its own item list — *cost if wrong:* not stated — *status:* accepted-as-is
- **F-6** — R1 (ensureLeftTheStack sites unpinned by any test) is load-bearing for the totality constraint, same defect class as Critical C1; fix now as a tests-only commit rather than park — *why:* not stated — *cost if wrong:* one extra sonnet dispatch; zero running-code risk — *status:* implemented
- **F-7** — This residual commit is outside the ONE-fix-wave cap since it changes no running code; verification = orchestrator-run mutation check on a scratch copy, NOT a fourth review dispatch — *why:* not stated — *cost if wrong:* a weak test slips in; bounded by the mutant check the orchestrator runs itself — *status:* implemented
- **F-8** — R2 (a false comment about intrinsic mana ability) folded into the same commit — *why:* not stated — *cost if wrong:* "none" — *status:* implemented

---

## Line index

First line number in `progress.md` where each ID appears (computed by exact-token `grep`/regex over the full file). For the six unnumbered Task 1/3/4/5/6 rulings, the line of their "Task N: Ruling:" paragraph is used, since the ID itself is assigned by this digest and is not present verbatim in the source. Where an ID's true first appearance is a forward reference inside earlier prose (e.g. "(see T21-a)", "**F4**" in the pre-flight interface table) rather than its own "Ruling ID:" paragraph, that earlier line is reported per the literal instruction — it is still a genuine appearance of that same ID, not a coincidence. Two apparent early hits that were pure token coincidences unrelated to the ruling (an unrelated "Probe P4"/"Probe P7" numbering scheme inside the Task 27 narrative, colliding with the later Ruling P4/P7 tokens) were corrected to point at the rulings' actual defining paragraphs (3408, 3423).

| ID | Line | ID | Line |
|---|---|---|---|
| F1 | 25 | D2-a | 1959 |
| F2 | 30 | T22-a | 2016 |
| F3 | 25 | T22-b | 2033 |
| F4 | 21 | T22-d | 2106 |
| F5 | 22 | T22-e | 2120 |
| F6 | 23 | T22-f | 2133 |
| F7 | 37 | T22-g | 2146 |
| F8 | 100 | T22-h | 2162 |
| T1-a | 107 | T22-i | 2228 |
| T3-a | 142 | T22-j | 2267 |
| T4-a | 161 | T22-k | 2302 |
| T4-b | 181 | T22-l | 2314 |
| T5-a | 200 | T22-m | 2354 |
| T6-a | 236 | T22-n | 2456 |
| T8-a | 300 | T22-o | 2480 |
| T8-b | 307 | T22-p | 2508 |
| T9-a | 319 | T22-q | 2597 |
| T9-b | 324 | T23-a | 3252 |
| T9-c | 329 | T23-b | 3257 |
| T9-d | 338 | T23-c | 3270 |
| T10-a | 353 | T23-d | 3276 |
| T10-b | 361 | T23-e | 3301 |
| T10-c | 367 | T23-f | 3305 |
| T10-d | 369 | T23-g | 3313 |
| T10-e | 377 | T23-h | 3316 |
| T11-a | 389 | T23-i | 3322 |
| T11-b | 398 | T23-j | 3329 |
| T11-c | 403 | T23-k | 3333 |
| T11-d | 406 | T23-l | 3339 |
| T11-e | 413 | T23-m | 3350 |
| T11-f | 460 | T23-n | 3359 |
| T11-g | 469 | T23-o | 3365 |
| T12-a | 510 | T23-p | 3499 |
| T12-b | 522 | T23-q | 3501 |
| T12-c | 533 | T23-r | 3506 |
| T12-d | 539 | T23-s | 3559 |
| T13-a | 547 | T23-t | 3588 |
| T13-b | 562 | T23-u | 3591 |
| T14-a | 587 | T23-v | 3595 |
| T14-b | 596 | T23-w | 3624 |
| T14-d | 922 | T23-x | 3634 |
| T14-e | 962 | T23-y | 3645 |
| T14-f | 983 | T23-z | 3699 |
| T15-a | 693 | P1 | 3381 |
| T15-b | 704 | P2 | 3390 |
| T15-c | 720 | P3 | 3398 |
| T15-d | 727 | P4 | 3408 |
| T16-a | 775 | P5 | 3415 |
| T16-b | 793 | P6 | 3420 |
| T17-a | 843 | P7 | 3423 |
| T17-b | 852 | P8 | 3432 |
| T17-c | 861 | P9 | 3439 |
| T17-d | 870 | P10 | 3448 |
| T18-a | 1053 | P11 | 3455 |
| T18-b | 1069 | P12 | 3462 |
| T18-c | 1080 | P13 | 3470 |
| T19-a | 1148 | P14 | 3475 |
| T19-b | 1166 | T24-a | 4081 |
| T19-c | 1173 | T24-b | 4084 |
| T19-d | 1159 (forward ref; def. at 1181) | T24-c | 4087 |
| T19b-a | 1241 | T25-a | 3879 |
| T19b-b | 1251 | T25-b | 3911 |
| T19b-c | 1271 | T25-c | 3926 |
| T19c-a | 1334 | T25-d | 3929 |
| T19c-b | 1340 | T25-e | 3932 |
| T20-a | 1485 | T25-f | 3933 |
| T20-b | 1498 | T25-g | 3978 |
| T20-c | 1510 | T26-a | 4144 |
| T20-d | 1518 | T26-b | 4162 |
| T20-e | 1678 | T26-c | 4208 |
| T20-f | 1706 | T28-a | 3745 |
| T20-g | 1717 | T28-b | 3821 |
| U-place | 1579 | T28-c | 3826 |
| U1 | 1586 | T29-a | 4320 |
| U2 | 1593 | T29-b | 4327 |
| U3 | 1600 | T29-c | 4329 |
| U4 | 1607 | M2-a | 4353 |
| U5 | 1614 | M2-b | 4454 |
| U6 | 2690 | M2-c | 4469 |
| U7 | 2713 | M2-d | 4475 |
| U8 | 2793 | M2-e | 4481 |
| U9 | 2824 | M2-f | 4486 |
| U10 | 2950 | M2-g | 4492 |
| U11 | 2969 | M2-h | 4497 |
| U12 | 2984 | M2-i | 4504 |
| U13 | 3184 | M2-j | 4527 |
| W1 | 1632 | M2-k | 4538 |
| W2 | 1645 | M2-l | 4540 |
| T21-a | 1785 (forward ref; def. at 1803) | F-1 | 4418 |
| T21-b | 1813 | F-2 | 4435 |
| T21-c | 1794 (forward ref; def. at 1824) | F-3 | 4440 |
| T21-d | 1794 (forward ref; def. at 1831) | F-4 | 4445 |
| T21-e | 1790 (forward ref; def. at 1839) | F-5 | 4519 |
| T21-f | 1849 | F-6 | 4533 |
| T21-g | 1875 | F-7 | 4534 |
| T21-h | 1882 | F-8 | 4535 |
| T21-i | 1910 | | |

Not found anywhere in the file (see summary at top): **T14-c**, **T22-c**.
