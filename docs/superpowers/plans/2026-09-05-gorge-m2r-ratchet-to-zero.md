# gorge M2r — Ratchet to Zero Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every one of the 136 distinct cards in the 12 repo decks plays with its printed text — `rules/acceptance_test.go`'s `knownUnsupported` table shrinks from 35 entries to none — by implementing the 21 primitives those cards need, plus the engine items the M0/M1 ledger booked for this milestone.

**Architecture:** Keywords stop being opaque strings. At link time `cards` expands each keyword the engine handles into the ordinary triggered abilities, replacement effects and activated abilities Forge itself expands them to (`Keyword$ <name>` tagged), so `rules` gets Undying, Prowess, Equip or Storm through the same trigger/stack/replacement machinery it already has. Six new event kinds (appended, never inserted) let the engine mint tokens and spell copies, record how a spell was cast (X, kicked, surged, flashback, miracle), record "as it enters" choices, and attach permanents — all through `events.Apply`, so replay stays exact. A small cast-flow state machine asks the decisions casting now needs (X, delve, sacrifice costs, cast modes) in handler context; a generic `choose` decision kind carries every list-pick; the bot policy grows to answer it. Protection, tokens, attachments and copies each get their semantics in `rules`/`effects` with a registration only once a test proves the behaviour.

**Tech Stack:** Go 1.26 stdlib only. `CGO_ENABLED=0`. Forge corpus pinned to the commit the lock already records; token scripts fetched from the same commit into gitignored `.cards/tokenscripts/`.

**Spec:** `docs/superpowers/specs/2026-09-04-gorge-post-m1-roadmap.md` §"M2r scope" (items 1–6 and the "Not M2r" list), governed by `docs/superpowers/specs/2026-09-03-mtgcore-go-engine-design.md`. Card semantics come from the Comprehensive Rules and the Forge scripts themselves (`.cards/cardsfolder/…`, read-only). The rulings the ledger booked for this milestone are in `docs/superpowers/reports/2026-09-03-m0-m1/rulings.md` (F-1, F-4, T19c-b, N2, T20-g, T21-i and the hygiene minors).

## Global Constraints

Every task's requirements implicitly include this section.

- **Stdlib only; no cgo.** `go.mod` gains no `require`.
- **Every mutation goes through `events.Apply`.** `events.Kind` is append-only; **never add a field to `events.Event`** (the hash chain covers its binary encoding). This plan appends six kinds (`CastInfo`, `Choose`, `TokenCreate`, `StackCopy`, `Attach`, `AbilityPush`) and adds fields to `state.Object` and `state.Game` only. `Apply` stays a pure function of `(g, e)`: token definitions it needs come from `g.Tokens`, set at genesis.
- **Dependency order** `cards → state → decision → events → effects → rules → view → seat → replay → …`. `effects` never imports `rules`. `rules` may register effect APIs (`effects.Register`) whose implementation needs the engine — it does so by type-asserting `Host` to `*Engine` inside `rules`, never by exporting engine internals downward.
- **Determinism.** No `time`, no global `math/rand`, no map-range order reaching an event, option order, view or file. Every enumeration that builds options or events walks `AliveFrom(0)` and zone slices, never a map. Token scripts and expanded keywords are compiled in sorted path order.
- **One goroutine per match; totality.** Any reachable panic or stall is a bug. Every new decision kind and every new cast path is covered by `rules/fuzz_test.go`'s invariant run (it must keep terminating within its intent budget) and by `internal/testutil.CheckInvariants`.
- **Licensing.** The Forge corpus (GPL-3.0) lives only in gitignored `.cards/` — `cardsfolder/` and, from Task 2, `tokenscripts/`. Never commit, stage or embed a Forge `.txt`. Test fixtures are authored in the test file, never copied from a script. `cards/boundary_test.go` stays. Before every push: `git ls-files | grep -c '\.txt$'` prints `0`.
- **Chain heads are goldens (R-14).** Task 1 pins the four acceptance chain heads (2 seats `7705a6505954f6cd`, 4 `2d5589b31c4853cd`, 6 `bf4012092fdad38b`, 8 `01b9f48c1b6dc135`) in `rules/heads_test.go`. A task that changes what the 12 decks do **must** update that table in the same commit and name, in the commit body, the card behaviour that moved it (e.g. "Monastery Swiftspear's Prowess now pumps"). A head that moves without a named cause is a bug. `make sim` must still print 20/20 `replay OK` after every task (its chains change with the engine; replay exactness is what it guards).
- **The ratchet only shrinks, and only by proof.** `knownUnsupported` loses an entry when — and only when — the primitive it names is registered, and a primitive is registered when — and only when — a test drives a real card through the behaviour. Registering to make the table green is forbidden (Ruling W2). The table's expected size after each task is stated in that task.
- **The coverage report may move either way.** `make report`'s `playable` count is recorded in each task's commit body. Keyword expansion (Task 11) can temporarily lower it (an expanded keyword now also needs, say, `api:Attach`) — that is honest, not a regression.
- **Gates every task runs before its commit:** `make lint`, `go build ./...`, `go test -count=1 ./...`, `go test -race -count=1 ./rules/ ./effects/ ./events/`, `make sim | grep -c 'replay OK'` = 20, `make report | grep '^cards:'`, `git ls-files | grep -c '\.txt$'` = 0, and the acceptance run `go test ./rules/ -run 'TestEveryRepoDeck|TestRepoDecks|TestRepoDeckGames|TestHeads' -count=1`.
- **Approximations are documented, never silent.** Where this plan chooses an engine-side default for a player choice the engine cannot yet ask (R-8, R-9), the primitive's doc comment says so and `AGENTS.md`'s "Known approximations" list (Task 21) names it with the milestone that removes it.
- **Isolation.** M2r runs in its own worktree on a branch `m2r/ratchet` off `main` (superpowers:using-git-worktrees). Task 1 is merged to `main` alone, first (fast-forward after review), so M2a never rebases across the file split. Every later task lands on the branch; the branch merges to `main` at the end (or in reviewed slices — the orchestrator decides per slice). M2r touches `cards/`, `effects/`, `rules/`, `state/`, `events/`, `decision/`, `seat/bot.go`, `internal/testutil/`, `cmd/forgec/`, `Makefile`, `AGENTS.md`, `README.md`, and exactly one `view/` edit (Task 4, four lines in `cardViews`). It never touches `host/`, `protocol/`, `web/`, `replay/`.
- **Git.** Never bare `git stash`/`git stash pop`. No commit carries a `Co-Authored-By` or any AI-attribution trailer (user rule, 2026-09-05).
- **Comment discipline.** Doc comments say what and why; they do not narrate ledger history. A ruling ID is cited only where the code would otherwise be surprising.

## Plan-level rulings (fixed here; executors treat them as spec)

- **R-1 Keywords expand at link time.** `cards.(*Face).expandKeywords()` runs inside `Link()` and appends, for each keyword the engine implements, the Forge-equivalent `Trigger`/`Repl`/`SA` entries with `Params["Keyword"] = "<name>"` and, where a description is wanted, `TriggerDescription$`. Downstream nothing knows the difference: `events.Apply`'s `TriggerPush` indexes `f.Triggers` and finds the expanded entry; `view.abilityText` shows its description; `rules` matches it like any T: line. `Face.Primitives()` still lists `kw:<name>` from `Keywords`, so the *registration* of the keyword remains the honesty gate — the expansion only supplies the mechanics. *Cost if wrong:* the expansion table is one file; deleting it restores today's inert keywords.
- **R-2 Six appended event kinds; new Object/Game fields.** `CastInfo{Obj, Amount: X, Counter: "kicked,surged,flashback,miracle"}` → `Object.X`, `Object.CastFlags`; `Choose{Obj, Counter: "name"|"type"|"number", Text}` → `Object.ChosenName/ChosenType/ChosenNumber`; `TokenCreate{Player: owner, Text: script key}` → mints from `g.Tokens[Text]`; `StackCopy{Obj: source spell, Player: controller}` → mints a copy on the stack; `Attach{Obj: attachment, IDs: [target] or empty}` → `Object.AttachedTo`; `AbilityPush{Obj: source, Amount: f.Abilities index, Player}` → mints an ability object (the activated twin of `TriggerPush`). `Game.Tokens map[string]*cards.Card` and `rules.Config.Tokens` carry the token definitions. *Cost if wrong:* kinds are append-only; unused ones cost one `case` each.
- **R-3 Corpus pinned; token scripts fetched.** `forgec fetch` accepts a commit SHA (`git fetch --depth 1 origin <sha>` after a sparse, no-checkout clone) and adds `forge-gui/res/tokenscripts` to the sparse set, landing at `.cards/tokenscripts/`. The `Makefile` pins `FORGE_REF` to the lock's commit `95f04e8a04c8925fa97cb226fc3341cabcc90a53` so every head movement in M2r is attributable to the engine, not upstream card edits. `Registry.Tokens` is keyed by script stem (`r_1_1_goblin`); the IR cache version bumps to 2. *Cost if wrong:* a refetch and recompile.
- **R-4 One generic choice decision.** `decision.KChoose = "choose"`: "pick between Min and Max of these options", options labelled by `Kind` (`"x"`, `"exile"`, `"sacrifice"`, `"name"`, `"type"`, `"number"`, `"yes"`, `"no"`). It carries X, Delve exiles, sacrifice costs, "as enters" names/types/numbers and may-pay yes/nos. The wire shape (`Decision`/`Intent`) is unchanged (Ruling U2). `seat.Bot` and `rules`' `testBot` learn one policy: X = the largest affordable; exile/sacrifice = the maximum allowed, lowest ids first; name/type/number = the first option; yes/no = yes when affordable. *Cost if wrong:* one Kind constant and two switch cases.
- **R-5 Casting is a small state machine.** `rules/cast.go` owns `pendingCast{player, card, mode, x, delve, sacrificed}`; `handlePriority`'s "cast" hands over to it; it asks its `KChoose` decisions one at a time (X, then Delve exiles, then non-mana costs), each answered through `handle`'s new `KChoose` case, and commits with `CastInfo` + the cost payments + `PutOnStack` (or the fizzle path). Every ask happens in handler context, where the engine may already suspend. `Option` gains server-side `Mode string` (json `-`): `""`, `"kicked"`, `"surged"`, `"flashback"`, `"miracle"`. *Cost if wrong:* a struct and a file; the old inline `castSpell` is its degenerate path.
- **R-6 "As it enters, choose …" is asked when the card is cast or played, not when it enters.** `ETBReplacement:Other:<SVar>` with `NameCard`/`ChooseType`/`ChooseNumber` asks its `KChoose` in the cast flow (or `play_land`), records the answer with a `Choose` event on the card, and the replacement's effect at ETB is then a no-op if already chosen. Opponents learn the choice a resolution early — a documented deviation; the correct fix (resumable resolution) is the mid-resolution decision machinery M2d-2 delivers (rules/resolution.go). *Cost if wrong:* moving the ask to resolution once that machinery exists.
- **R-7 Copies are ephemeral stack objects.** A `StackCopy` object shares the original's `Card`/`FaceIdx`/`Ability`/`Source`, copies `Targets`, sets `IsCopy`, and leaves the stack to exile (the same parking spot resolved ability objects already use — CR 608.2m has no "ceases to exist" zone here). `view.cardViews` skips `IsCopy` objects (and Face-less ones, landed by M2a Task 4); `effects.MatchesSpecFrom` never matches an `IsCopy` object off the stack; `Count$` heads therefore never count one. *Cost if wrong:* a flag and two guards.
- **R-8 `UnlessCost$` ("may pay … if they do …") is declined.** Chain Lightning's copy clause needs the target's controller to decide mid-resolution; the engine could not suspend a resolution until M2d-2 (KModes and the mid-resolution ask, rules/resolution.go) delivered that machinery. `CopySpellAbility` is implemented and registered because Storm's copies exercise it fully; the may-pay clause is declined deterministically with a `Note`. Documented in the primitive and in `AGENTS.md`. *Cost if wrong:* one `if` once mid-resolution asks exist.
- **R-9 Engine-deterministic stand-ins for in-resolution player choices stay.** `Sacrifice` with `SacValid$` for a *player* target sacrifices that player's lowest-id matching permanent; `Discard` keeps "first in hand"; `Charm` keeps "first mode"; `Mana | Produced$ Any` keeps colourless. Each is documented and listed. *Cost if wrong:* none now; M2d-2 replaces Charm with a real KModes decision (UnlessCost$ is already real there); the rest await a later milestone.
- **R-10 `Defined$`'s default is Self when the ability names no targets.** Forge's own rule: with neither `Defined$` nor `ValidTgts$`, an effect acts on its source; with `ValidTgts$`, on the chosen targets. `effects.Defined` gains that rule (it used to return `c.Targets` unconditionally). This changes what cards already in the decks do (Batterskull's bounce, Rancor's return, and others) and therefore moves chain heads in Task 4 — ledgered there. *Cost if wrong:* one branch.
- **R-11 Last known information for zone changes.** `Engine.emit` snapshots the moving object (a struct copy) before `events.Emit` applies a `MoveZone`/`Draw`/`PutOnStack`, and hands it to `checkTriggers`; a matched trigger's `Ctx.LKI *state.Object` carries it. Undying's "if it had no +1/+1 counters" reads `LKI.Counter("P1P1")`, not the post-move object (whose counters `Move` has already cleared). *Cost if wrong:* one struct copy per zone change.
- **R-12 Triggered and activated abilities take targets.** `pushTrigger`/`pushAbility` call `askTarget` when the ability's SA declares `ValidTgts$`, honouring `TargetMin$`/`TargetMax$` (default 1/1; `TargetMin$ 0` — N2 — lets a spell resolve with none) and `TgtZone$` (`Graveyard`, `Battlefield`; default battlefield + players). `askTarget` is widened once, for spells and abilities alike. *Cost if wrong:* Min/Max fields on a decision the client already renders.
- **R-13 Protection lives in `rules`.** `protectedFrom(target, source)` reads the target's *derived* keywords for `Protection from <quality>` and the source's colours (`effects.ColorsOf`, honouring Devoid). It gates blocking (`canBlock`), targeting (`askTarget`, `legalTargets`), attaching (`attach`), and damage — the last as an emit-time rule: a `Damage` event whose source (the resolving stack object, or the combat attacker/blocker recorded on the assignment) the target is protected from is dropped before it is logged. *Cost if wrong:* one predicate and four call sites.
- **R-14 Chain-head goldens.** `rules/heads_test.go` pins the four acceptance heads; `TestHeads` fails when they move. Each task that moves them edits the table with a named cause in the commit body. *Cost if wrong:* a table edit per task.
- **R-15 `Host.HasKeyword`.** `effects.Host` gains `HasKeyword(id state.ObjID, kw string) bool` so `Destroy`/`DestroyAll` respect *granted* Indestructible (today they read the printed face only). *Cost if wrong:* one interface method, implemented by `*rules.Engine` already.
- **R-16 Miracle is rules-native.** The expansion tags the card; `rules` watches `Draw` events: the first card a player draws this turn (counted from the log since the last `TurnChange`) with Miracle asks its owner a `KChoose` yes/no; yes enters the cast flow in `"miracle"` mode with the miracle cost (X asked as usual). *Cost if wrong:* one trigger mode in `rules`.
- **R-17 Activated abilities are options.** `legalActions` lists every `AB$` ability of a permanent (and of a graveyard card when `ActivationZone$ Graveyard`) whose cost is payable, honouring `SorcerySpeed$ True` and `CantBeActivated` with `ValidSA$`; `Option.Ability int` (json `-`) says which. Activation pays the cost, emits `AbilityPush`, then asks targets (R-12). *Cost if wrong:* a field and an enumeration loop.
- **R-18 Cost grammar.** `rules.Cost` gains `X int` (count of X symbols, replacing the bool), `Tap bool`, `Sac []CostPart{N int; Spec string}`, `SubCounter []CostPart{N int; Kind string}`, parsed from Forge's `Cost$` (`T`, `Sac<1/Creature>`, `SubCounter<2/P1P1>`, `2 C`, `X X`). `payCost` pays mana, taps, removes counters and — after a `KChoose` — sacrifices. *Cost if wrong:* a parser; the mana half is unchanged.
- **R-19 Register what is already honoured, with proof.** `kw:Flash` (legal.go already gates instant-speed casting on it; `TestFlashCreatureIsCastableOffTurn` is the proof), `kw:Indestructible` (SBA and, after R-15, Destroy), `kw:Devoid` (colour helper) are registered in Task 3 with a test each. *Cost if wrong:* three strings.
- **R-20 The ratchet's end state keeps the test.** Task 18 replaces the table with `map[string][]string{}` and the test asserts `len(measured) == 0` with the same in-both-directions message, so a future regression names the card.

## File structure

| Path | Responsibility | Task |
|---|---|---|
| `rules/trigger_match.go`, `rules/trigger_queue.go`, `rules/replacement.go` | the split of `rules/trigger.go` (pure move) | 1 |
| `rules/heads_test.go` | chain-head goldens | 1 |
| `cards/fetch.go`, `cards/registry.go`, `cards/tokens.go`, `cmd/forgec/main.go`, `Makefile` | SHA fetch, token scripts, `Registry.Tokens`, cache v2 | 2 |
| `effects/colors.go`, `effects/registry.go` (Host.HasKeyword), `effects/zone.go`, `rules/combat.go` (registrations) | Flash/Indestructible/Devoid | 3 |
| `state/object.go`, `state/game.go`, `events/event.go`, `events/apply.go`, `rules/engine.go`, `rules/clone.go`, `view/view.go` (4 lines) | fields, six kinds, Apply cases, Config.Tokens | 4 |
| `effects/context.go`, `effects/count.go`, `effects/filter.go`, `effects/cardflow.go` (Dig All) | Defined default & forms, Count$ heads, predicates with SVar/chosen context | 5 |
| `rules/engine.go` (emit LKI), `rules/trigger_match.go`, `effects/registry.go` (Ctx.LKI) | last known information | 6 |
| `rules/stack.go` (askTarget widened), `rules/trigger_queue.go` | abilities take targets; TargetMin/Max; TgtZone | 7 |
| `decision/decision.go`, `seat/bot.go`, `rules/testbot_test.go`, `rules/turn.go` (handle) | `KChoose` | 8 |
| `rules/mana.go` (Cost grammar), `rules/cast.go`, `rules/legal.go`, `rules/stack.go` | cast flow: X, Kicker, Surge, Flashback, Delve, costs | 9 |
| `rules/legal.go`, `rules/activate.go`, `rules/statics.go` | activated abilities | 10 |
| `cards/keywords.go`, `cards/link.go`, `cards/registry.go` (cache v3) | keyword expansion | 11 |
| `rules/replacement.go`, `rules/cast.go`, `effects/choose.go`, `rules/statics.go` | etbCounter, ETBReplacement choices, NamedCard/ChosenType/cmcEQChosen | 12 |
| `effects/token.go` | tokens | 13 |
| `effects/attach.go`, `rules/attach.go`, `rules/sba.go`, `rules/stack.go` (aura resolution) | attachments, Equip, Enchant, Living Weapon | 14 |
| `rules/protection.go`, `rules/combat.go`, `rules/stack.go`, `rules/engine.go` (emit rule) | protection | 15 |
| `rules/trigger_match.go` (conditions), `cards/keywords.go` | Undying, Evolve, Exalted, Prowess | 16 |
| `effects/copy.go`, `rules/storm.go` | Storm, CopySpellAbility | 17 |
| `rules/miracle.go` | Miracle | 18 |
| `effects/context.go` (ReplacedCard), `rules/replacement.go` | `Defined$ ReplacedCard` | 19 |
| `effects/misc.go`, `effects/count.go`, `rules/mana.go` | hygiene minors | 20 |
| `rules/acceptance_test.go`, `AGENTS.md`, `README.md` | end state, docs | 21 |

## Ratchet schedule

| After task | `knownUnsupported` entries | Cards that drop |
|---|---|---|
| 3 | 31 | Snapcaster Mage, Spectral Sailor (Flash); Ulamog (Indestructible); World Breaker (Devoid) |
| 9 | 24 | Gatekeeper of Malakir, Goblin Bushwhacker, Vines of Vastwood (Kicker); Reckless Bushwhacker (Surge); Cabal Therapy (Flashback); Gurmag Angler, Tombstalker (Delve) |
| 12 | 17 | Chalice of the Void, Endless One, Walking Ballista (etbCounter); Cavern of Souls, Phyrexian Revoker, Pithing Needle, Sanctum Prelate (ETBReplacement) |
| 13 | 15 | Young Pyromancer, Wurmcoil Engine (Token) |
| 14 | 11 | Batterskull (Equip, Living Weapon), Sword of Fire and Ice, Umezawa's Jitte (Equip), Rancor (Enchant) |
| 15 | 10 | Goblin Piledriver (Protection from blue) |
| 16 | 5 | Geralf's Messenger, Strangleroot Geist (Undying); Experiment One (Evolve); Knight of Infamy (Exalted + Protection from white); Monastery Swiftspear (Prowess) |
| 17 | 2 | Tendrils of Agony, Empty the Warrens (Storm + Token); Chain Lightning (CopySpellAbility) |
| 18 | 0 | Terminus, Entreat the Angels (Miracle + Token) |

Tasks 4–8, 10 and 11 change no ratchet entry: they are infrastructure. Every task states its expected table size; the acceptance test enforces it in both directions.

---

## Phase 0 — structure and corpus

### Task 1: split `rules/trigger.go`; pin the chain heads

**Files:**
- Create: `rules/trigger_match.go`, `rules/trigger_queue.go`, `rules/replacement.go`, `rules/heads_test.go`
- Delete: `rules/trigger.go`

**Interfaces:**
- Consumes: the whole of `rules/trigger.go` (1062 lines, three concerns).
- Produces: identical behaviour, byte-identical chain heads. The three files' contents (functions moved verbatim, comments included):
  - `trigger_match.go`: `pendingTrigger`, `triggerKey`, `maxTriggerFires`, `forEachObject`, `controllerOf`, `checkTriggers`, `triggerRemembered`, `triggerMatches`, `zoneGate`, `zoneChangeMatches`, `spellCastMatches`, `attacksMatches`, `damageSource`, `damageMatches`, `becomesTargetMatches`, `landPlayedMatches`, `phaseMatches`, the `init()` registration.
  - `trigger_queue.go`: `putTriggersOnStack`, `sortPendingTriggers`, `dropDepartedTriggers`, `popFrontTrigger`, `removeTriggerAt`, `takeAnsweredTrigger`, `pushTrigger`, `triggerOf`, `optionalDecider`, `PendingTriggers`, `triggerLabel`, `askTriggerOrder`, `handleTriggerOrder`, `frontIsTheOfferedGroup`, `askTriggerOptional`, `handleTriggerOptional`.
  - `replacement.go`: `applyReplacements`, `replacementMatches`.
  The package doc comment that opens `trigger.go` (if any) goes to `trigger_match.go`; each new file opens with a two-sentence comment naming its concern.

This is the roadmap's item 1 and Ruling F-4; it merges to `main` alone and first.

- [ ] **Step 1: Write the heads golden test (it passes today)**

Create `rules/heads_test.go`:

```go
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
)

// acceptanceHeads pins the chain head of the deterministic acceptance game
// at each seat count (R-14). A change here is a change to what the 12 repo
// decks do; the commit that makes it must name the card behaviour that
// moved it. The seeds and deck assignment are TestRepoDeckGamesReplayExactly's
// own (rules/acceptance_test.go), so the two tests always agree.
var acceptanceHeads = map[int]string{
	2: "7705a6505954f6cd",
	4: "2d5589b31c4853cd",
	6: "bf4012092fdad38b",
	8: "01b9f48c1b6dc135",
}

func TestHeads(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, seats := range []int{2, 4, 6, 8} {
		got := acceptanceHead(t, reg, seats)
		if want := acceptanceHeads[seats]; got != want {
			t.Errorf("%d seats: chain head %s, golden %s — if this move is intended, update acceptanceHeads and name the cause in the commit body", seats, got, want)
		}
	}
}
```

`acceptanceHead(t, reg, seats)` is whatever helper `rules/acceptance_test.go` uses to play the deterministic acceptance game for `TestRepoDeckGamesReplayExactly` (read that test: it builds the seat→deck assignment from `testutil.RepoDeckNames()` and a fixed seed, plays with the `testBot`, and compares a replay). Factor that game-playing body into `acceptanceHead(t, reg, seats) string` in `acceptance_test.go` if it is not already a function, and have the existing test call it too — the helper must be the *same* code path so the goldens and the replay test cannot drift apart.

Run: `go test ./rules/ -run 'TestHeads|TestRepoDeckGamesReplayExactly' -count=1 -v`
Expected: PASS; the `-v` output shows the four heads above.

- [ ] **Step 2: Split the file**

Move the functions listed under Interfaces into the three new files with `sed -n`/editor cut-and-paste — not by retyping. Every function, type, constant and comment moves verbatim; the only new text is each file's opening comment. Imports per file are whatever `gofmt`/`go vet` require (`goimports` is not available; add them by hand). Delete `rules/trigger.go`.

- [ ] **Step 3: Prove nothing changed**

Run: `go build ./... && go vet ./rules/ && go test -count=1 ./rules/ && go test -race -count=1 ./rules/ && go test ./rules/ -run TestHeads -count=1 && make sim | grep -c 'replay OK'`
Expected: build clean, all rules tests PASS, heads unchanged, `20`.
Run: `git diff --stat HEAD -- rules/ | tail -1` and `git diff HEAD --diff-filter=M -- rules/*.go | grep -c '^[-+][^-+]'` — the only modified file should be `acceptance_test.go` (the helper extraction); everything else is adds/deletes. Then compare the moved bodies: `cat rules/trigger_match.go rules/trigger_queue.go rules/replacement.go | grep -v '^package\|^import\|^\t"' | sort > /tmp/after.txt; git show HEAD:rules/trigger.go | grep -v '^package\|^import\|^\t"' | sort > /tmp/before.txt; diff /tmp/before.txt /tmp/after.txt` — the diff must show only the new file-header comment lines, the closing `)` of removed import blocks, and blank lines.

- [ ] **Step 4: Gates and commit**

Run: `make lint && go test -count=1 ./... && make report | grep '^cards:' && git ls-files | grep -c '\.txt$'`
Expected: clean; `cards: 33667  playable: 15265 (45.3%)`; `0`.

```bash
git add rules/
git commit -m "refactor(rules): split trigger.go into matching, queue and replacements; pin chain heads

Pure move: every function, type and comment is byte-identical to its
source. rules/heads_test.go pins the four acceptance chain heads as
goldens so every later change to what the repo decks do is named."
```

Merge this commit to `main` (fast-forward) before any other M2r task starts; M2a never rebases across it.

---

### Task 2: corpus pinned by commit; token scripts fetched and compiled

**Files:**
- Modify: `cards/fetch.go` (SHA refs; second sparse path), `cards/fetch_test.go`, `cards/registry.go` (`Tokens`, cache v2, `CompileDir` compiles `tokenscripts/`), `cards/registry_test.go`, `cmd/forgec/main.go` (report prints token count), `Makefile` (`FORGE_REF ?= 95f04e8a04c8925fa97cb226fc3341cabcc90a53`), `AGENTS.md` (the fetch line)
- Create: `cards/tokens.go`, `cards/tokens_test.go`

**Interfaces:**
- Consumes: `cards.Parse` (token scripts use the same line grammar: `Name`, `ManaCost`, `Types`, `Colors`, `PT`, `K`, `Oracle`).
- Produces: `func TokensDir(dir string) string` (`<dir>/tokenscripts`); `Registry.Tokens map[string]*Card` keyed by script stem; `func (r *Registry) Token(key string) (*Card, bool)`; `Lock.FetchedPaths []string` (replacing `FetchedPath`, keeping the JSON key `fetched_path` for the first entry so the existing lock still reads); `cacheVersion = 2`.

- [ ] **Step 1: Look at three real token scripts before writing the test**

Run: `ls .cards/tokenscripts 2>/dev/null | head` — absent before this task. Fetch once by hand to see the format the test must mirror: in a scratch directory, `git clone --depth 1 --filter=blob:none --sparse https://github.com/Card-Forge/forge.git t && cd t && git sparse-checkout set forge-gui/res/tokenscripts && sed -n '1,20p' forge-gui/res/tokenscripts/r_1_1_goblin.txt forge-gui/res/tokenscripts/c_3_3_a_phyrexian_wurm_deathtouch.txt forge-gui/res/tokenscripts/b_0_0_phyrexian_germ.txt`. Note the exact lines (`Name:`, `ManaCost:no cost`, `Types:`, `Colors:`, `PT:`, `K:`, `Oracle:`). Delete the scratch clone. If a line kind appears that `ParseBytes` diagnoses as "unkeyed" or unknown, add it to the parser as an ignored key with a comment, in this task.

- [ ] **Step 2: Write the failing tests**

Append to `cards/tokens_test.go`:

```go
package cards

import (
	"os"
	"path/filepath"
	"testing"
)

const goblinTokenSrc = "Name:Goblin Token\nManaCost:no cost\nTypes:Creature Goblin\nColors:red\nPT:1/1\nOracle:\n"
const wurmTokenSrc = "Name:Phyrexian Wurm Token\nManaCost:no cost\nTypes:Artifact Creature Phyrexian Wurm\nColors:colorless\nPT:3/3\nK:Deathtouch\nOracle:Deathtouch\n"

func TestCompileDirLoadsTokenScriptsByStem(t *testing.T) {
	dir := t.TempDir()
	writeCardFile(t, CorpusDir(dir), "mountain.txt", "Name:Mountain\nTypes:Basic Land Mountain\nOracle:\n")
	writeCardFile(t, TokensDir(dir), "r_1_1_goblin.txt", goblinTokenSrc)
	writeCardFile(t, TokensDir(dir), "c_3_3_a_phyrexian_wurm_deathtouch.txt", wurmTokenSrc)
	r, diags, err := CompileDir(CorpusDir(dir))
	if err != nil || len(diags) != 0 {
		t.Fatalf("%v %v", err, diags)
	}
	if len(r.Cards) != 1 {
		t.Fatalf("tokens leaked into Cards: %d", len(r.Cards))
	}
	tok, ok := r.Token("r_1_1_goblin")
	if !ok || tok.Faces[0].Name != "Goblin Token" || tok.Faces[0].Power() != 1 || tok.Faces[0].Colors != "red" {
		t.Fatalf("goblin token %+v", tok)
	}
	if w, ok := r.Token("c_3_3_a_phyrexian_wurm_deathtouch"); !ok || !w.Faces[0].HasKeyword("Deathtouch") {
		t.Fatal("wurm token lacks Deathtouch")
	}
	if _, ok := r.Lookup("Goblin Token"); ok {
		t.Fatal("a token is not a card: Lookup must not find it")
	}
}

func TestTokensSurviveTheCache(t *testing.T) {
	dir := t.TempDir()
	writeCardFile(t, CorpusDir(dir), "mountain.txt", "Name:Mountain\nTypes:Basic Land Mountain\nOracle:\n")
	writeCardFile(t, TokensDir(dir), "r_1_1_goblin.txt", goblinTokenSrc)
	r, _, _ := CompileDir(CorpusDir(dir))
	cache := filepath.Join(dir, "ir.gob.gz")
	if err := r.Save(cache); err != nil {
		t.Fatal(err)
	}
	back, err := LoadRegistry(cache)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := back.Token("r_1_1_goblin"); !ok {
		t.Fatal("token lost through Save/Load")
	}
}

func TestCompileDirWithoutTokensStillCompiles(t *testing.T) {
	dir := t.TempDir()
	writeCardFile(t, CorpusDir(dir), "mountain.txt", "Name:Mountain\nTypes:Basic Land Mountain\nOracle:\n")
	if _, err := os.Stat(TokensDir(dir)); !os.IsNotExist(err) {
		t.Fatal("fixture has a tokens dir")
	}
	r, _, err := CompileDir(CorpusDir(dir))
	if err != nil || len(r.Tokens) != 0 {
		t.Fatalf("%v, %d tokens", err, len(r.Tokens))
	}
}
```

`writeCardFile(t, dir, name, src)` exists in `cards/registry_test.go` (line ~171); check its signature and reuse it. `CompileDir` takes the *cardsfolder* path today; it derives the tokens path as the sibling `tokenscripts/` (`filepath.Join(filepath.Dir(cardsDir), "tokenscripts")`).

Append to `cards/fetch_test.go` (it already drives `fetchRepo` against a local repository):

```go
func TestFetchByCommitSHAAndTokenScripts(t *testing.T) {
	repo := newLocalForgeRepo(t) // the existing helper that builds a bare repo with forge-gui/res/cardsfolder
	addTokenScript(t, repo, "r_1_1_goblin.txt", goblinTokenSrc)
	sha := headSHA(t, repo)
	dir := t.TempDir()
	l, err := fetchRepo(repo, dir, sha)
	if err != nil {
		t.Fatal(err)
	}
	if l.Commit != sha || len(l.FetchedPaths) != 2 {
		t.Fatalf("lock %+v", l)
	}
	if _, err := os.Stat(filepath.Join(TokensDir(dir), "r_1_1_goblin.txt")); err != nil {
		t.Fatal("token script not fetched")
	}
}
```

Adapt `newLocalForgeRepo`/`addTokenScript`/`headSHA` to the helpers `fetch_test.go` already has (it must have a local-repo builder — the file's existing tests pass without network); add the two small helpers beside them if missing. Run: `go test ./cards/ -run 'Token|FetchByCommit' -count=1` — FAIL.

- [ ] **Step 3: Implement**

`cards/tokens.go`:

```go
package cards

import "path/filepath"

// TokensDir is where Fetch places Forge's token scripts inside dir. They
// share the card scripts' line grammar and licence (GPL-3.0, never
// committed) and are keyed by file stem — "r_1_1_goblin" — which is the
// name a card's TokenScript$ parameter uses.
func TokensDir(dir string) string { return filepath.Join(dir, "tokenscripts") }

// Token looks a token definition up by script stem.
func (r *Registry) Token(key string) (*Card, bool) {
	c, ok := r.Tokens[key]
	return c, ok
}
```

`cards/registry.go`: add `Tokens map[string]*Card` to `Registry` (initialised in `NewRegistry`); `cacheFile` gains `Tokens map[string]*Card`; `cacheVersion = 2`; `Save` encodes it; `LoadRegistry` restores it (`r.Tokens = cf.Tokens`, nil-safe). `CompileDir(dir)`: after the cards loop, walk `filepath.Join(filepath.Dir(dir), "tokenscripts")` if it exists — sorted paths, `Parse`, `Link`, `ApplyIntrinsics`, key = stem — into `r.Tokens`; tokens are **not** `Add`ed to `Cards`/`byName`. A missing tokens directory is not an error (the tests for a bare cardsfolder still pass).

`cards/fetch.go`: `forgeSubpaths = []string{"forge-gui/res/cardsfolder", "forge-gui/res/tokenscripts"}`; `Lock.FetchedPaths []string \`json:"fetched_paths"\`` plus keep `FetchedPath string \`json:"fetched_path,omitempty"\`` populated with the first for the old lock's readers. `fetchRepo`: when `ref` looks like a full SHA (40 hex chars), clone with `--no-checkout --filter=blob:none --sparse` (no `--branch`), `sparse-checkout set <paths…>`, `git fetch --depth 1 origin <sha>`, `git checkout <sha>`; otherwise the existing `--branch` path with both sparse paths. Move each fetched subpath to its destination (`CorpusDir`, `TokensDir`). The stderr notice counts both.

`Makefile`: `FORGE_REF ?= 95f04e8a04c8925fa97cb226fc3341cabcc90a53` with a comment "pinned to the lock's commit for M2r; a corpus bump is a deliberate, ledgered change". `cmd/forgec/main.go`: `report` prints `tokens: N` after the corpus line; `compile` prints the token count.

- [ ] **Step 4: Refetch, recompile, verify the pin**

Run: `make fetch-cards && make compile-cards && cat .cards/cards.lock`
Expected: the lock's `commit` is still `95f04e8a…`, `fetched_paths` lists both, `.cards/tokenscripts/` holds Forge's token scripts (thousands of files), `ir.gob.gz` is version 2. `ls .cards/tokenscripts | grep -c 'r_1_1_goblin\|w_4_4_angel_flying\|c_3_3_a_phyrexian_wurm_deathtouch\|c_3_3_a_phyrexian_wurm_lifelink\|r_1_1_elemental\|b_0_0_phyrexian_germ'` prints `6` — every token the ratchet cards create exists.
Run: `go test -count=1 ./cards/ ./rules/ && make report | head -3 && make sim | grep -c 'replay OK' && go test ./rules/ -run TestHeads -count=1`
Expected: PASS; `cards: 33667  playable: 15265 (45.3%)` (the card corpus is byte-identical: same commit); `20`; heads unchanged.

- [ ] **Step 5: Gates and commit**

```bash
git ls-files | grep -c '\.txt$'   # 0
git add cards/ cmd/forgec/main.go Makefile AGENTS.md
git commit -m "feat(cards): fetch token scripts, pin the corpus to a commit, Registry.Tokens"
```

---

### Task 3: register Flash, Indestructible and Devoid — with proof

**Files:**
- Create: `effects/colors.go`, `effects/colors_test.go`, `rules/keyword_registration_test.go`
- Modify: `effects/registry.go` (`Host.HasKeyword`), `effects/zone.go` (Destroy/DestroyAll consult the host), `rules/stack.go` (Engine satisfies the new method — `HasKeyword` already exists on `*Engine`), `rules/combat.go` (`init` registrations), `rules/acceptance_test.go` (`knownUnsupported` loses four entries), every test double that implements `effects.Host` (grep `AddContinuous(` in `*_test.go`; add a `HasKeyword` method returning the printed keyword)

**Interfaces:**
- Produces: `func ColorsOf(o *state.Object) string` — the object's colours as WUBRG letters (`"R"`, `"UB"`, `""` for colourless), honouring Devoid (always `""`) and a Face-less object (`""`); `Host.HasKeyword(id state.ObjID, kw string) bool`. Registered: `kw:Flash`, `kw:Indestructible`, `kw:Devoid`. Ratchet: 31.

- [ ] **Step 1: Write the failing tests**

`effects/colors_test.go`:

```go
package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func objFrom(t *testing.T, src string) *state.Object {
	t.Helper()
	c, diags := cards.ParseBytes("x.txt", []byte(src))
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	return &state.Object{ID: 1, Card: c}
}

func TestColorsOf(t *testing.T) {
	cases := []struct{ src, want string }{
		{"Name:Bolt\nManaCost:R\nTypes:Instant\nOracle:x\n", "R"},
		{"Name:Dimir\nManaCost:U B\nTypes:Instant\nOracle:x\n", "UB"},
		{"Name:Colorless\nManaCost:3\nTypes:Artifact\nOracle:x\n", ""},
		{"Name:Colored by line\nManaCost:2\nTypes:Artifact\nColors:green\nOracle:x\n", "G"},
		{"Name:Breaker\nManaCost:6 G\nTypes:Creature Eldrazi\nK:Devoid\nOracle:x\n", ""},
		{"Name:Land\nManaCost:no cost\nTypes:Land\nOracle:x\n", ""},
		{"Name:Hybrid\nManaCost:W/U\nTypes:Instant\nOracle:x\n", "WU"},
	}
	for _, tc := range cases {
		if got := ColorsOf(objFrom(t, tc.src)); got != tc.want {
			t.Errorf("%q: %q, want %q", tc.src[:20], got, tc.want)
		}
	}
	if ColorsOf(&state.Object{}) != "" || ColorsOf(nil) != "" {
		t.Fatal("faceless/nil object is not colourless")
	}
}
```

`rules/keyword_registration_test.go`:

```go
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/effects"
)

// Registered keywords must each have a behaviour test naming them; this
// pins the M2r registrations against the tests that prove them.
func TestRegisteredKeywordsAreHonoured(t *testing.T) {
	sup := effects.Supported()
	for kw, proof := range map[string]string{
		"kw:Flash":          "TestFlashCreatureIsCastableOffTurn",
		"kw:Indestructible": "TestIndestructibleSurvivesLethalDamageAndDestroy",
		"kw:Devoid":         "TestDevoidCreatureIsColourless",
	} {
		if !sup[kw] {
			t.Errorf("%s is not registered (proof test: %s)", kw, proof)
		}
	}
}

func TestIndestructibleSurvivesLethalDamageAndDestroy(t *testing.T) {
	// Printed Indestructible survives lethal damage (SBA) and a Destroy
	// effect; a Destroy against a creature that GAINED Indestructible via a
	// Pump also does nothing (Host.HasKeyword reads derived keywords).
	e, cfg, id := newFixtureDeck(t, 5, "Name:Rock\nManaCost:1\nTypes:Creature Wall\nPT:0/3\nK:Indestructible\nOracle:x\n")
	_ = cfg
	// Put it onto the battlefield the way the fixture helpers do, then hit it.
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 5})
	e.checkStateBased()
	if e.G.Obj(id).Zone != state.ZBattlefield {
		t.Fatal("indestructible creature died to lethal damage")
	}
	ctx := &effects.Ctx{Source: id, Controller: 0, Targets: []state.Target{{Obj: id}}}
	effects.Resolve(e, ctx, &cards.SA{Kind: "DB", API: "Destroy", Params: map[string]string{}})
	if e.G.Obj(id).Zone != state.ZBattlefield {
		t.Fatal("indestructible creature was destroyed")
	}
	// Granted: a plain bear pumped with KW$ Indestructible.
	e2, _, bear := newFixtureDeck(t, 6, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e2.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZHand, To: state.ZBattlefield})
	effects.Resolve(e2, &effects.Ctx{Source: bear, Controller: 0, Targets: []state.Target{{Obj: bear}}},
		&cards.SA{Kind: "DB", API: "Pump", Params: map[string]string{"KW": "Indestructible"}})
	effects.Resolve(e2, &effects.Ctx{Source: bear, Controller: 0, Targets: []state.Target{{Obj: bear}}},
		&cards.SA{Kind: "DB", API: "Destroy", Params: map[string]string{}})
	if e2.G.Obj(bear).Zone != state.ZBattlefield {
		t.Fatal("Destroy ignored granted Indestructible")
	}
}

func TestDevoidCreatureIsColourless(t *testing.T) {
	e, _, id := newFixtureDeck(t, 7, "Name:Breaker\nManaCost:6 G\nTypes:Creature Eldrazi\nK:Devoid\nOracle:x\n")
	if got := effects.ColorsOf(e.G.Obj(id)); got != "" {
		t.Fatalf("devoid creature has colours %q", got)
	}
}
```

(imports: `cards`, `events`, `state`.) `newFixtureDeck(t, seed, src) (*Engine, Config, state.ObjID)` is `rules/replacement_updated_test.go`'s helper: it builds a two-seat game with one copy of `src` in seat 0's hand and returns its id — reuse it. Run: `go test ./effects/ -run Colors -count=1; go test ./rules/ -run 'Registered|Indestructible|Devoid' -count=1` — FAIL.

- [ ] **Step 2: Implement**

`effects/colors.go`:

```go
package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/state"
)

// ColorsOf is an object's colours as WUBRG letters in that fixed order:
// the colours of its mana cost, or an explicit Colors: line for a card
// whose cost does not show them (a token, an artifact "that is green").
// Devoid makes a card colourless regardless (CR 702.114). A Face-less
// object (an ability, a copy of nothing) is colourless. Protection (rules)
// and the colour predicates read this rather than the face directly.
func ColorsOf(o *state.Object) string {
	if o == nil {
		return ""
	}
	f := o.Face()
	if f == nil || f.HasKeyword("Devoid") {
		return ""
	}
	set := map[byte]bool{}
	for _, r := range f.ManaCost {
		if strings.ContainsRune("WUBRG", r) {
			set[byte(r)] = true
		}
	}
	if len(set) == 0 && f.Colors != "" {
		for _, word := range strings.Split(strings.ToLower(f.Colors), ",") {
			switch strings.TrimSpace(word) {
			case "white":
				set['W'] = true
			case "blue":
				set['U'] = true
			case "black":
				set['B'] = true
			case "red":
				set['R'] = true
			case "green":
				set['G'] = true
			}
		}
	}
	var b strings.Builder
	for _, c := range "WUBRG" {
		if set[byte(c)] {
			b.WriteRune(c)
		}
	}
	return b.String()
}
```

Also make `filter.go`'s colour predicates (`White`…`Green`) read `ColorsOf(o)` (`strings.Contains(ColorsOf(o), "W")`), so a Devoid card stops matching `Green` — say so in a comment.

`effects/registry.go`: add to `Host`:

```go
	// HasKeyword reports a DERIVED keyword — printed or granted by a
	// continuous effect (rules.Engine.HasKeyword). Effects that gate on a
	// keyword (Destroy on Indestructible) must ask this, never the face.
	HasKeyword(id state.ObjID, kw string) bool
```

`effects/zone.go`: `effDestroy` and `effDestroyAll` replace `o.Face() != nil && o.Face().HasKeyword("Indestructible")` with `h.HasKeyword(o.ID, "Indestructible")`. Every test `Host` double in `effects/*_test.go` and `rules/*_test.go` gains `func (h *fakeHost) HasKeyword(id state.ObjID, kw string) bool { o := h.g.Obj(id); return o != nil && o.Face() != nil && o.Face().HasKeyword(kw) }` (find them with `grep -ln 'AddContinuous(' --include='*_test.go' -r effects rules`).

`rules/combat.go` `init`: add `"kw:Flash", "kw:Indestructible", "kw:Devoid"` with the comment "Flash: legal.go's instant-speed gate; Indestructible: destroyLethalDamage and, via Host.HasKeyword, Destroy/DestroyAll; Devoid: effects.ColorsOf. Each has a named proof test in keyword_registration_test.go." `rules/acceptance_test.go`: delete the Snapcaster Mage, Spectral Sailor, Ulamog and World Breaker entries; the table comment's "35 of the 136" becomes a pointer to the plan's ratchet schedule.

- [ ] **Step 3: Run, gates, commit**

Run: `go test -count=1 ./effects/ ./rules/ && go test ./rules/ -run 'TestEveryRepoDeck|TestHeads' -count=1 -v | grep -E 'ratchet|ok|FAIL'`
Expected: PASS; the ratchet log line reads `31 of 136`; heads unchanged (nothing these cards do in the acceptance games changes — Flash was already honoured, no acceptance game destroys an indestructible creature; if a head moves, find out why before proceeding).
Run: `make lint && go build ./... && go test -count=1 ./... && make sim | grep -c 'replay OK' && make report | grep '^cards:'`
Expected: `20`; playable count rises (three keywords unlock corpus cards) — record it.

```bash
git add effects/ rules/
git commit -m "feat(rules): register Flash, Indestructible and Devoid with proof tests

Host gains HasKeyword so Destroy respects granted Indestructible;
effects.ColorsOf is the one colour reader (Devoid-aware). Ratchet 35 -> 31.
make report: <playable count>."
```

---

## Phase 1 — engine infrastructure

### Task 4: object fields, six event kinds, `Apply` cases

**Files:**
- Modify: `state/object.go`, `state/game.go`, `state/game_test.go`, `events/event.go`, `events/apply.go`, `events/apply_test.go`, `rules/engine.go` (`Config.Tokens` → `Game.Tokens`), `view/view.go` (`cardViews` skip), `view/identity_test.go` (one test), `internal/testutil/invariants.go` (ephemeral objects may sit in no zone list only when in exile)
- Modify if present: `rules/clone.go` (new fields are value types; nothing to add unless a slice was introduced)

**Interfaces:**
- Produces, in `state`:

```go
const (
	FlagKicked    uint8 = 1 << iota // CastFlags bits
	FlagSurged
	FlagFlashback
	FlagMiracle
)
type Object struct { /* existing … */
	X            int32   // the value chosen for {X} when this was cast
	CastFlags    uint8   // how it was cast (Flag*)
	ChosenName   string  // "as it enters, choose a card name"
	ChosenType   string  // … a creature type
	ChosenNumber int32   // … a number
	AttachedTo   ObjID   // the permanent this Aura/Equipment is attached to; 0 = none
	IsToken      bool
	IsCopy       bool
}
func (o *Object) Ephemeral() bool // IsToken || IsCopy || Card == nil: out of the game once off the battlefield/stack
type Game struct { /* existing … */ Tokens map[string]*cards.Card } // token definitions by script key, set at genesis
```

in `events`: kinds `CastInfo`, `Choose`, `TokenCreate`, `StackCopy`, `Attach`, `AbilityPush` appended after `EndCombatReset`, names `"cast_info"`, `"choose"`, `"token_create"`, `"stack_copy"`, `"attach"`, `"ability_push"`; `func FlagsFrom(s string) uint8` / `func FlagsString(f uint8) string` (csv of `kicked,surged,flashback,miracle` in that fixed order). `TargetsChosen` learns two more `Amount` shapes: `2` appends `IDs` as object targets, `3` appends `Player` as a player target (0 and 1 keep replacing, so every existing log applies unchanged).

in `rules`: `Config.Tokens map[string]*cards.Card` copied to `Game.Tokens` in `New`.

- [ ] **Step 1: Write the failing tests**

Append to `events/apply_test.go` (it already has a purity harness — `TestApplyIsPure` or similar — and a two-seat game builder; reuse them):

```go
func TestCastInfoRecordsXAndFlags(t *testing.T) {
	g, id := gameWithOneCard(t) // helper in this file: two seats, one card in seat 0's hand
	Apply(g, Event{Kind: CastInfo, Obj: id, Amount: 3, Counter: "kicked,flashback"})
	o := g.Obj(id)
	if o.X != 3 || o.CastFlags != state.FlagKicked|state.FlagFlashback {
		t.Fatalf("%+v", o)
	}
	if FlagsString(o.CastFlags) != "kicked,flashback" || FlagsFrom("surged,miracle") != state.FlagSurged|state.FlagMiracle {
		t.Fatal("flag round trip")
	}
	Move(g, id, state.ZHand, state.ZBattlefield)
	if g.Obj(id).X != 3 || g.Obj(id).CastFlags == 0 {
		t.Fatal("cast info must survive onto the battlefield (ETB 'if it was kicked')")
	}
	Move(g, id, state.ZBattlefield, state.ZGraveyard)
	if g.Obj(id).X != 0 || g.Obj(id).CastFlags != 0 {
		t.Fatal("cast info must reset when the permanent leaves the battlefield")
	}
}

func TestChooseRecordsNameTypeAndNumber(t *testing.T) {
	g, id := gameWithOneCard(t)
	Apply(g, Event{Kind: Choose, Obj: id, Counter: "name", Text: "Lightning Bolt"})
	Apply(g, Event{Kind: Choose, Obj: id, Counter: "type", Text: "Goblin"})
	Apply(g, Event{Kind: Choose, Obj: id, Counter: "number", Amount: 2})
	o := g.Obj(id)
	if o.ChosenName != "Lightning Bolt" || o.ChosenType != "Goblin" || o.ChosenNumber != 2 {
		t.Fatalf("%+v", o)
	}
	Apply(g, Event{Kind: Choose, Obj: id, Counter: "colour", Text: "x"}) // unknown kind: no-op, no panic
	Apply(g, Event{Kind: Choose, Obj: 999, Counter: "name", Text: "x"})
}

func TestTokenCreateMintsFromTheGameTokenTable(t *testing.T) {
	g, _ := gameWithOneCard(t)
	tok, _ := cards.ParseBytes("tok.txt", []byte("Name:Goblin Token\nManaCost:no cost\nTypes:Creature Goblin\nColors:red\nPT:1/1\nOracle:\n"))
	g.Tokens = map[string]*cards.Card{"r_1_1_goblin": tok}
	before := len(g.Objs)
	Apply(g, Event{Kind: TokenCreate, Player: 1, Text: "r_1_1_goblin"})
	if len(g.Objs) != before+1 {
		t.Fatal("no object minted")
	}
	o := g.Obj(state.ObjID(before + 1))
	if !o.IsToken || o.Owner != 1 || o.Controller != 1 || o.Zone != state.ZBattlefield || o.Face().Name != "Goblin Token" || !o.SummonSick {
		t.Fatalf("token %+v", o)
	}
	if bf := g.Zone(state.ZBattlefield, 1); len(bf) != 1 || bf[0] != o.ID {
		t.Fatal("token not in its controller's battlefield list")
	}
	Apply(g, Event{Kind: TokenCreate, Player: 1, Text: "no_such_token"}) // unknown key: no-op
	Apply(g, Event{Kind: TokenCreate, Player: 9, Text: "r_1_1_goblin"})  // invalid player: no-op
	if len(g.Objs) != before+1 {
		t.Fatal("bad TokenCreate minted something")
	}
}

func TestStackCopyDuplicatesASpellAboveIt(t *testing.T) {
	g, id := gameWithOneCard(t)
	Apply(g, Event{Kind: PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
	Apply(g, Event{Kind: TargetsChosen, Obj: id, Player: 1, Amount: 1})
	Apply(g, Event{Kind: CastInfo, Obj: id, Amount: 2, Counter: "kicked"})
	Apply(g, Event{Kind: StackCopy, Obj: id, Player: 0})
	if len(g.Stack) != 2 || g.Stack[1] == id {
		t.Fatalf("stack %v", g.Stack)
	}
	c := g.Obj(g.Stack[1])
	if !c.IsCopy || c.Card != g.Obj(id).Card || c.FaceIdx != g.Obj(id).FaceIdx || c.Controller != 0 || c.X != 2 || c.CastFlags != state.FlagKicked {
		t.Fatalf("copy %+v", c)
	}
	if len(c.Targets) != 1 || !c.Targets[0].IsPlayer || c.Targets[0].Player != 1 {
		t.Fatalf("copy targets %v", c.Targets)
	}
	c.Targets[0].Player = 0
	if g.Obj(id).Targets[0].Player != 1 {
		t.Fatal("copy shares its Targets array with the original")
	}
	Apply(g, Event{Kind: StackCopy, Obj: 999, Player: 0}) // nothing there: no-op
	Move(g, id, state.ZStack, state.ZGraveyard)
	Apply(g, Event{Kind: StackCopy, Obj: id, Player: 0}) // not on the stack: no-op
	if len(g.Stack) != 1 {
		t.Fatalf("stack %v", g.Stack)
	}
}

func TestAttachAndDetach(t *testing.T) {
	g, eq := gameWithOneCard(t)
	tgt := g.AddObject(g.Obj(eq).Card, 0).ID
	Move(g, eq, state.ZLibrary, state.ZBattlefield)
	Move(g, tgt, state.ZLibrary, state.ZBattlefield)
	Apply(g, Event{Kind: Attach, Obj: eq, IDs: []state.ObjID{tgt}})
	if g.Obj(eq).AttachedTo != tgt {
		t.Fatal("not attached")
	}
	Apply(g, Event{Kind: Attach, Obj: eq})
	if g.Obj(eq).AttachedTo != 0 {
		t.Fatal("not detached")
	}
	Apply(g, Event{Kind: Attach, Obj: eq, IDs: []state.ObjID{tgt}})
	Move(g, eq, state.ZBattlefield, state.ZGraveyard)
	if g.Obj(eq).AttachedTo != 0 {
		t.Fatal("leaving the battlefield must detach")
	}
	Apply(g, Event{Kind: Attach, Obj: eq, IDs: []state.ObjID{999}}) // unknown target: no-op
}

func TestAbilityPushMintsAnActivatedAbilityObject(t *testing.T) {
	g, id := gameWithOneCardSrc(t, "Name:Sailor\nManaCost:U\nTypes:Creature Spirit\nPT:1/1\nA:AB$ Draw | Cost$ 3 U | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\nOracle:x\n")
	Move(g, id, state.ZHand, state.ZBattlefield)
	Apply(g, Event{Kind: AbilityPush, Obj: id, Player: 0, Amount: 0})
	if len(g.Stack) != 1 {
		t.Fatal("no ability object")
	}
	ab := g.Obj(g.Stack[0])
	if ab.Card != nil || ab.Ability != g.Obj(id).Face().Abilities[0] || ab.Source != id || ab.Controller != 0 {
		t.Fatalf("ability object %+v", ab)
	}
	Apply(g, Event{Kind: AbilityPush, Obj: id, Player: 0, Amount: 7}) // index out of range: no-op
	Apply(g, Event{Kind: AbilityPush, Obj: id, Player: 9, Amount: 0}) // invalid player: no-op
	if len(g.Stack) != 1 {
		t.Fatal("bad AbilityPush minted something")
	}
}

func TestTargetsChosenAppendShapes(t *testing.T) {
	g, id := gameWithOneCard(t)
	other := g.AddObject(g.Obj(id).Card, 1).ID
	Apply(g, Event{Kind: PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack})
	Apply(g, Event{Kind: TargetsChosen, Obj: id, IDs: []state.ObjID{other}})          // replace with objects
	Apply(g, Event{Kind: TargetsChosen, Obj: id, Player: 1, Amount: 3})               // append player
	Apply(g, Event{Kind: TargetsChosen, Obj: id, IDs: []state.ObjID{id}, Amount: 2})  // append object
	tg := g.Obj(id).Targets
	if len(tg) != 3 || tg[0].Obj != other || !tg[1].IsPlayer || tg[1].Player != 1 || tg[2].Obj != id {
		t.Fatalf("targets %+v", tg)
	}
	Apply(g, Event{Kind: TargetsChosen, Obj: id, Player: 0, Amount: 1}) // shape 1 still replaces
	if tg := g.Obj(id).Targets; len(tg) != 1 || !tg[0].IsPlayer {
		t.Fatalf("targets %+v", tg)
	}
}
```

`gameWithOneCard`/`gameWithOneCardSrc` — write them beside the file's existing builders if absent: two seats named Ann/Bob, one card (default a 2/2 Bear) parsed with `cards.ParseBytes`, in seat 0's hand via `AddObject` + `SetZone`; the object's `Zone` set to `ZHand`. Extend the file's purity test so every new kind is applied to two independent clones and compared — the existing test almost certainly iterates a list of sample events; add one of each new kind to it.

Append to `state/game_test.go`:

```go
func TestCloneCopiesTheNewFieldsAndSharesTokens(t *testing.T) {
	g := NewGame([]string{"a", "b"})
	g.Tokens = map[string]*cards.Card{"x": {}}
	o := g.AddObject(nil, 0)
	o.X, o.CastFlags, o.ChosenName, o.ChosenType, o.ChosenNumber, o.AttachedTo, o.IsToken, o.IsCopy = 2, FlagKicked, "n", "t", 4, 7, true, true
	c := g.Clone()
	co := c.Obj(o.ID)
	if *co != *o {
		t.Fatalf("clone %+v vs %+v", *co, *o)
	}
	if c.Tokens["x"] != g.Tokens["x"] {
		t.Fatal("Tokens is immutable and should be shared, not copied")
	}
	if !co.Ephemeral() || !(&Object{}).Ephemeral() || (&Object{Card: &cards.Card{}}).Ephemeral() {
		t.Fatal("Ephemeral")
	}
}
```

(`*co != *o` compares structs with slice fields — Counters etc. are nil here so it compiles and holds; if the compiler rejects the comparison because of a slice field, compare the eight new fields individually.)

Run: `go test ./events/ ./state/ -count=1` — FAIL.

- [ ] **Step 2: Implement**

`state/object.go`: the fields and constants above, plus:

```go
// Ephemeral reports an object that exists only while on the stack or the
// battlefield: a token (CR 111.7), a copy of a spell or ability (CR
// 707.10), or an ability object (no card). Off those zones it has ceased
// to exist; this build parks such objects in exile, and view/filters skip
// them there.
func (o *Object) Ephemeral() bool { return o.IsToken || o.IsCopy || o.Card == nil }
```

`state/game.go`: `Tokens map[string]*cards.Card` on `Game` with the comment "Tokens is the token definitions this match may create, keyed by Forge script stem; set at genesis, never mutated, so Clone shares it." `Clone` needs no change (struct copy shares the map).

`events/event.go`: append the six kinds with a comment block each (what the fields mean — copy the R-2 shapes), extend `kindNames`, and add:

```go
// Cast flags travel as text so an event stays readable; the order is fixed
// so FlagsString(FlagsFrom(s)) is canonical.
var flagNames = [...]struct {
	name string
	bit  uint8
}{{"kicked", state.FlagKicked}, {"surged", state.FlagSurged}, {"flashback", state.FlagFlashback}, {"miracle", state.FlagMiracle}}

func FlagsFrom(s string) uint8 {
	var f uint8
	for _, part := range strings.Split(s, ",") {
		for _, fn := range flagNames {
			if strings.TrimSpace(part) == fn.name {
				f |= fn.bit
			}
		}
	}
	return f
}

func FlagsString(f uint8) string {
	var parts []string
	for _, fn := range flagNames {
		if f&fn.bit != 0 {
			parts = append(parts, fn.name)
		}
	}
	return strings.Join(parts, ",")
}
```

`events/apply.go` cases (each guarded like its siblings — `validPlayer`, nil objects, ranges):

```go
	case CastInfo:
		if o := g.Obj(e.Obj); o != nil {
			o.X = e.Amount
			o.CastFlags = FlagsFrom(e.Counter)
		}

	case Choose:
		if o := g.Obj(e.Obj); o != nil {
			switch e.Counter {
			case "name":
				o.ChosenName = e.Text
			case "type":
				o.ChosenType = e.Text
			case "number":
				o.ChosenNumber = e.Amount
			}
		}

	case TokenCreate:
		if !validPlayer(g, e.Player) {
			break
		}
		def, ok := g.Tokens[e.Text]
		if !ok || def == nil {
			break
		}
		o := g.AddObject(def, e.Player)
		o.IsToken = true
		Move(g, o.ID, state.ZLibrary, state.ZBattlefield)

	case StackCopy:
		if !validPlayer(g, e.Player) {
			break
		}
		src := g.Obj(e.Obj)
		if src == nil || src.Zone != state.ZStack {
			break
		}
		o := g.AddObject(src.Card, e.Player)
		o.Controller = e.Player
		Move(g, o.ID, state.ZLibrary, state.ZStack)
		o.FaceIdx, o.Ability, o.Source = src.FaceIdx, src.Ability, src.Source
		o.Targets = append([]state.Target(nil), src.Targets...)
		o.Remembered = append([]state.Target(nil), src.Remembered...)
		o.X, o.CastFlags, o.IsCopy = src.X, src.CastFlags, true

	case Attach:
		if o := g.Obj(e.Obj); o != nil {
			if len(e.IDs) == 0 {
				o.AttachedTo = 0
			} else if g.Obj(e.IDs[0]) != nil {
				o.AttachedTo = e.IDs[0]
			}
		}

	case AbilityPush:
		if !validPlayer(g, e.Player) {
			break
		}
		src := g.Obj(e.Obj)
		if src == nil {
			break
		}
		f := src.Face()
		if f == nil || e.Amount < 0 || int(e.Amount) >= len(f.Abilities) {
			break
		}
		o := g.AddObject(nil, e.Player)
		Move(g, o.ID, state.ZLibrary, state.ZStack)
		o.Ability = f.Abilities[e.Amount]
		o.Source = e.Obj
		for _, id := range e.IDs {
			o.Remembered = append(o.Remembered, state.Target{Obj: id})
		}
```

`TargetsChosen`: add before the existing branches `case e.Amount == 2: append object targets from IDs; case e.Amount == 3: if validPlayer append the player` — as a `switch` with the existing 0/1 behaviour unchanged. `Move`'s default (leaving battlefield/stack) reset gains `o.X, o.CastFlags = 0, 0`, `o.ChosenName, o.ChosenType, o.ChosenNumber = "", "", 0`, `o.AttachedTo = 0` — but **only** when the object is leaving the *battlefield* for X/CastFlags/Chosen (a spell moving hand→stack→battlefield must keep them): implement as `if from == state.ZBattlefield { … }` inside the default branch, using the `o.Zone` value read before the move. `AttachedTo` resets on any non-battlefield destination.

`rules/engine.go`: `Config.Tokens map[string]*cards.Card` (doc: "token definitions the decks can create; cards.Registry.Tokens. Replay must pass the same table."); `New` sets `e.G.Tokens = cfg.Tokens`. `internal/testutil` and `cmd/mtgsim`, `host` (if present) pass `reg.Tokens` when they build a `Config` — `grep -rn 'rules.Config{' --include='*.go' .` and add `Tokens: reg.Tokens` where a registry is at hand (tests using `SampleDecks` have none and pass nil).

`view/view.go` `cardViews`: extend the skip to `o == nil || o.Face() == nil || o.IsCopy || (o.IsToken && o.Zone != state.ZBattlefield)` with the comment "Ephemeral objects (copies, tokens off the battlefield, ability objects) have ceased to exist; they are parked in exile by the engine and are not cards in this zone." Add `TestZoneListsSkipEphemeralObjects` to `view/identity_test.go` (a token created then moved to exile does not appear; a copy in exile does not appear; a token on the battlefield does).

`internal/testutil/invariants.go`: no change needed — ephemeral objects still live in exactly one zone list (exile); the invariant holds.

- [ ] **Step 3: Run, gates, commit**

Run: `go test -count=1 ./state/ ./events/ ./view/ ./rules/ && go test -race -count=1 ./events/ && go test ./rules/ -run TestHeads -count=1 && make sim | grep -c 'replay OK'`
Expected: PASS, heads unchanged (nothing emits the new kinds yet), `20`.

```bash
git add state/ events/ rules/engine.go view/ internal/ cmd/
git commit -m "feat(events): cast info, choices, tokens, copies, attachments and activated abilities as events

Six appended kinds and the Object/Game fields they set; Apply stays a pure
function of (game, event) — token definitions come from Game.Tokens."
```

---

### Task 5: `effects` foundations — Defined, Count, predicates, Dig All

**Files:**
- Modify: `effects/context.go`, `effects/context_test.go`, `effects/count.go`, `effects/count_test.go`, `effects/filter.go`, `effects/filter_test.go`, `effects/cardflow.go` (`ChangeNum$ All`), `rules/acceptance_test.go` (no table change), `rules/heads_test.go` (heads move — see Step 3)

**Interfaces:**
- Produces:

```go
// context.go
func Defined(h Host, c *Ctx, sa *cards.SA) []state.Target   // R-10 default; new forms below
// forms: "" (targets if the SA has ValidTgts$, else Self), You, Self, Parent (Self), Remembered, Targeted, ParentTarget,
//        TriggeredCard, TriggeredCardLKICopy, TriggeredNewCardLKICopy, TriggeredSpellAbility, TriggeredAttacker (object entries of Remembered),
//        TriggeredDefendingPlayer, TriggeredPlayer (player entries of Remembered), TriggeredCardController (controller of the first object entry),
//        Opponent, Player, Equipped (the source's AttachedTo, when set — Task 14 wires the field's producer)
// count.go
//   Count$CardCounters.<KIND>  — counters of that kind on the source
//   Count$Kicked.<yes>.<no>    — yes when the source was kicked (CastFlags), else no
//   /Times.N                   — multiply
// filter.go
type SpecContext struct { You state.PlayerID; Source state.ObjID; Resolve func(name string) (int32, bool) }
func MatchesSpecCtx(g *state.Game, spec string, id state.ObjID, sc SpecContext) bool
// MatchesSpec / MatchesSpecFrom stay as wrappers (Resolve nil).
// predicates added: kicked, surged, StrictlyOther (= Other), NamedCard, ChosenType, and numeric RHS that is not a literal
//   ("cmcEQY", "cmcEQChosen", "powerGEX") resolved through sc.Resolve; an unresolvable RHS never matches.
// An IsCopy object off the stack never matches anything.
```

- [ ] **Step 1: Write the failing tests**

`effects/context_test.go` (append; the file has a fake host — reuse it, adding `HasKeyword` if Task 3 did not already):

```go
func TestDefinedDefaultsToSelfWithoutTargets(t *testing.T) {
	h, c := fixtureHost(t) // seat 0 controls object 1
	got := Defined(h, c, &cards.SA{Params: map[string]string{}})
	if len(got) != 1 || got[0].Obj != c.Source {
		t.Fatalf("no ValidTgts, no Defined: %v, want Self", got)
	}
	c.Targets = []state.Target{{Player: 1, IsPlayer: true}}
	got = Defined(h, c, &cards.SA{Params: map[string]string{"ValidTgts": "Player"}})
	if len(got) != 1 || !got[0].IsPlayer {
		t.Fatalf("ValidTgts present: %v, want the chosen targets", got)
	}
	got = Defined(h, c, &cards.SA{Params: map[string]string{}})
	if len(got) != 1 || got[0].Obj != c.Source {
		t.Fatalf("a sub-ability without ValidTgts acts on Self even when the root had targets: %v", got)
	}
}

func TestDefinedTriggeredForms(t *testing.T) {
	h, c := fixtureHost(t)
	c.Remembered = []state.Target{{Obj: 2}, {Player: 1, IsPlayer: true}}
	for _, form := range []string{"TriggeredCard", "TriggeredCardLKICopy", "TriggeredNewCardLKICopy", "TriggeredSpellAbility", "TriggeredAttacker"} {
		got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": form}})
		if len(got) != 1 || got[0].Obj != 2 {
			t.Errorf("%s: %v", form, got)
		}
	}
	for _, form := range []string{"TriggeredDefendingPlayer", "TriggeredPlayer"} {
		got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": form}})
		if len(got) != 1 || !got[0].IsPlayer || got[0].Player != 1 {
			t.Errorf("%s: %v", form, got)
		}
	}
	got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "TriggeredCardController"}})
	if len(got) != 1 || !got[0].IsPlayer || got[0].Player != h.Game().Obj(2).Controller {
		t.Errorf("TriggeredCardController: %v", got)
	}
	if got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "Parent"}}); len(got) != 1 || got[0].Obj != c.Source {
		t.Errorf("Parent: %v", got)
	}
	if got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "Equipped"}}); len(got) != 0 {
		t.Errorf("Equipped with nothing attached: %v", got)
	}
	h.Game().Obj(c.Source).AttachedTo = 2
	if got := Defined(h, c, &cards.SA{Params: map[string]string{"Defined": "Equipped"}}); len(got) != 1 || got[0].Obj != 2 {
		t.Errorf("Equipped: %v", got)
	}
}
```

`effects/count_test.go` (append):

```go
func TestCountCardCountersKickedAndTimes(t *testing.T) {
	h, c := fixtureHost(t)
	src := h.Game().Obj(c.Source)
	src.AddCounter("CHARGE", 3)
	if n := EvalCount(h, c, "Count$CardCounters.CHARGE"); n != 3 {
		t.Fatalf("CardCounters.CHARGE = %d", n)
	}
	if n := EvalCount(h, c, "Count$CardCounters.P1P1"); n != 0 {
		t.Fatalf("absent counter kind = %d", n)
	}
	if n := EvalCount(h, c, "Count$Kicked.4.0"); n != 0 {
		t.Fatalf("not kicked = %d", n)
	}
	src.CastFlags = state.FlagKicked
	if n := EvalCount(h, c, "Count$Kicked.4.0"); n != 4 {
		t.Fatalf("kicked = %d", n)
	}
	if n := EvalCount(h, c, "Count$CardCounters.CHARGE/Times.2"); n != 6 {
		t.Fatalf("Times.2 = %d", n)
	}
}
```

`effects/filter_test.go` (append):

```go
func TestSpecContextResolvesNumericRHSAndChoices(t *testing.T) {
	g, id := twoSeatGame(t, "Name:Bolt\nManaCost:R\nTypes:Instant\nOracle:x\n") // cmc 1; helper in this file or write it
	src := g.AddObject(g.Obj(id).Card, 0)
	src.ChosenName, src.ChosenType, src.ChosenNumber = "Bolt", "Goblin", 1
	sc := SpecContext{You: 0, Source: src.ID, Resolve: func(name string) (int32, bool) {
		switch name {
		case "Y":
			return 1, true
		case "Chosen":
			return src.ChosenNumber, true
		}
		return 0, false
	}}
	for spec, want := range map[string]bool{
		"Card.cmcEQY":       true,
		"Card.cmcEQChosen":  true,
		"Card.cmcGTY":       false,
		"Card.cmcEQZ":       false, // unresolvable: never matches
		"Card.NamedCard":    true,
		"Card.ChosenType":   false, // Bolt is not a Goblin
		"Card.kicked":       false,
		"Card.StrictlyOther": true,
	} {
		if got := MatchesSpecCtx(g, spec, id, sc); got != want {
			t.Errorf("%s: %v, want %v", spec, got, want)
		}
	}
	g.Obj(id).CastFlags = state.FlagKicked | state.FlagSurged
	if !MatchesSpecCtx(g, "Card.kicked+surged", id, sc) {
		t.Error("kicked+surged")
	}
	if MatchesSpecFrom(g, "Card.cmcEQY", id, 0, src.ID) {
		t.Error("MatchesSpecFrom has no resolver and must not match an SVar-shaped RHS")
	}
	// A copy off the stack matches nothing.
	g.Obj(id).IsCopy = true
	g.Obj(id).Zone = state.ZExile
	if MatchesSpecCtx(g, "Card", id, sc) {
		t.Error("an exiled copy matched")
	}
}
```

`effects/cardflow_test.go` (append a Dig test with `ChangeNum$ All` moving every matching card of the top N). Run: `go test ./effects/ -count=1` — FAIL.

- [ ] **Step 2: Implement**

`context.go` `Defined`:

```go
	switch sa.Params["Defined"] {
	case "":
		// Forge's rule: an ability that names targets acts on them; one that
		// names none acts on its source. A sub-ability that wants its parent's
		// targets says so (Defined$ Targeted / ParentTarget), which every
		// script in the corpus does.
		if _, targeted := sa.Params["ValidTgts"]; targeted {
			return copyTargets(c.Targets)
		}
		return []state.Target{{Obj: c.Source}}
	case "Self", "Parent":
		return []state.Target{{Obj: c.Source}}
	case "TriggeredCard", "TriggeredCardLKICopy", "TriggeredNewCardLKICopy", "TriggeredSpellAbility", "TriggeredAttacker", "TriggeredSource":
		return objectsOf(c.Remembered)
	case "TriggeredDefendingPlayer", "TriggeredPlayer":
		return playersOf(c.Remembered)
	case "TriggeredCardController":
		for _, t := range c.Remembered {
			if !t.IsPlayer {
				if o := g.Obj(t.Obj); o != nil {
					return []state.Target{{Player: o.Controller, IsPlayer: true}}
				}
			}
		}
		return nil
	case "Equipped", "Enchanted", "AttachedTo":
		if o := g.Obj(c.Source); o != nil && o.AttachedTo != 0 && g.Obj(o.AttachedTo) != nil {
			return []state.Target{{Obj: o.AttachedTo}}
		}
		return nil
	// … existing You/Remembered/Targeted/ParentTarget/Opponent/Player cases unchanged
	}
```

with `objectsOf`/`playersOf` filtering `Remembered` by `IsPlayer` (fresh slices). The trailing fallback (unknown form → chosen targets) stays.

`count.go`: in `evalCountBody`, before the `Valid` handling: `if kind, ok := strings.CutPrefix(head, "CardCounters."); ok { if o := g.Obj(c.Source); o != nil { return o.Counter(kind) }; return 0 }` and `if rest, ok := strings.CutPrefix(head, "Kicked."); ok { yes, no := splitDot(rest); if o := g.Obj(c.Source); o != nil && o.CastFlags&state.FlagKicked != 0 { return yes }; return no }`; `applyCountOp` gains `case strings.HasPrefix(op, "Times."): multiply`.

`filter.go`: `SpecContext`, `MatchesSpecCtx`; `MatchesSpecFrom(g, spec, id, you, source)` = `MatchesSpecCtx(g, spec, id, SpecContext{You: you, Source: source})`. Predicates:

```go
	"kicked":  func(_ *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool { return o.CastFlags&state.FlagKicked != 0 },
	"surged":  func(_ *state.Game, o *state.Object, _ state.PlayerID, _ state.ObjID) bool { return o.CastFlags&state.FlagSurged != 0 },
	"StrictlyOther": func(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool { return o.ID != src },
	"NamedCard": func(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool {
		s := g.Obj(src)
		return s != nil && s.ChosenName != "" && o.Face() != nil && o.Face().Name == s.ChosenName
	},
	"ChosenType": func(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool {
		s := g.Obj(src)
		return s != nil && s.ChosenType != "" && hasType(o, s.ChosenType)
	},
```

`numericPred(name, g, o, resolve)`: when the RHS is not an integer, call `resolve(rhs)`; `(0, false)` → `(false, true)` (recognised shape, never matches). `MatchesSpecCtx` first: `if o.IsCopy && o.Zone != state.ZStack { return false }`. `UnknownPredicates`/`KnownPredicates` unchanged apart from the new names.

`cardflow.go` `effDig`: `changeNum := digNum; if raw := sa.Params["ChangeNum"]; raw != "" && raw != "All" { changeNum = Num(...) }` with the doc updated (the "degrades to move nothing" paragraph goes).

- [ ] **Step 3: Measure the head movement, record it, commit**

Run: `go test -count=1 ./effects/ ./rules/ -run . 2>&1 | tail -20`; then `go test ./rules/ -run 'TestHeads|TestRepoDeckGamesReplayExactly' -count=1 -v | grep -E 'chain head|seats'`.
Expected: `effects` PASS. `TestHeads` very likely FAILS: the Defined default now makes abilities without targets act on their source where they used to act on nothing (e.g. an `AB$ ChangeZone | Cost$ 3 | Origin$ Battlefield | Destination$ Hand` resolving on Self) — every such card in the 12 decks changes the acceptance games. For each of the four seat counts, find the first divergent event against the old golden by replaying the old engine's game? No — simpler and sufficient: run `go test ./rules/ -run TestRepoDeckGamesReplayExactly -v` and read which cards' abilities now resolve differently by diffing a 4-seat game transcript before and after (`git stash` is forbidden; use `git worktree add /tmp/before HEAD~1` and run `mtgsim -seats 4 -games 1 -seed 0 -v` in both trees, then diff the event counts and the winner). Name at least one card whose behaviour changed in the commit body. Update `acceptanceHeads` to the new values. `make sim` must still print `20`.

```bash
git add effects/ rules/heads_test.go
git commit -m "feat(effects): Defined defaults to Self, triggered forms, Count heads, spec context

Chain heads move: <2/4/6/8 old -> new>. Cause: Defined$ without targets now
acts on the source (Forge's rule) — e.g. <card> in the <n>-seat game <what changed>."
```

---

### Task 6: last known information for zone changes

**Files:**
- Modify: `rules/engine.go` (`emit`), `rules/trigger_match.go` (`checkTriggers(ev, lki)`, `triggerRemembered` adds the defending player), `effects/registry.go` (`Ctx.LKI *state.Object`), `rules/trigger_match_test.go` (or the existing `trigger_test.go`)
- Modify if present: `rules/clone.go` (deep-copy `Ctx.LKI`)

**Interfaces:**
- Produces: `Ctx.LKI` — for a trigger matched on a `MoveZone`/`Draw`/`PutOnStack` of object `X`, a copy of `X` as it was immediately before the move (zone, counters, controller, tapped, damage, face); nil otherwise. `triggerRemembered` for `DeclareAttackers` returns the attackers followed by `{Player: defender, IsPlayer: true}`.

- [ ] **Step 1: Write the failing test**

```go
func TestDiesTriggerSeesTheCreatureAsItWas(t *testing.T) {
	// A creature with a dies trigger and two +1/+1 counters: Move clears the
	// counters, so only last known information can say it had them.
	src := "Name:Geist\nManaCost:G G\nTypes:Creature Spirit\nPT:2/1\n" +
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ TrigNote | TriggerDescription$ When CARDNAME dies, note.\n" +
		"SVar:TrigNote:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	e, _, id := newFixtureDeck(t, 3, src)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 2})
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("%d pending triggers", len(e.pendingTriggers))
	}
	lki := e.pendingTriggers[0].Ctx.LKI
	if lki == nil || lki.ID != id || lki.Zone != state.ZBattlefield || lki.Counter("P1P1") != 2 {
		t.Fatalf("LKI %+v", lki)
	}
	if e.G.Obj(id).Counter("P1P1") != 0 {
		t.Fatal("the live object should have lost its counters on leaving")
	}
	lki.AddCounter("P1P1", 5)
	if e.G.Obj(id).Counter("P1P1") != 0 {
		t.Fatal("LKI shares a Counters array with the live object")
	}
}

func TestAttacksTriggerRemembersTheDefendingPlayer(t *testing.T) {
	ids := []state.ObjID{4, 5}
	got := triggerRemembered(events.Event{Kind: events.DeclareAttackers, Player: 2, IDs: ids}, 9)
	if len(got) != 3 || got[0].Obj != 4 || got[1].Obj != 5 || !got[2].IsPlayer || got[2].Player != 2 {
		t.Fatalf("%v", got)
	}
}
```

Run: `go test ./rules/ -run 'LKI|Dies|Remembers' -count=1` — FAIL.

- [ ] **Step 2: Implement**

`effects/registry.go` `Ctx`: add

```go
	// LKI is the object a zone-change trigger fired for, as it was just
	// before the move (CR 603.10 "look back in time"): Move resets counters,
	// tapped state and damage on the way out, so a "dies" condition such as
	// Undying's "if it had no +1/+1 counters" must read this, not the live
	// object. nil for every other trigger.
	LKI *state.Object
```

`rules/engine.go` `emit`: after the replacement check and before `events.Emit`:

```go
	var lki *state.Object
	switch ev.Kind {
	case events.MoveZone, events.Draw, events.PutOnStack:
		if o := e.G.Obj(ev.Obj); o != nil {
			cp := *o
			cp.Counters = append([]state.Counter(nil), o.Counters...)
			cp.Targets = append([]state.Target(nil), o.Targets...)
			cp.Remembered = append([]state.Target(nil), o.Remembered...)
			cp.BlockedBy = append([]state.ObjID(nil), o.BlockedBy...)
			lki = &cp
		}
	}
	stored := events.Emit(e.G, e.L, ev)
	e.checkTriggers(stored, lki)
```

`checkTriggers(ev events.Event, lki *state.Object)` sets `Ctx.LKI = lki` when `lki != nil && lki.ID == ev.Obj`. Every other caller of `checkTriggers` (grep) passes `nil`. `triggerRemembered`: for `DeclareAttackers`, append the defending player after the attacker ids. If `rules/clone.go` exists, deep-copy `pt.Ctx.LKI` (struct copy plus its slices) in `Clone`.

- [ ] **Step 3: Run, gates, commit**

Run: `go test -count=1 ./rules/ ./effects/ && go test -race -count=1 ./rules/ && go test ./rules/ -run TestHeads -count=1 && make sim | grep -c 'replay OK'`
Expected: PASS; heads unchanged (LKI is read by nothing yet; the extra Remembered player entry is consumed only by `Defined$ TriggeredDefendingPlayer`, which no deck card uses until Task 7 lets Ulamog's trigger resolve — if a head moves here, find out why); `20`.

```bash
git add rules/ effects/
git commit -m "feat(rules): last known information for zone-change triggers"
```

---

### Task 7: triggered and activated abilities take targets; TargetMin/TargetMax/TgtZone

**Files:**
- Modify: `rules/stack.go` (`askTarget`, `handleTarget`, `resolveTop`'s recheck), `rules/trigger_queue.go` (`pushTrigger` asks; drain resumes after the answer), `rules/turn.go` (`handleTarget` continuation), `rules/stack_test.go`, `rules/trigger_order_test.go` (a drain-with-target test)

**Interfaces:**
- Produces: `askTarget(p, source, sa)` honours `TargetMin$`/`TargetMax$` (ints; missing → 1/1; `TargetMin$ 0` allowed), `TgtZone$` (`Battlefield` default with players when the spec names them; `Graveyard`; `Hand` for the chooser's own hand; comma-separated lists), and offers options in `AliveFrom(0)` × zone-slice order; `handleTarget` records N targets with the `TargetsChosen` shapes (first replace, then append); a spell or ability with `TargetMin$ 0` and no targets resolves (N2); a trigger with `ValidTgts$` asks its controller right after `TriggerPush`, and the drain resumes after the answer through the same continuation `handleTriggerOrder` uses.

- [ ] **Step 1: Write the failing tests**

```go
func TestASpellCastTriggerAsksForTwoTargetsAndExilesBoth(t *testing.T) {
	// Ulamog's shape: "When you cast this spell, exile two target permanents."
	src := "Name:Titan\nManaCost:1\nTypes:Creature Eldrazi\nPT:10/10\n" +
		"T:Mode$ SpellCast | ValidCard$ Card.Self | Execute$ TrigChange | TriggerDescription$ When you cast this spell, exile two target permanents.\n" +
		"SVar:TrigChange:DB$ ChangeZone | ValidTgts$ Permanent | TargetMin$ 2 | TargetMax$ 2 | Origin$ Battlefield | Destination$ Exile\nOracle:x\n"
	e, _, titan := newFixtureDeck(t, 11, src)
	lands := putLands(t, e, 1, 3) // helper: three Mountains onto seat 1's battlefield (see replacement_updated_test.go's helpers)
	castFirst(t, e, "cast")       // stack_test.go helper: take the first "cast" option
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 2 || d.Max != 2 || len(d.Options) != 3 {
		t.Fatalf("decision %+v", d)
	}
	submitChoices(t, e, 0, 1) // view_test.go has this helper shape; add one here if absent
	// The trigger is on the stack above the spell with two targets recorded.
	top := e.G.Obj(e.G.Stack[len(e.G.Stack)-1])
	if top.Ability == nil || len(top.Targets) != 2 {
		t.Fatalf("top of stack %+v", top)
	}
	passUntilStackEmpty(t, e, 50)
	exiled := 0
	for _, id := range lands {
		if e.G.Obj(id).Zone == state.ZExile {
			exiled++
		}
	}
	if exiled != 2 || e.G.Obj(titan).Zone != state.ZBattlefield {
		t.Fatalf("exiled %d, titan in %s", exiled, e.G.Obj(titan).Zone)
	}
}

func TestTargetMinZeroResolvesWithNoTargets(t *testing.T) {
	src := "Name:Optional\nManaCost:R\nTypes:Instant\n" +
		"A:SP$ DealDamage | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ 1 | NumDmg$ 2 | SubAbility$ DBDraw | SpellDescription$ x\n" +
		"SVar:DBDraw:DB$ Draw | Defined$ You | NumCards$ 1\nOracle:x\n"
	e, _, _ := newFixtureDeck(t, 12, src)
	hand := len(e.G.Zone(state.ZHand, 0))
	castFirst(t, e, "cast") // no creatures anywhere: with Min 0 the spell goes on the stack untargeted
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		t.Fatal("asked for a target with none available and TargetMin 0")
	}
	passUntilStackEmpty(t, e, 50)
	if len(e.G.Zone(state.ZHand, 0)) != hand { // cast -1, draw +1
		t.Fatal("the untargeted spell did not resolve its rider")
	}
}

func TestTgtZoneGraveyardOffersGraveyardCards(t *testing.T) {
	// Snapcaster's ETB shape, reduced: target instant card in your graveyard.
	src := "Name:Mage\nManaCost:1 U\nTypes:Creature Human Wizard\nPT:2/1\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigPump | TriggerDescription$ x\n" +
		"SVar:TrigPump:DB$ Pump | ValidTgts$ Instant.YouCtrl,Sorcery.YouCtrl | TgtZone$ Graveyard | KW$ Flashback | PumpZone$ Graveyard\nOracle:x\n"
	e, _, mage := newFixtureDeck(t, 13, src)
	bolt := addToGraveyard(t, e, 0, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: mage, From: state.ZHand, To: state.ZBattlefield})
	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || len(d.Options) != 1 || d.Options[0].Obj != bolt {
		t.Fatalf("decision %+v", d)
	}
}
```

Write `putLands`/`addToGraveyard` beside the fixture helpers if absent (they are five-line `AddObject` + `SetZone` + `Zone =` builders). Run: `go test ./rules/ -run 'TwoTargets|TargetMinZero|TgtZone' -count=1` — FAIL.

- [ ] **Step 2: Implement**

`askTarget(p, source, sa)`: read `min, max := targetBounds(sa)` (defaults 1,1; parse ints; clamp `max >= min`, `max >= 1`); zones from `TgtZone$` (default: battlefield, plus players when `targetsPlayers(spec)`); for `Graveyard`/`Hand`/`Exile` walk `AliveFrom(0)`'s zone slices and match with `MatchesSpecFrom(e.G, spec, oid, p, source)`; `Hand` is the chooser's own hand only. If `len(options) < min`: `min == 0` → return without asking (the caller proceeds; for a spell that means the cast completes untargeted); otherwise the existing fizzle path. Decision `Min: min, Max: max`. Store the ability's `sa` on the decision? `handleTarget` needs no SA: it emits `TargetsChosen` for `d.Source` from the chosen options: first chosen → shape 0/1 (replace), each further chosen → shape 2/3 (append).

`resolveTop`: both rechecks become `if spec := …["ValidTgts"]; spec != "" && !(targetMin(sa) == 0 && len(targets) == 0)`.

`pushTrigger`: after the `TriggerPush` emit, `id := e.G.Stack[len(e.G.Stack)-1]`; `if pt.SA.Params["ValidTgts"] != "" { e.askTarget(pt.Controller, id, pt.SA); e.drainAwaitsTarget = e.Pending() != nil }`; `putTriggersOnStack` returns `true` when an ask happened (it already returns true for its own asks — after `pushTrigger`, check `e.Pending() != nil` and return true). `handleTarget` ends with: `if e.drainAwaitsTarget { e.drainAwaitsTarget = false; e.resumeTriggerDrain(); return }` before its Priority emit — a trigger's target answer does not hand out priority; the drain's own tail does. (Read `resumeTriggerDrain`'s doc in `trigger_queue.go` first; mirror exactly what `handleTriggerOptional` does after it records the answer.) Add `drainAwaitsTarget bool` to `Engine` (copied by `Clone` if present).

- [ ] **Step 3: Run, gates, commit**

Run: `go test -count=1 ./rules/ && go test -race -count=1 ./rules/ && go test ./rules/ -run TestHeads -count=1 -v | grep 'chain head' ; make sim | grep -c 'replay OK'`
Expected: PASS; heads move if any deck card's trigger declares `ValidTgts$` (Ulamog's and World Breaker's cast triggers do — in the 6- and 8-seat games at least): record old → new and name the cards. `20`.

```bash
git add rules/
git commit -m "feat(rules): triggered abilities take targets; TargetMin/TargetMax/TgtZone

Chain heads: <old -> new per seat count>. Cause: Ulamog's and World
Breaker's cast triggers now choose and exile their targets."
```

---

### Task 8: the `choose` decision and the bot policy

**Files:**
- Modify: `decision/decision.go`, `decision/decision_test.go`, `seat/bot.go`, `seat/bot_test.go`, `rules/testbot_test.go`, `rules/turn.go` (`handle`), `rules/engine.go` (`chooseFor`), `rules/choose_test.go` (new)

**Interfaces:**
- Produces: `decision.KChoose Kind = "choose"`; option kinds `"x"`, `"exile"`, `"sacrifice"`, `"name"`, `"type"`, `"number"`, `"yes"`, `"no"`; in `rules`: `type chooseFor uint8` (`chooseNone`, `chooseCast`, `chooseMiracle`, `chooseETB`) stored on `Engine` as `e.choosing chooseFor` — **data, not a closure**, so `Clone` copies it — and `handleChoose(d, in)` dispatching on it (Task 9 fills the cast case; Tasks 12 and 18 the others). Bot policy (both bots, verbatim mirror): `x` → the highest option; `exile`/`sacrifice` → the first `Max` options; `yes`/`no` → `yes`; `name`/`type`/`number` → the first option.

- [ ] **Step 1: Write the failing tests**

`decision/decision_test.go` (append):

```go
func TestChooseValidatesLikeAnyDecision(t *testing.T) {
	d := &Decision{Seq: 1, Player: 0, Kind: KChoose, Min: 0, Max: 2, Options: []Option{{Index: 0, Kind: "exile"}, {Index: 1, Kind: "exile"}, {Index: 2, Kind: "exile"}}}
	if err := d.Validate(Intent{Seq: 1, Player: 0, Choices: []int{}}); err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(Intent{Seq: 1, Player: 0, Choices: []int{0, 1, 2}}); err == nil {
		t.Fatal("three of max two accepted")
	}
	if KChoose != "choose" {
		t.Fatal("wire name")
	}
}
```

`seat/bot_test.go` (append; mirror in `rules/testbot_test.go` as `TestTestBotChoosePolicy` calling `answer`):

```go
func TestBotChoosePolicy(t *testing.T) {
	b := NewBot(1)
	choose := func(kind string, n, min, max int) decision.Intent {
		d := decision.Decision{Player: 0, Kind: decision.KChoose, Min: min, Max: max}
		for i := 0; i < n; i++ {
			d.Options = append(d.Options, decision.Option{Index: i, Kind: kind, Label: kind})
		}
		if kind == "yes" {
			d.Options = []decision.Option{{Index: 0, Kind: "yes"}, {Index: 1, Kind: "no"}}
		}
		in, _ := b.Decide(context.Background(), view.View{}, d)
		return in
	}
	if got := choose("x", 4, 1, 1).Choices; len(got) != 1 || got[0] != 3 {
		t.Fatalf("x: %v, want the highest", got)
	}
	if got := choose("exile", 5, 0, 3).Choices; len(got) != 3 || got[0] != 0 || got[2] != 2 {
		t.Fatalf("exile: %v, want the first three", got)
	}
	if got := choose("sacrifice", 2, 1, 1).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("sacrifice: %v", got)
	}
	if got := choose("yes", 2, 1, 1).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("yes/no: %v, want yes", got)
	}
	for _, k := range []string{"name", "type", "number"} {
		if got := choose(k, 3, 1, 1).Choices; len(got) != 1 || got[0] != 0 {
			t.Fatalf("%s: %v, want the first", k, got)
		}
	}
}
```

`rules/choose_test.go`:

```go
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
)

func TestHandleRoutesChooseToTheAsker(t *testing.T) {
	names, decks := testutil.SampleDecks(t, 2)
	e := New(Config{Seed: 1, Names: names, Decks: decks})
	e.Advance()
	// Ask a bare choose with no asker registered: the engine must not panic
	// and must not strand the match — it records a Note and re-grants priority.
	d := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 1, Options: []decision.Option{{Index: 0, Kind: "number", Label: "0"}}}
	e.pending = nil
	e.ask(d)
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{0}}); err != nil {
		t.Fatal(err)
	}
	if e.Pending() == nil || e.Pending().Kind != decision.KPriority {
		t.Fatalf("after a stray choose the engine should be back at priority, got %+v", e.Pending())
	}
}
```

Run: `go test ./decision/ ./seat/ ./rules/ -run 'Choose' -count=1` — FAIL.

- [ ] **Step 2: Implement**

`decision/decision.go`: after `KTriggerOptional`:

```go
	// KChoose is one list-pick: choose between Min and Max of the offered
	// options. Every option in one decision shares a Kind that says what is
	// being chosen — "x" (a value for {X}; options ascend), "exile" (cards
	// to exile for Delve), "sacrifice" (a permanent to sacrifice as a cost),
	// "name"/"type"/"number" (an "as this enters" choice), "yes"/"no" (a
	// may-cast such as Miracle). The wire shape is the same as every other
	// decision; only the vocabulary of Option.Kind is new.
	KChoose Kind = "choose"
```

Bots (`seat/bot.go` `botDecide` and `rules/testbot_test.go` `answer`, identical text):

```go
	case decision.KChoose:
		if len(d.Options) == 0 {
			break
		}
		switch d.Options[0].Kind {
		case "x":
			in.Choices = []int{d.Options[len(d.Options)-1].Index} // the most it can pay for
		case "exile", "sacrifice":
			for i := 0; i < len(d.Options) && i < d.Max; i++ {
				in.Choices = append(in.Choices, d.Options[i].Index)
			}
		default: // yes/no (yes is first), name, type, number: the first offer
			in.Choices = []int{d.Options[0].Index}
		}
		return clamp(d, in)
```

`rules/engine.go`: `choosing chooseFor` on `Engine`; `rules/turn.go` `handle`: `case decision.KChoose: e.handleChoose(d, in)`:

```go
// handleChoose routes a choose answer to whichever flow asked it. The flows
// are data on the engine (never closures, so Clone copies them): the cast
// flow (Task 9), a miracle offer (Task 18), an "as this enters" choice
// (Task 12). A choose nobody is waiting for — only reachable from a
// hand-built decision — is dropped with a Note and priority resumes.
func (e *Engine) handleChoose(d *decision.Decision, in decision.Intent) {
	chosen := d.Chosen(in)
	switch e.choosing {
	// Tasks 9, 12 and 18 add their cases here.
	default:
		e.emit(events.Event{Kind: events.Note, Player: in.Player, Text: "choose answered with no flow waiting"})
		e.emit(events.Event{Kind: events.Priority, Player: in.Player, Amount: 0})
	}
	_ = chosen
}
```

(`chooseFor` constants declared in `rules/cast.go` in Task 9; declare `type chooseFor uint8` and `chooseNone` here now.)

- [ ] **Step 3: Run, gates, commit**

Run: `go test -count=1 ./decision/ ./seat/ ./rules/ && go test ./rules/ -run TestHeads -count=1 && make sim | grep -c 'replay OK'`
Expected: PASS; heads unchanged; `20`.

```bash
git add decision/ seat/ rules/
git commit -m "feat(decision): the choose decision kind and its bot policy"
```

---

## Phase 2 — casting and activating

### Task 9: the cost grammar and the cast flow — X, Kicker, Surge, Flashback, Delve

**Files:**
- Modify: `rules/mana.go` (Cost grammar), `rules/mana_test.go`, `rules/legal.go` (cast options), `rules/stack.go` (`castSpell` → `beginCast`; flashback exile), `rules/turn.go` (`handleChoose` cast case), `rules/engine.go` (`cast *pendingCast`), `cards/face.go` (`KeywordParam`), `cards/face_test.go`, `rules/acceptance_test.go` (seven entries), `rules/heads_test.go`
- Create: `rules/cast.go`, `rules/cast_test.go`

**Interfaces:**
- Produces:

```go
// cards
func (f *Face) KeywordParam(head string) (string, bool)  // "Kicker:B" -> "B", true; "Flash" -> "", true; absent -> "", false
// rules/mana.go
type CostPart struct { N int32; Spec string }
type Cost struct { Colored state.Mana; Generic int32; X int; Tap bool; Sac []CostPart; SubCounter []CostPart }
func ParseCost(s string) Cost              // "X X", "T", "Sac<1/Creature>", "SubCounter<2/P1P1>", "2 C Sac<1/Land>", "{2}{U}{U}"
func (c Cost) WithX(x int32) Cost          // folds x into Generic per X symbol; X becomes 0
func (c Cost) Plus(d Cost) Cost            // mana sum (for Kicker)
func (c Cost) HasNonMana() bool
// rules/cast.go
type pendingCast struct {
	player state.PlayerID; card state.ObjID; from state.Zone; mode string; ability int // -1 for a spell (Task 10 uses >= 0)
	cost Cost; x int32; xDone bool; delve []state.ObjID; delveDone bool; sacs []state.ObjID; sacPart int
}
const ( chooseNone chooseFor = iota; chooseCast; chooseETB; chooseMiracle )
func (e *Engine) beginCast(p state.PlayerID, opt decision.Option)   // from handlePriority "cast"
func (e *Engine) continueCast()                                    // asks the next KChoose or commits
func (e *Engine) spellsCastThisTurn(p state.PlayerID) int          // from the log since the last TurnChange
func (e *Engine) castable(p state.PlayerID, id state.ObjID, cost Cost) bool // mana payable allowing Delve; non-mana parts satisfiable
```

`Option.Mode string` (json `-`) on `decision.Option`: `""`, `"kicked"`, `"surged"`, `"flashback"`, `"miracle"`. Registered: `kw:Kicker`, `kw:Surge`, `kw:Flashback`, `kw:Delve`. Ratchet: 24.

Flow: `handlePriority` "cast" emits its Priority event as today, then `beginCast`. `beginCast` builds the `pendingCast` (base or alternative cost as before; `kicked` adds `KeywordParam("Kicker")`; `surged` replaces with `KeywordParam("Surge")`; `flashback` replaces with `KeywordParam("Flashback")` or, for Flashback granted by a continuous effect (no printed parameter), the card's own mana cost; `miracle` — Task 18). `continueCast` runs the stages in order, each either asking a `KChoose` (setting `e.choosing = chooseCast`) and returning, or moving on: (1) X: options `x` for 0…max where max is the largest value with `cost.WithX(x)` payable after the best Delve; (2) Delve, when the card has it and the graveyard is non-empty and generic > 0: options `exile` over the caster's graveyard (zone order), Min 0, Max = min(len, generic); (3) each `Sac` part: options `sacrifice` over the caster's permanents matching the spec (battlefield order), Min = Max = N; (4) commit: pay mana (`WithX`, minus one generic per exiled card), exile the delved cards (`MoveZone` graveyard→exile), sacrifice (`MoveZone` battlefield→graveyard, Text "sacrificed"), emit `CastInfo{Obj, Amount: x, Counter: FlagsString(flags)}`, `PutOnStack` from `from`, then `askTarget` if the SA declares targets. `handleChoose`'s `chooseCast` case records the answer into the flow and calls `continueCast`. A payment that fails at commit (a pool that changed under a hand-built intent) aborts with a `Note` and leaves the card where it was. A spell cast with the flashback flag leaves the stack to exile instead of the graveyard at every exit in `resolveTop`.

- [ ] **Step 1: Write the failing tests**

`cards/face_test.go` (append):

```go
func TestKeywordParam(t *testing.T) {
	c, _ := ParseBytes("k.txt", []byte("Name:K\nTypes:Creature\nK:Kicker:B\nK:Flash\nK:Flashback:Sac<1/Creature>\nK:Protection from blue\nOracle:x\n"))
	f := c.Faces[0]
	for head, want := range map[string]string{"Kicker": "B", "Flashback": "Sac<1/Creature>", "Flash": "", "Protection from blue": ""} {
		if got, ok := f.KeywordParam(head); !ok || got != want {
			t.Errorf("%s: %q %v", head, got, ok)
		}
	}
	if _, ok := f.KeywordParam("Delve"); ok {
		t.Error("absent keyword reported present")
	}
}
```

`rules/mana_test.go` (append; fix the existing `X: true` literals to `X: 1`):

```go
func TestParseCostNonManaParts(t *testing.T) {
	c := ParseCost("2 C Sac<1/Land>")
	if c.Generic != 2 || c.Colored[state.MC] != 1 || len(c.Sac) != 1 || c.Sac[0] != (CostPart{1, "Land"}) {
		t.Fatalf("%+v", c)
	}
	c = ParseCost("SubCounter<2/P1P1>")
	if c.CMC() != 0 || len(c.SubCounter) != 1 || c.SubCounter[0] != (CostPart{2, "P1P1"}) || !c.HasNonMana() {
		t.Fatalf("%+v", c)
	}
	c = ParseCost("T")
	if !c.Tap || c.CMC() != 0 {
		t.Fatalf("%+v", c)
	}
	c = ParseCost("X X W W W")
	if c.X != 2 || c.Colored[state.MW] != 3 || c.CMC() != 3 {
		t.Fatalf("%+v", c)
	}
	if w := c.WithX(2); w.X != 0 || w.Generic != 4 || w.CMC() != 7 {
		t.Fatalf("WithX %+v", w)
	}
	if p := ParseCost("R").Plus(ParseCost("R")); p.Colored[state.MR] != 2 {
		t.Fatalf("Plus %+v", p)
	}
	if ParseCost("3 U").HasNonMana() {
		t.Fatal("mana-only cost reports non-mana parts")
	}
}
```

`rules/cast_test.go`:

```go
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// castOptions lists the "cast" options for the first card in seat 0's hand.
func castOptions(t *testing.T, e *Engine) []decision.Option {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority: %+v", d)
	}
	var out []decision.Option
	for _, o := range d.Options {
		if o.Kind == "cast" {
			out = append(out, o)
		}
	}
	return out
}

func TestXIsChosenAndRecorded(t *testing.T) {
	e, cfg, id := newFixtureDeck(t, 21, "Name:Endless\nManaCost:X\nTypes:Creature Eldrazi\nPT:0/0\nOracle:x\n")
	addMana(t, e, 0, "GGG") // helper: three ManaAdd events into seat 0's pool
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "x" || len(d.Options) != 4 || d.Options[3].Label != "X = 3" {
		t.Fatalf("X decision %+v", d)
	}
	submitChoices(t, e, 2)
	o := e.G.Obj(id)
	if o.Zone != state.ZStack || o.X != 2 || e.G.Players[0].Pool.Total() != 1 {
		t.Fatalf("after X=2: zone %s X %d pool %d", o.Zone, o.X, e.G.Players[0].Pool.Total())
	}
	if !hasEvent(e, events.CastInfo, id) {
		t.Fatal("no CastInfo event")
	}
	replayCheck(t, e, cfg)
}

func TestKickerOffersASecondCastOptionAndFlagsTheSpell(t *testing.T) {
	src := "Name:Whacker\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nK:Kicker:R\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self+kicked | Execute$ TrigPump | TriggerDescription$ if kicked\n" +
		"SVar:TrigPump:DB$ PumpAll | ValidCards$ Creature.YouCtrl | NumAtt$ +1\nOracle:x\n"
	e, _, id := newFixtureDeck(t, 22, src)
	addMana(t, e, 0, "R")
	if opts := castOptions(t, e); len(opts) != 1 || opts[0].Mode != "" {
		t.Fatalf("one mana: %+v", opts)
	}
	addMana(t, e, 0, "R")
	opts := castOptions(t, e)
	if len(opts) != 2 || opts[1].Mode != "kicked" || opts[1].Label != "Cast Whacker (kicked)" {
		t.Fatalf("two mana: %+v", opts)
	}
	submitChoices(t, e, opts[1].Index)
	if o := e.G.Obj(id); o.CastFlags&state.FlagKicked == 0 || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("kicked cast: %+v pool %d", o, e.G.Players[0].Pool.Total())
	}
	passUntilStackEmpty(t, e, 20)
	if len(e.pendingTriggers) == 0 && !hasEvent(e, events.TriggerPush, id) {
		t.Fatal("the 'if kicked' trigger did not fire")
	}
	// Not kicked: the trigger must not fire.
	e2, _, id2 := newFixtureDeck(t, 23, src)
	addMana(t, e2, 0, "RR")
	submitChoices(t, e2, castOptions(t, e2)[0].Index)
	passUntilStackEmpty(t, e2, 20)
	if hasEvent(e2, events.TriggerPush, id2) {
		t.Fatal("the 'if kicked' trigger fired on an unkicked cast")
	}
}

func TestSurgeNeedsAnotherSpellThisTurn(t *testing.T) {
	src := "Name:Reckless\nManaCost:2 R\nTypes:Creature Goblin\nPT:2/1\nK:Surge:1 R\nK:Haste\nOracle:x\n"
	e, _, _ := newFixtureDeck(t, 24, src)
	bolt := addToHand(t, e, 0, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	addMana(t, e, 0, "RRR")
	for _, o := range castOptions(t, e) {
		if o.Mode == "surged" {
			t.Fatal("surge offered before any spell this turn")
		}
	}
	castObj(t, e, bolt) // helper: choose the cast option for this object, answer its target with the first option
	if e.spellsCastThisTurn(0) != 1 {
		t.Fatalf("spells cast this turn = %d", e.spellsCastThisTurn(0))
	}
	var surged *decision.Option
	for _, o := range castOptions(t, e) {
		if o.Mode == "surged" {
			surged = &o
		}
	}
	if surged == nil {
		t.Fatal("surge not offered after a spell this turn")
	}
	pool := e.G.Players[0].Pool.Total()
	submitChoices(t, e, surged.Index)
	if e.G.Players[0].Pool.Total() != pool-2 {
		t.Fatal("surge cost {1}{R} not what was paid")
	}
}

func TestFlashbackCastsFromTheGraveyardPaysASacrificeAndExiles(t *testing.T) {
	src := "Name:Therapy\nManaCost:B\nTypes:Sorcery\nK:Flashback:Sac<1/Creature>\n" +
		"A:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	e, cfg, therapy := newFixtureDeck(t, 25, src)
	bear := putCreature(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: therapy, From: state.ZHand, To: state.ZGraveyard})
	e.Advance()
	var fb *decision.Option
	for _, o := range castOptions(t, e) {
		if o.Mode == "flashback" && o.Obj == therapy {
			fb = &o
		}
	}
	if fb == nil {
		t.Fatal("flashback not offered from the graveyard")
	}
	submitChoices(t, e, fb.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "sacrifice" || d.Options[0].Obj != bear {
		t.Fatalf("sacrifice choice %+v", d)
	}
	submitChoices(t, e, 0)
	if e.G.Obj(bear).Zone != state.ZGraveyard || e.G.Obj(therapy).Zone != state.ZStack || e.G.Obj(therapy).CastFlags&state.FlagFlashback == 0 {
		t.Fatalf("after paying: bear %s therapy %s", e.G.Obj(bear).Zone, e.G.Obj(therapy).Zone)
	}
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(therapy).Zone != state.ZExile {
		t.Fatalf("flashback spell went to %s, want exile", e.G.Obj(therapy).Zone)
	}
	replayCheck(t, e, cfg)
}

func TestDelveExilesFromTheGraveyardToPayGeneric(t *testing.T) {
	src := "Name:Angler\nManaCost:6 B\nTypes:Creature Zombie Fish\nPT:5/5\nK:Delve\nOracle:x\n"
	e, cfg, angler := newFixtureDeck(t, 26, src)
	var gy []state.ObjID
	for i := 0; i < 4; i++ {
		gy = append(gy, addToGraveyard(t, e, 0, "Name:Junk\nManaCost:1\nTypes:Sorcery\nOracle:x\n"))
	}
	addMana(t, e, 0, "BGG")
	opts := castOptions(t, e)
	if len(opts) != 1 {
		t.Fatalf("with delve, {6}{B} is castable off B+2 and four graveyard cards: %+v", opts)
	}
	submitChoices(t, e, opts[0].Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "exile" || d.Min != 0 || d.Max != 4 {
		t.Fatalf("delve choice %+v", d)
	}
	submitChoices(t, e, 0, 1, 2, 3)
	if e.G.Obj(angler).Zone != state.ZStack || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("angler %s pool %d", e.G.Obj(angler).Zone, e.G.Players[0].Pool.Total())
	}
	for _, id := range gy {
		if e.G.Obj(id).Zone != state.ZExile {
			t.Fatal("delved card not exiled")
		}
	}
	replayCheck(t, e, cfg)
	// Without delve fodder the same hand is not castable.
	e2, _, _ := newFixtureDeck(t, 27, src)
	addMana(t, e2, 0, "BGG")
	if len(castOptions(t, e2)) != 0 {
		t.Fatal("castable without enough mana or graveyard")
	}
}
```

Write the small helpers in this file (`addMana` emits `ManaAdd` per symbol; `addToHand`/`addToGraveyard`/`putCreature` add a parsed card into the zone; `castObj` picks the cast option with `Obj == id` then answers a target decision with option 0 if one appears; `hasEvent(e, kind, obj)` scans `e.L.Events`; `hasNote(e, substr)` scans for a Note whose Text contains substr). Every flow test ends with `replayCheck`:

```go
// replayCheck rebuilds the game from the log alone and compares it with the
// live game — the same fidelity check trigger_test.go's replayFromLog gives.
func replayCheck(t *testing.T, e *Engine, cfg Config) {
	t.Helper()
	if diff := diffGames(e.G, replayFromLog(t, cfg, e.L.Events)); diff != "" {
		t.Fatalf("log-only replay differs:\n%s", diff)
	}
}
```

Run: `go test ./rules/ -run 'Cast|Kicker|Surge|Flashback|Delve|ParseCostNonMana' -count=1; go test ./cards/ -run KeywordParam -count=1` — FAIL.

- [ ] **Step 2: Implement**

`cards/face.go`:

```go
// KeywordParam returns the text after the colon of a parameterised keyword
// ("Kicker:B" -> "B"; "Equip:2" -> "2") and reports whether the keyword is
// printed at all ("Flash" -> "", true).
func (f *Face) KeywordParam(head string) (string, bool) {
	for _, k := range f.Keywords {
		if strings.EqualFold(KeywordHead(k), head) {
			if i := strings.IndexByte(k, ':'); i >= 0 {
				return strings.TrimSpace(k[i+1:]), true
			}
			return "", true
		}
	}
	return "", false
}
```

`rules/mana.go`: the `Cost` shape above; `ParseCost` recognises, per whitespace-separated token: `T` → `Tap`; `Sac<N/Spec>` and `SubCounter<N/Kind>` (regexp `^(Sac|SubCounter)<(\d+)/([^>]+)>$`; a malformed one is one generic, the same degradation as an unknown mana token); `X` increments `X`; the rest as today. `WithX`, `Plus`, `HasNonMana` as documented. `Pay`/`CanPay` unchanged (mana only). `CMC` unchanged.

`decision.Option` gains `Mode string \`json:"-"\`` with the comment "Mode is server-side only: how a "cast" option pays — "" the card's own cost, "kicked", "surged", "flashback", "miracle"."

`rules/cast.go`: the `pendingCast` type, `chooseFor` constants, `beginCast`, `continueCast`, `commitCast`, `spellsCastThisTurn` (walk `e.L.Events` backwards to the last `TurnChange`, counting `PutOnStack` with `Player == p`), `castable`, `kickerCost`/`surgeCost`/`flashbackCost` helpers, and `init()` registering the four keywords. Keep every stage a method returning `bool` ("asked"), and `continueCast` a loop over them. Sketch of the committing stage:

```go
func (e *Engine) commitCast() {
	pc := e.cast
	e.cast, e.choosing = nil, chooseNone
	o := e.G.Obj(pc.card)
	if o == nil || o.Zone != pc.from {
		e.emit(events.Event{Kind: events.Note, Player: pc.player, Text: "cast aborted: the card moved"})
		return
	}
	mana := pc.cost.WithX(pc.x)
	mana.Generic -= int32(len(pc.delve))
	if mana.Generic < 0 {
		mana.Generic = 0
	}
	if !e.payMana(pc.player, mana) {
		e.emit(events.Event{Kind: events.Note, Player: pc.player, Text: "cast aborted: cost no longer payable"})
		return
	}
	for _, id := range pc.delve {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZExile, Text: "delved"})
	}
	for _, id := range pc.sacs {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
	}
	if pc.x != 0 || pc.mode != "" {
		e.emit(events.Event{Kind: events.CastInfo, Obj: pc.card, Amount: pc.x, Counter: modeFlags(pc.mode)})
	}
	e.emit(events.Event{Kind: events.PutOnStack, Obj: pc.card, Player: pc.player, From: pc.from, To: state.ZStack, Text: o.Face().Name})
	if sa := o.Face().SpellAbility(); sa != nil && sa.Params["ValidTgts"] != "" {
		e.askTarget(pc.player, pc.card, sa)
	}
}
```

`CastInfo` is emitted only when it carries information (X or a mode), so a plain cast produces exactly the events it produced before this task and the chain heads of games without X/modes are unchanged. `legal.go`: for each hand card, after the base option: `if kc, ok := f.KeywordParam("Kicker"); ok && e.castable(p, id, base.Plus(ParseCost(kc)))` → option `Mode: "kicked"`, label `Cast <name> (kicked)`; `Surge` → when `spellsCastThisTurn(p) > 0` and the surge cost is castable → `Mode: "surged"`; then a graveyard walk: cards with `e.HasKeyword(id, "Flashback")` (derived — Snapcaster's grant counts), instant-speed timing as for hand cards, `castable(p, id, flashbackCost(id))` → `Mode: "flashback"`. `castable`: mana payable when generic is reduced by `min(generic, len(graveyard))` if the card has Delve; every `Sac` part has at least N matching permanents; every `SubCounter` part is satisfied by the source; `Tap` requires the source untapped. `stack.go`: `castSpell` becomes `beginCast`; the four graveyard exits in `resolveTop` (and `ensureLeftTheStack`'s `to`) go through `spellRestZone(o)` → exile when `o.CastFlags&state.FlagFlashback != 0`. `turn.go` `handleChoose`: `case chooseCast: e.castAnswer(chosen); e.continueCast()`.

- [ ] **Step 3: Run, ratchet, heads, commit**

Run: `go test -count=1 ./cards/ ./decision/ ./rules/ && go test -race -count=1 ./rules/`. Delete the seven ratchet entries (Gatekeeper of Malakir, Goblin Bushwhacker, Vines of Vastwood, Reckless Bushwhacker, Cabal Therapy, Gurmag Angler, Tombstalker); `go test ./rules/ -run 'TestEveryRepoDeck' -v -count=1 | grep ratchet` → `24 of 136`. `TestHeads`: heads move (the bots now kick Bushwhackers and cast Delve creatures) — record and name. `make sim` 20/20. `make report`.

```bash
git add cards/ decision/ rules/
git commit -m "feat(rules): the cast flow — X, Kicker, Surge, Flashback, Delve; cost grammar

Chain heads: <old -> new>. Cause: Goblin Bushwhacker is now cast kicked,
Gurmag Angler and Tombstalker are cast by delving. Ratchet 31 -> 24.
make report: <playable>."
```

---

### Task 10: activated abilities as options

**Files:**
- Modify: `rules/legal.go`, `rules/statics.go` (`abilityRestricted(id, ability *cards.SA)` with `ValidSA$`), `rules/cast.go` (the flow serves activations too: `ability >= 0`), `rules/turn.go`, `seat/bot.go` + `rules/testbot_test.go` (the `"ability"` option kind), `rules/acceptance_test.go` (no table change)
- Create: `rules/activate.go`, `rules/activate_test.go`

**Interfaces:**
- Produces: `Option.Ability int` (json `-`; index into `Face.Abilities`), option kind `"ability"` with label `"<card>: <SpellDescription$>"`; `func (e *Engine) beginActivation(p, opt)`; costs paid through the Task 9 flow (mana, `Tap`, `SubCounter`, `Sac` with its `KChoose`); `AbilityPush` then `askTarget`. Rules honoured: `SorcerySpeed$ True`; `ActivationZone$ Graveyard`; a creature's `T` cost needs no summoning sickness (CR 302.6) unless Haste; `CantBeActivated` with `ValidSA$ Activated.!ManaAbility` spares mana abilities; the existing `"activate"` (tap for mana) option is unchanged. Bots: in a main phase, after `cast` fails to find anything, take the first `"ability"` option.

- [ ] **Step 1: Write the failing tests**

`rules/activate_test.go`:

```go
func TestManaCostAbilityGoesOnTheStackAndResolves(t *testing.T) {
	src := "Name:Sailor\nManaCost:U\nTypes:Creature Spirit\nPT:1/1\nA:AB$ Draw | Cost$ 3 U | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 31, src)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	addMana(t, e, 0, "UUUU")
	e.Advance()
	opt := abilityOption(t, e, id, 0) // helper: the "ability" option for (obj, index), fatal if absent
	if opt.Label != "Sailor: Draw a card." {
		t.Fatalf("label %q", opt.Label)
	}
	hand := len(e.G.Zone(state.ZHand, 0))
	submitChoices(t, e, opt.Index)
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Ability == nil || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("stack %v pool %d", e.G.Stack, e.G.Players[0].Pool.Total())
	}
	passUntilStackEmpty(t, e, 20)
	if len(e.G.Zone(state.ZHand, 0)) != hand+1 {
		t.Fatal("the ability did not draw")
	}
	replayCheck(t, e, cfg)
}

func TestRemoveCounterCostAndTargetedAbility(t *testing.T) {
	src := "Name:Ballista\nManaCost:X X\nTypes:Artifact Creature Construct\nPT:0/0\n" +
		"A:AB$ PutCounter | Cost$ 4 | CounterType$ P1P1 | CounterNum$ 1 | SpellDescription$ Put a +1/+1 counter on CARDNAME.\n" +
		"A:AB$ DealDamage | Cost$ SubCounter<1/P1P1> | ValidTgts$ Any | NumDmg$ 1 | SpellDescription$ It deals 1 damage to any target.\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 32, src)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 2})
	e.Advance()
	if _, ok := findAbilityOption(e, id, 0); ok {
		t.Fatal("{4} ability offered with an empty pool")
	}
	opt := abilityOption(t, e, id, 1)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target decision: %+v", d)
	}
	life := e.G.Players[1].Life
	submitChoices(t, e, indexOfPlayerOption(d, 1))
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(id).Counter("P1P1") != 1 || e.G.Players[1].Life != life-1 {
		t.Fatalf("counters %d life %d", e.G.Obj(id).Counter("P1P1"), e.G.Players[1].Life)
	}
	replayCheck(t, e, cfg)
}

func TestGraveyardActivationWithSacrificeCost(t *testing.T) {
	src := "Name:Breaker\nManaCost:6 G\nTypes:Creature Eldrazi\nPT:5/7\nK:Devoid\n" +
		"A:AB$ ChangeZone | Cost$ 2 C Sac<1/Land> | Origin$ Graveyard | Destination$ Hand | ActivationZone$ Graveyard | SpellDescription$ Return CARDNAME from your graveyard to your hand.\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 33, src)
	land := putLands(t, e, 0, 1)[0]
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
	addMana(t, e, 0, "CGG")
	e.Advance()
	opt := abilityOption(t, e, id, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "sacrifice" || d.Options[0].Obj != land {
		t.Fatalf("sacrifice choice %+v", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(land).Zone != state.ZGraveyard || e.G.Obj(id).Zone != state.ZHand || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("land %s breaker %s pool %d", e.G.Obj(land).Zone, e.G.Obj(id).Zone, e.G.Players[0].Pool.Total())
	}
	replayCheck(t, e, cfg)
}

func TestSorcerySpeedAndSummoningSicknessGates(t *testing.T) {
	src := "Name:Gear\nManaCost:1\nTypes:Artifact\n" +
		"A:AB$ GainLife | Cost$ 1 | Defined$ You | LifeAmount$ 1 | SorcerySpeed$ True | SpellDescription$ x\nOracle:x\n"
	e, _, id := newFixtureDeck(t, 34, src)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	addMana(t, e, 0, "G")
	e.Advance()
	abilityOption(t, e, id, 0) // seat 0's own main phase, empty stack: offered
	bolt := addToHand(t, e, 0, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	addMana(t, e, 0, "R")
	castObj(t, e, bolt) // stack non-empty now
	if _, ok := findAbilityOption(e, id, 0); ok {
		t.Fatal("sorcery-speed ability offered with a spell on the stack")
	}
	tapper := "Name:Tapper\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nA:AB$ GainLife | Cost$ T | Defined$ You | LifeAmount$ 1 | SpellDescription$ x\nOracle:x\n"
	e2, _, elf := newFixtureDeck(t, 35, tapper)
	e2.emit(events.Event{Kind: events.MoveZone, Obj: elf, From: state.ZHand, To: state.ZBattlefield})
	e2.Advance()
	if _, ok := findAbilityOption(e2, elf, 0); ok {
		t.Fatal("a summoning-sick creature's {T} ability was offered")
	}
}

func TestCantBeActivatedValidSASparesManaAbilities(t *testing.T) {
	needle := "Name:Needle\nManaCost:1\nTypes:Artifact\nS:Mode$ CantBeActivated | ValidCard$ Card.NamedCard | ValidSA$ Activated.!ManaAbility | Description$ x\nOracle:x\n"
	e, _, n := newFixtureDeck(t, 36, needle)
	e.emit(events.Event{Kind: events.MoveZone, Obj: n, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Choose, Obj: n, Counter: "name", Text: "Ballista"})
	b := putCreature(t, e, 0, "Name:Ballista\nManaCost:X X\nTypes:Artifact Creature Construct\nPT:0/0\nA:AB$ DealDamage | Cost$ SubCounter<1/P1P1> | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n")
	e.emit(events.Event{Kind: events.CounterChange, Obj: b, Counter: "P1P1", Amount: 1})
	land := putLands(t, e, 0, 1)[0]
	e.emit(events.Event{Kind: events.Choose, Obj: n, Counter: "name", Text: "Mountain"})
	e.Advance()
	if _, ok := findAbilityOption(e, b, 0); !ok {
		t.Fatal("Ballista (not the named card any more) should be activatable")
	}
	if !hasActivateOption(e, land) {
		t.Fatal("the named land's mana ability must still be offered: ValidSA spares mana abilities")
	}
}
```

Write `abilityOption`, `findAbilityOption`, `hasActivateOption`, `indexOfPlayerOption` helpers. Run — FAIL.

- [ ] **Step 2: Implement**

`legal.go`: after the mana-ability loop, for each battlefield permanent controlled by `p` (and each of `p`'s graveyard cards), for `i, ab := range f.Abilities` with `ab.Kind == "AB" && ab.API != "Mana"`: skip unless the zone matches `ActivationZone$` (default battlefield); skip if `ab.Params["SorcerySpeed"] == "True" && !sorcery`; skip if `e.abilityRestricted(id, ab)`; skip if `cost.Tap` and (`o.Tapped` or a creature with `SummonSick` and no Haste); skip unless `e.castable(p, id, cost)`; add `Option{Kind: "ability", Label: f.Name + ": " + ab.Params["SpellDescription"], Obj: id, Ability: i}`. `statics.go` `abilityRestricted(id, ab *cards.SA)`: `ValidSA$` absent → applies to every activated ability including mana; `Activated.!ManaAbility` → applies unless `ab.API == "Mana"`; the mana-ability call site passes the mana SA. `activate.go`: `beginActivation(p, opt)` builds a `pendingCast{ability: opt.Ability, from: o.Zone, cost: ParseCost(ab.Params["Cost"])}` and runs the same stages; `commitCast` for `ability >= 0`: pay mana, `Tap` (`Tap` event), `SubCounter` (`CounterChange -N`), sacrifices, then `AbilityPush{Obj, Amount: ability, Player}` and `askTarget(p, top, ab)` when the SA targets. `handlePriority`: `case "ability": e.emit(Priority{p, 0}); e.beginActivation(p, opt)`. Bots: in the `KPriority` branch, after the `play_land`/`cast` loop and before the explicit pass, `if isMain { for … if o.Kind == "ability" { pick } }` — in both bots, verbatim.

- [ ] **Step 3: Run, gates, commit**

Run: `go test -count=1 ./rules/ ./seat/ && go test -race -count=1 ./rules/ && go test ./rules/ -run 'TestEveryRepoDeck|TestHeads' -count=1 -v | grep -E 'ratchet|chain head'; make sim | grep -c 'replay OK'`
Expected: PASS; ratchet still 24 (nothing registers here); heads move (Spectral Sailor draws, Ballista shoots, Batterskull bounces… name them); `20`.

```bash
git add rules/ seat/
git commit -m "feat(rules): activated abilities are options; tap, counter and sacrifice costs

Chain heads: <old -> new>. Cause: <cards> now activate their abilities."
```

---

### Task 11: keyword expansion at link time

**Files:**
- Create: `cards/keywords.go`, `cards/keywords_test.go`
- Modify: `cards/link.go` (`link` calls `f.expandKeywords()` first), `cards/registry.go` (`cacheVersion = 3`), `cards/primitive.go` (doc only)

**Interfaces:**
- Produces: `func (f *Face) expandKeywords()` — idempotent (a second call adds nothing), appends entries tagged `Params["Keyword"] = <head>`; the SVars it needs are added under names beginning `__kw` so they can never collide with a script's own. Table:

| Keyword | Expands to |
|---|---|
| `etbCounter:<KIND>:<N>` | `R:Event$ Moved \| Destination$ Battlefield \| ValidCard$ Card.Self \| ReplacementResult$ Updated \| ReplaceWith$ __kwEtbCounter<i> \| Description$ CARDNAME enters with <N> <KIND> counters.` + SVar `DB$ PutCounter \| Defined$ Self \| CounterType$ <KIND> \| CounterNum$ <N> \| ETB$ True` |
| `ETBReplacement:Other:<SVar>` | `R:Event$ Moved \| Destination$ Battlefield \| ValidCard$ Card.Self \| ReplacementResult$ Updated \| ReplaceWith$ <SVar>` |
| `Undying` | `T:Mode$ ChangesZone \| Origin$ Battlefield \| Destination$ Graveyard \| ValidCard$ Card.Self+counters_EQ0_P1P1 \| Execute$ __kwUndying \| TriggerDescription$ Undying` + SVar `DB$ ChangeZone \| Defined$ TriggeredNewCardLKICopy \| Origin$ Graveyard \| Destination$ Battlefield \| WithCountersType$ P1P1 \| WithCountersAmount$ 1` |
| `Evolve` | `T:Mode$ ChangesZone \| Destination$ Battlefield \| ValidCard$ Creature.YouCtrl+Other \| Evolve$ True \| Execute$ __kwEvolve \| TriggerDescription$ Evolve` + SVar `DB$ PutCounter \| Defined$ Self \| CounterType$ P1P1 \| CounterNum$ 1` |
| `Exalted` | `T:Mode$ Attacks \| ValidCard$ Creature.YouCtrl \| Alone$ True \| Execute$ __kwExalted \| TriggerDescription$ Exalted` + SVar `DB$ Pump \| Defined$ TriggeredAttacker \| NumAtt$ +1 \| NumDef$ +1` |
| `Prowess` | `T:Mode$ SpellCast \| ValidCard$ Card.nonCreature \| ValidActivatingPlayer$ You \| Execute$ __kwProwess \| TriggerDescription$ Prowess` + SVar `DB$ Pump \| Defined$ Self \| NumAtt$ +1 \| NumDef$ +1` |
| `Storm` | `T:Mode$ SpellCast \| ValidCard$ Card.Self \| TriggerZones$ Stack \| Execute$ __kwStorm \| TriggerDescription$ Storm` + SVar `DB$ CopySpellAbility \| Defined$ TriggeredSpellAbility \| Amount$ Count$ThisTurnCast/Minus1 \| MayChooseTarget$ True` |
| `Living Weapon` | `T:Mode$ ChangesZone \| Destination$ Battlefield \| ValidCard$ Card.Self \| Execute$ __kwLivingWeapon \| TriggerDescription$ Living weapon` + SVars `DB$ Token \| TokenScript$ b_0_0_phyrexian_germ \| TokenOwner$ You \| RememberTokens$ True \| SubAbility$ __kwLWAttach` and `DB$ Attach \| Defined$ Remembered \| Object$ Self` |
| `Equip:<cost>` | `A:AB$ Attach \| Cost$ <cost> \| ValidTgts$ Creature.YouCtrl \| TgtPrompt$ Select target creature you control \| SorcerySpeed$ True \| SpellDescription$ Equip <cost>` |
| `Enchant:<spec>` | `A:SP$ Attach \| ValidTgts$ <spec> \| TgtPrompt$ Select target <spec> \| Object$ Self` (only when the face has no SP$ ability) |
| `Kicker`, `Surge`, `Flashback`, `Delve`, `Flash`, `Miracle`, `Protection …`, `Indestructible`, `Devoid` | nothing — cast-time or static semantics live in `rules` |

`Face.Primitives()` therefore lists, for a Batterskull, `kw:Living Weapon`, `kw:Equip`, `api:Token`, `api:Attach`, `trig:ChangesZone`… — every mechanic it needs.

- [ ] **Step 1: Write the failing tests**

`cards/keywords_test.go`:

```go
package cards

import "testing"

func expanded(t *testing.T, src string) *Face {
	t.Helper()
	c, diags := ParseBytes("k.txt", []byte(src))
	if len(diags) > 0 {
		t.Fatal(diags)
	}
	if d := c.Link(); len(d) > 0 {
		t.Fatal(d)
	}
	return c.Faces[0]
}

func TestEtbCounterExpandsToAReplacement(t *testing.T) {
	f := expanded(t, "Name:Endless\nManaCost:X\nTypes:Creature Eldrazi\nPT:0/0\nK:etbCounter:P1P1:X\nSVar:X:Count$xPaid\nOracle:x\n")
	if len(f.Repls) != 1 || f.Repls[0].Event != "Moved" || f.Repls[0].Params["Keyword"] != "etbCounter" || f.Repls[0].With == nil {
		t.Fatalf("%+v", f.Repls)
	}
	w := f.Repls[0].With
	if w.API != "PutCounter" || w.Params["CounterType"] != "P1P1" || w.Params["CounterNum"] != "X" || w.Params["Defined"] != "Self" {
		t.Fatalf("%+v", w)
	}
}

func TestTriggerKeywordsExpandWithLinkedEffects(t *testing.T) {
	cases := map[string]struct{ src, mode, api string }{
		"Undying":       {"K:Undying", "ChangesZone", "ChangeZone"},
		"Evolve":        {"K:Evolve", "ChangesZone", "PutCounter"},
		"Exalted":       {"K:Exalted", "Attacks", "Pump"},
		"Prowess":       {"K:Prowess", "SpellCast", "Pump"},
		"Storm":         {"K:Storm", "SpellCast", "CopySpellAbility"},
		"Living Weapon": {"K:Living Weapon", "ChangesZone", "Token"},
	}
	for kw, tc := range cases {
		f := expanded(t, "Name:C\nManaCost:1\nTypes:Creature\nPT:1/1\n"+tc.src+"\nOracle:x\n")
		if len(f.Triggers) != 1 {
			t.Errorf("%s: %d triggers", kw, len(f.Triggers))
			continue
		}
		tr := f.Triggers[0]
		if tr.Mode != tc.mode || tr.Params["Keyword"] != kw || tr.Effect == nil || tr.Effect.API != tc.api {
			t.Errorf("%s: %+v effect %+v", kw, tr.Params, tr.Effect)
		}
	}
	lw := expanded(t, "Name:B\nManaCost:5\nTypes:Artifact Equipment\nK:Living Weapon\nOracle:x\n")
	if sub := lw.Triggers[0].Effect.Sub; sub == nil || sub.API != "Attach" || sub.Params["Defined"] != "Remembered" {
		t.Fatalf("living weapon sub-ability %+v", sub)
	}
}

func TestEquipAndEnchantExpandToAbilities(t *testing.T) {
	eq := expanded(t, "Name:Sword\nManaCost:3\nTypes:Artifact Equipment\nK:Equip:2\nOracle:x\n")
	if len(eq.Abilities) != 1 || eq.Abilities[0].Kind != "AB" || eq.Abilities[0].API != "Attach" || eq.Abilities[0].Params["Cost"] != "2" || eq.Abilities[0].Params["SorcerySpeed"] != "True" || eq.Abilities[0].Params["ValidTgts"] != "Creature.YouCtrl" {
		t.Fatalf("%+v", eq.Abilities)
	}
	aura := expanded(t, "Name:Rancor\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n")
	sp := aura.SpellAbility()
	if sp == nil || sp.API != "Attach" || sp.Params["ValidTgts"] != "Creature" || sp.Params["Object"] != "Self" {
		t.Fatalf("%+v", sp)
	}
}

func TestExpansionIsIdempotentAndTagged(t *testing.T) {
	c, _ := ParseBytes("k.txt", []byte("Name:C\nManaCost:1\nTypes:Creature\nPT:1/1\nK:Prowess\nK:Equip:1\nOracle:x\n"))
	c.Link()
	n, m := len(c.Faces[0].Triggers), len(c.Faces[0].Abilities)
	c.Link()
	if len(c.Faces[0].Triggers) != n || len(c.Faces[0].Abilities) != m {
		t.Fatal("a second Link expanded again")
	}
	prims := c.Primitives()
	for _, want := range []string{"kw:Prowess", "kw:Equip", "trig:SpellCast", "api:Pump", "api:Attach"} {
		if !contains(prims, want) {
			t.Errorf("primitives lack %s: %v", want, prims)
		}
	}
}

func TestUnexpandedKeywordsStayAlone(t *testing.T) {
	f := expanded(t, "Name:C\nManaCost:1\nTypes:Creature\nPT:1/1\nK:Flash\nK:Kicker:R\nK:Delve\nK:Protection from blue\nOracle:x\n")
	if len(f.Triggers)+len(f.Repls)+len(f.Abilities) != 0 {
		t.Fatalf("cast-time/static keywords must not expand: %d/%d/%d", len(f.Triggers), len(f.Repls), len(f.Abilities))
	}
}
```

Run: `go test ./cards/ -run 'Expand|Keyword' -count=1` — FAIL.

- [ ] **Step 2: Implement `cards/keywords.go`**

```go
package cards

import "strings"

// expandKeywords turns each keyword the engine implements through ordinary
// machinery into the triggered ability, replacement effect or activated
// ability Forge itself expands it to (CardFactoryUtil, in spirit), tagged
// Params["Keyword"] so nothing downstream needs to know the difference. The
// SVars it adds start with "__kw" and cannot collide with a script's own.
// Idempotent: an already-tagged entry is never added twice. Keywords whose
// meaning is a casting option (Kicker, Surge, Flashback, Delve, Flash,
// Miracle) or a static property (Protection, Indestructible, Devoid) are
// not expanded: rules reads them directly.
func (f *Face) expandKeywords() {
	if f.SVars == nil {
		f.SVars = map[string]string{}
	}
	has := func(kind, kw string) bool {
		switch kind {
		case "T":
			for _, t := range f.Triggers {
				if t.Params["Keyword"] == kw {
					return true
				}
			}
		case "R":
			for _, r := range f.Repls {
				if r.Params["Keyword"] == kw {
					return true
				}
			}
		case "A":
			for _, a := range f.Abilities {
				if a.Params["Keyword"] == kw {
					return true
				}
			}
		}
		return false
	}
	for i, k := range f.Keywords {
		head := KeywordHead(k)
		param := ""
		if j := strings.IndexByte(k, ':'); j >= 0 {
			param = strings.TrimSpace(k[j+1:])
		}
		switch head {
		case "etbCounter":
			if has("R", head) {
				continue
			}
			kind, n, _ := strings.Cut(param, ":")
			sv := "__kwEtbCounter" + itoa(i)
			f.SVars[sv] = "DB$ PutCounter | Defined$ Self | CounterType$ " + kind + " | CounterNum$ " + n + " | ETB$ True"
			f.Repls = append(f.Repls, Repl{Event: "Moved", Params: parseParams(
				"Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv +
					" | Keyword$ etbCounter | Description$ CARDNAME enters with " + n + " " + kind + " counters.")})
		case "ETBReplacement":
			if has("R", head) {
				continue
			}
			_, sv, _ := strings.Cut(param, ":") // "Other:ChooseCT" -> "ChooseCT"
			f.Repls = append(f.Repls, Repl{Event: "Moved", Params: parseParams(
				"Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv + " | Keyword$ ETBReplacement")})
		case "Undying":
			f.addKeywordTrigger(head, "Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self+counters_EQ0_P1P1 | TriggerDescription$ Undying",
				"DB$ ChangeZone | Defined$ TriggeredNewCardLKICopy | Origin$ Graveyard | Destination$ Battlefield | WithCountersType$ P1P1 | WithCountersAmount$ 1", has)
		case "Evolve":
			f.addKeywordTrigger(head, "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Creature.YouCtrl+Other | Evolve$ True | TriggerDescription$ Evolve",
				"DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1", has)
		case "Exalted":
			f.addKeywordTrigger(head, "Mode$ Attacks | ValidCard$ Creature.YouCtrl | Alone$ True | TriggerDescription$ Exalted",
				"DB$ Pump | Defined$ TriggeredAttacker | NumAtt$ +1 | NumDef$ +1", has)
		case "Prowess":
			f.addKeywordTrigger(head, "Mode$ SpellCast | ValidCard$ Card.nonCreature | ValidActivatingPlayer$ You | TriggerDescription$ Prowess",
				"DB$ Pump | Defined$ Self | NumAtt$ +1 | NumDef$ +1", has)
		case "Storm":
			f.addKeywordTrigger(head, "Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | TriggerDescription$ Storm",
				"DB$ CopySpellAbility | Defined$ TriggeredSpellAbility | Amount$ Count$ThisTurnCast/Minus1 | MayChooseTarget$ True", has)
		case "Living Weapon":
			if has("T", head) {
				continue
			}
			f.SVars["__kwLWAttach"] = "DB$ Attach | Defined$ Remembered | Object$ Self"
			f.addKeywordTrigger(head, "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | TriggerDescription$ Living weapon",
				"DB$ Token | TokenScript$ b_0_0_phyrexian_germ | TokenOwner$ You | RememberTokens$ True | SubAbility$ __kwLWAttach", has)
		case "Equip":
			if has("A", head) {
				continue
			}
			sa, _ := parseSA("", "AB$ Attach | Cost$ "+param+" | ValidTgts$ Creature.YouCtrl | TgtPrompt$ Select target creature you control | SorcerySpeed$ True | Keyword$ Equip | SpellDescription$ Equip "+param)
			if sa != nil {
				f.Abilities = append(f.Abilities, sa)
			}
		case "Enchant":
			if has("A", head) || f.SpellAbility() != nil {
				continue
			}
			sa, _ := parseSA("", "SP$ Attach | ValidTgts$ "+param+" | TgtPrompt$ Select target "+strings.ToLower(param)+" | Object$ Self | Keyword$ Enchant")
			if sa != nil {
				f.Abilities = append(f.Abilities, sa)
			}
		}
	}
}

// addKeywordTrigger appends one tagged T: line whose Execute$ is an SVar
// this function creates, unless the keyword was already expanded.
func (f *Face) addKeywordTrigger(kw, trigger, effect string, has func(kind, kw string) bool) {
	if has("T", kw) {
		return
	}
	sv := "__kw" + strings.ReplaceAll(kw, " ", "")
	f.SVars[sv] = effect
	p := parseParams(trigger + " | Execute$ " + sv + " | Keyword$ " + kw)
	f.Triggers = append(f.Triggers, Trigger{Mode: p["Mode"], Params: p})
}
```

(`itoa` → `strconv.Itoa`; `parseParams`/`parseSA` are the parser's own; check their exact names in `parse.go`.) `link()`: call `f.expandKeywords()` as its first statement, so the resolve pass links the `__kw` SVars like any other. `registry.go`: `cacheVersion = 3`. `TriggerZones$ Stack` requires `zoneGate` to understand `Stack` — check `ParseZone("Stack")` is already `ZStack` (it is) and that `zoneGate` splits `TriggerZones$` on commas (read it; extend if it only takes one zone).

- [ ] **Step 3: Recompile, measure, commit**

Run: `go test -count=1 ./cards/ && make compile-cards && go test -count=1 ./rules/ && go test ./rules/ -run 'TestEveryRepoDeck|TestHeads' -count=1 -v | grep -E 'ratchet|chain head'; make sim | grep -c 'replay OK'; make report | grep '^cards:'`
Expected: cards PASS; ratchet still `24 of 136` — the expansion adds *needs*, never registrations (Batterskull now lists `api:Token`/`api:Attach` too, which the table entry must reflect: update its expected list); heads MOVE — Monastery Swiftspear's Prowess and Knight of Infamy's Exalted now pump through existing `Pump`; the `report` count changes either way — record all three. `20`.

```bash
git add cards/ rules/acceptance_test.go rules/heads_test.go
git commit -m "feat(cards): expand engine-handled keywords into triggers, replacements and abilities at link time

Chain heads: <old -> new>. Cause: Prowess and Exalted now pump through the
ordinary trigger path. make report: <playable> (expanded keywords now
declare every mechanic they need)."
```

---

### Task 12: enters-the-battlefield replacements and choices

**Files:**
- Modify: `rules/replacement.go` (Ctx.X from the object; the `Updated` path runs the effect after the move), `rules/cast.go` (the ETB-choice stage; `play_land` goes through a one-stage flow), `rules/turn.go` (`chooseETB`), `rules/statics.go` (spec context with `Chosen`/SVar resolution), `rules/trigger_match.go` (spec context), `effects/cardflow.go` (`NameCard` honours a recorded choice), `rules/acceptance_test.go` (seven entries)
- Create: `effects/choose.go` (`ChooseType`, `ChooseNumber`), `effects/choose_test.go`, `rules/etb_test.go`

**Interfaces:**
- Produces: `func (e *Engine) specCtx(source state.ObjID, you state.PlayerID) effects.SpecContext` whose `Resolve` answers `"Chosen"` (source's `ChosenNumber`) and any SVar name on the source's face via `EvalCount`; every `MatchesSpecFrom` in `statics.go` and `trigger_match.go` becomes `MatchesSpecCtx(…, e.specCtx(…))`. Cast/play flow stage `etbChoices`: for each `Repl` tagged `Keyword$ ETBReplacement` whose `With.API` is `NameCard`/`ChooseType`/`ChooseNumber`, a `KChoose` with kind `name`/`type`/`number`, recorded with a `Choose` event on the card before it is put on the stack (or onto the battlefield for a land). Options: names — the distinct face names of nonland cards (per `ValidCards$`) in the chooser's hand, every battlefield and every graveyard, sorted; types — the creature subtypes on cards the chooser owns anywhere, sorted, or `["Human"]` when none; numbers — `0`…`12`. Effects `ChooseType`/`ChooseNumber` registered: at resolution they are no-ops when the object already carries the choice, else record the first option deterministically (R-9). `NameCard` likewise. Registered: `kw:etbCounter`, `kw:ETBReplacement`, `api:ChooseType`, `api:ChooseNumber`. Ratchet: 17.

- [ ] **Step 1: Write the failing tests**

`rules/etb_test.go`:

```go
func TestEtbCounterUsesTheChosenX(t *testing.T) {
	e, cfg, id := newFixtureDeck(t, 41, "Name:Endless\nManaCost:X\nTypes:Creature Eldrazi\nPT:0/0\nK:etbCounter:P1P1:X\nSVar:X:Count$xPaid\nOracle:x\n")
	addMana(t, e, 0, "GGG")
	castFirst(t, e, "cast")
	submitChoices(t, e, 3) // X = 3
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.Counter("P1P1") != 3 || e.Power(id) != 3 {
		t.Fatalf("%+v power %d", o, e.Power(id))
	}
	replayCheck(t, e, cfg)
	// X = 0: a 0/0 that dies to state-based actions at once.
	e2, _, id2 := newFixtureDeck(t, 42, "Name:Endless\nManaCost:X\nTypes:Creature Eldrazi\nPT:0/0\nK:etbCounter:P1P1:X\nSVar:X:Count$xPaid\nOracle:x\n")
	castFirst(t, e2, "cast")
	submitChoices(t, e2, 0)
	passUntilStackEmpty(t, e2, 20)
	if e2.G.Obj(id2).Zone != state.ZGraveyard {
		t.Fatalf("a 0/0 survived: %s", e2.G.Obj(id2).Zone)
	}
}

func TestChaliceCountersSpellsOfTheChargedManaValue(t *testing.T) {
	chalice := "Name:Chalice\nManaCost:X X\nTypes:Artifact\nK:etbCounter:CHARGE:X\n" +
		"T:Mode$ SpellCast | ValidCard$ Card.cmcEQY | ValidActivatingPlayer$ Player | TriggerZones$ Battlefield | Execute$ TrigCounter | TriggerDescription$ x\n" +
		"SVar:TrigCounter:DB$ Counter | Defined$ TriggeredSpellAbility\nSVar:X:Count$xPaid\nSVar:Y:Count$CardCounters.CHARGE\nOracle:x\n"
	e, cfg, ch := newFixtureDeck(t, 43, chalice)
	addMana(t, e, 0, "GG")
	castFirst(t, e, "cast")
	submitChoices(t, e, 1) // X = 1
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(ch).Counter("CHARGE") != 1 {
		t.Fatal("chalice has no charge counter")
	}
	bolt := addToHand(t, e, 1, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	passToPlayerOne(t, e) // stack_test.go helper
	addMana(t, e, 1, "R")
	life := e.G.Players[0].Life
	castObj(t, e, bolt)
	passUntilStackEmpty(t, e, 30)
	if e.G.Obj(bolt).Zone != state.ZGraveyard || e.G.Players[0].Life != life {
		t.Fatalf("bolt %s life %d (want countered, life unchanged)", e.G.Obj(bolt).Zone, e.G.Players[0].Life)
	}
	replayCheck(t, e, cfg)
}

func TestSanctumPrelateNumberIsChosenAtCastAndRestrictsCasting(t *testing.T) {
	prelate := "Name:Prelate\nManaCost:1 W W\nTypes:Creature Human Cleric\nPT:2/2\nK:ETBReplacement:Other:ChooseNumber\n" +
		"SVar:ChooseNumber:DB$ ChooseNumber | Defined$ You | SpellDescription$ As CARDNAME enters, choose a number.\n" +
		"S:Mode$ CantBeCast | ValidCard$ Card.nonCreature+cmcEQChosen | Description$ x\nOracle:x\n"
	e, cfg, pr := newFixtureDeck(t, 44, prelate)
	addMana(t, e, 0, "WWW")
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "number" || len(d.Options) != 13 {
		t.Fatalf("number choice %+v", d)
	}
	submitChoices(t, e, 1) // choose 1
	if e.G.Obj(pr).ChosenNumber != 1 {
		t.Fatal("choice not recorded on the card")
	}
	passUntilStackEmpty(t, e, 20)
	bolt := addToHand(t, e, 1, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	passToPlayerOne(t, e)
	addMana(t, e, 1, "R")
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == bolt {
			t.Fatal("a 1-mana noncreature spell was castable under Prelate on 1")
		}
	}
	replayCheck(t, e, cfg)
}

func TestNeedleNamesACardAndCavernChoosesAType(t *testing.T) {
	needle := "Name:Needle\nManaCost:1\nTypes:Artifact\nK:ETBReplacement:Other:DBNameCard\n" +
		"SVar:DBNameCard:DB$ NameCard | Defined$ You | SpellDescription$ x\n" +
		"S:Mode$ CantBeActivated | ValidCard$ Card.NamedCard | ValidSA$ Activated.!ManaAbility | Description$ x\nOracle:x\n"
	e, cfg, n := newFixtureDeck(t, 45, needle)
	b := putCreature(t, e, 1, "Name:Ballista\nManaCost:X X\nTypes:Artifact Creature Construct\nPT:0/0\nA:AB$ DealDamage | Cost$ SubCounter<1/P1P1> | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n")
	e.emit(events.Event{Kind: events.CounterChange, Obj: b, Counter: "P1P1", Amount: 2})
	addMana(t, e, 0, "G")
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "name" {
		t.Fatalf("name choice %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Label == "Ballista" {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("Ballista (on the battlefield) not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(n).ChosenName != "Ballista" {
		t.Fatal("name not recorded")
	}
	passToPlayerOne(t, e)
	if _, ok := findAbilityOption(e, b, 0); ok {
		t.Fatal("the named card's ability was offered")
	}
	replayCheck(t, e, cfg)

	cavern := "Name:Cavern\nManaCost:no cost\nTypes:Land\nK:ETBReplacement:Other:ChooseCT\n" +
		"SVar:ChooseCT:DB$ ChooseType | Defined$ You | Type$ Creature | SpellDescription$ x\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add {C}.\nOracle:x\n"
	e2, cfg2, cv := newFixtureDeck(t, 46, cavern)
	addToHand(t, e2, 0, "Name:Grunt\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n")
	castFirst(t, e2, "play_land")
	d = e2.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "type" || d.Options[0].Label != "Goblin" {
		t.Fatalf("type choice %+v", d)
	}
	submitChoices(t, e2, 0)
	if o := e2.G.Obj(cv); o.Zone != state.ZBattlefield || o.ChosenType != "Goblin" {
		t.Fatalf("cavern %+v", o)
	}
	replayCheck(t, e2, cfg2)
}
```

`effects/choose_test.go`: `ChooseType`/`ChooseNumber` on an object with a recorded choice emit nothing; without one they emit a `Choose` event with the deterministic fallback. Run — FAIL.

- [ ] **Step 2: Implement**

`rules/replacement.go`: when building the `Ctx` for a `ReplaceWith` effect, set `X: o.X` from the moving object; confirm the `Updated` path emits the original move first and then resolves the effect (that is what Task 29 built for "enters tapped"; keep it). `rules/cast.go`: stage `etbChoices` between the sacrifices and the commit: collect `(kind, options)` from the card's tagged Repls; ask one at a time (`chooseETB`); on answer, emit `Choose{Obj, Counter: kind, Text/Amount}`. `play_land` in `handlePriority`: if the land has ETB choices, run the same stage then `MoveZone` + `LandPlayed`; else as today. `specCtx` and its use in `castRestricted`/`abilityRestricted`/`blockRestricted`/`adjustedCost`/`alternativeCosts` (statics.go) and every matcher in `trigger_match.go`. `effects/choose.go`:

```go
package effects

// ChooseType and ChooseNumber record an "as this enters" choice on the
// source. The real choice is asked by rules at cast time and recorded with a
// Choose event before this ever resolves (plan ruling R-6), so with a
// choice already present these do nothing. Without one — a script that uses
// them outside an ETB replacement — they record the deterministic fallback
// (the first creature type the controller owns / 0) rather than asking,
// which the mid-resolution machinery (M2d-2) replaces when it lands for each.
func init() {
	Register("ChooseType", effChooseType)
	Register("ChooseNumber", effChooseNumber)
}
```

with bodies emitting `Choose` events; `effNameCard` gains the same "already chosen → no-op" guard and otherwise emits `Choose{name}` instead of only a `Note`. Registrations `kw:etbCounter`, `kw:ETBReplacement` in `rules/replacement.go`'s `init` (create one). Delete the seven ratchet entries (Chalice of the Void, Endless One, Walking Ballista, Cavern of Souls, Phyrexian Revoker, Pithing Needle, Sanctum Prelate).

- [ ] **Step 3: Run, gates, commit**

Run: full rules/effects tests with `-race`; `TestEveryRepoDeck` → `17 of 136`; heads move (Chalice, Endless One, Ballista, Cavern, Revoker, Needle, Prelate now do things) — name them; `make sim` 20; `make report`.

```bash
git add rules/ effects/
git commit -m "feat(rules): ETB replacements with X counters and as-enters choices

Chain heads: <old -> new>. Cause: <cards>. Ratchet 24 -> 17. make report: <n>."
```

---

## Phase 3 — the remaining primitives

### Task 13: tokens

**Files:**
- Create: `effects/token.go`, `effects/token_test.go`
- Modify: `rules/sba.go` (a token off the battlefield ceases to exist → exile), `rules/sba_test.go`, `rules/acceptance_test.go` (two entries), `internal/testutil` / `cmd/mtgsim` / acceptance harness (pass `Tokens: reg.Tokens`)

**Interfaces:**
- Produces: `api:Token` — `TokenScript$` (comma-separated stems, each created `TokenAmount$` times, default 1), `TokenOwner$` (`You` default; `Opponent`/`Player`/`Defined`-style forms via `Defined`-like resolution — implement `You` and `Opponent`, else the controller), `RememberTokens$ True` appends the created objects to `Ctx.Remembered` (for Living Weapon's attach). Each token is one `TokenCreate` event; an unknown script key is a `Note` ("unknown token script") and nothing is created. SBA: a token in any zone but the battlefield or stack is moved to exile with Text "ceased to exist" (once; exile-to-exile is not re-moved). Ratchet: 15.

- [ ] **Step 1: Write the failing tests**

`effects/token_test.go` (with a fake host whose `Game().Tokens` holds two fixtures):

```go
func TestTokenCreatesEachScriptTheGivenNumberOfTimes(t *testing.T) {
	h, c := fixtureHostWithTokens(t) // Game.Tokens: r_1_1_goblin, c_3_3_a_phyrexian_wurm_deathtouch
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{"TokenAmount": "2", "TokenScript": "r_1_1_goblin", "TokenOwner": "You"}})
	if n := countKind(h, events.TokenCreate); n != 2 {
		t.Fatalf("%d TokenCreate events", n)
	}
	bf := h.Game().Zone(state.ZBattlefield, c.Controller)
	if len(bf) != 2 || !h.Game().Obj(bf[0]).IsToken || h.Game().Obj(bf[0]).Face().Name != "Goblin Token" {
		t.Fatalf("battlefield %v", bf)
	}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{"TokenScript": "c_3_3_a_phyrexian_wurm_deathtouch,r_1_1_goblin", "RememberTokens": "True"}})
	if len(c.Remembered) != 2 || len(h.Game().Zone(state.ZBattlefield, c.Controller)) != 4 {
		t.Fatalf("remembered %v", c.Remembered)
	}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Token", Params: map[string]string{"TokenScript": "no_such"}})
	if countKind(h, events.TokenCreate) != 4 {
		t.Fatal("unknown script created something")
	}
}
```

`rules/sba_test.go` (append):

```go
func TestATokenThatDiesCeasesToExist(t *testing.T) {
	e, _, _ := newFixtureDeckWithTokens(t, 51, "Name:Pyro\nManaCost:1 R\nTypes:Creature Human Shaman\nPT:2/1\nOracle:x\n")
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "r_1_1_goblin"})
	tok := e.G.Zone(state.ZBattlefield, 0)[len(e.G.Zone(state.ZBattlefield, 0))-1]
	e.emit(events.Event{Kind: events.Damage, Obj: tok, Amount: 1})
	e.checkStateBased()
	o := e.G.Obj(tok)
	if o.Zone != state.ZExile || len(e.G.Zone(state.ZGraveyard, 0)) != 0 {
		t.Fatalf("dead token in %s; graveyard %v", o.Zone, e.G.Zone(state.ZGraveyard, 0))
	}
	n := len(e.L.Events)
	e.checkStateBased()
	if len(e.L.Events) != n {
		t.Fatal("an exiled token keeps being re-exiled")
	}
}
```

(`newFixtureDeckWithTokens` = `newFixtureDeck` with `Config.Tokens` holding the two fixture token cards parsed in the test file.) Run — FAIL.

- [ ] **Step 2: Implement**

`effects/token.go`:

```go
package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("Token", effToken) }

// effToken creates TokenAmount$ tokens of each TokenScript$ (comma list) for
// TokenOwner$ (You by default). Every token is its own TokenCreate event:
// Apply mints the object from Game.Tokens, so replay creates the very same
// objects. RememberTokens$ True hands the new objects to the rest of the
// chain through Ctx.Remembered (Living Weapon attaches to its Germ this way).
func effToken(h Host, c *Ctx, sa *cards.SA) {
	g := h.Game()
	n := Num(h, c, sa, "TokenAmount", 1)
	owner := c.Controller
	switch sa.Params["TokenOwner"] {
	case "Opponent":
		for _, p := range g.AliveFrom(c.Controller) {
			if p != c.Controller {
				owner = p
				break
			}
		}
	}
	remember := sa.Params["RememberTokens"] == "True"
	for _, key := range strings.Split(sa.Params["TokenScript"], ",") {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := g.Tokens[key]; !ok {
			h.Emit(events.Event{Kind: events.Note, Obj: c.Source, Text: "unknown token script " + key})
			continue
		}
		for i := int32(0); i < n; i++ {
			before := len(g.Objs)
			h.Emit(events.Event{Kind: events.TokenCreate, Player: owner, Text: key})
			if remember && len(g.Objs) > before {
				c.Remembered = append(c.Remembered, state.Target{Obj: g.Objs[len(g.Objs)-1].ID})
			}
		}
	}
}
```

`rules/sba.go`: a new pass `exileDeadTokens` in `checkStateBased`'s loop: for every object with `IsToken && Zone != Battlefield && Zone != Stack && Zone != Exile` → `MoveZone` to exile, Text "ceased to exist"; report whether anything moved so the fixed-point loop re-runs. Wire `Tokens: reg.Tokens` everywhere a `Config` is built from a registry. Registration `api:Token` happens through `Register`. Delete the Young Pyromancer and Wurmcoil Engine entries.

- [ ] **Step 3: Run, gates, commit**

Ratchet `15 of 136`; heads move (Young Pyromancer makes Elementals; Wurmcoil leaves Wurms) — name them; `make sim` 20; `make report` (api:Token unlocks ~2 900 corpus cards — record the jump).

```bash
git add effects/ rules/ internal/ cmd/
git commit -m "feat(effects): tokens

Chain heads: <old -> new>. Cause: Young Pyromancer and Wurmcoil Engine
create tokens. Ratchet 17 -> 15. make report: <n>."
```

---

### Task 14: attachments — Attach, Equip, Enchant, Living Weapon

**Files:**
- Create: `effects/attach.go`, `effects/attach_test.go`, `rules/attach.go`, `rules/attach_test.go`
- Modify: `effects/filter.go` (`EquippedBy`, `EnchantedBy`, `AttachedBy`), `rules/sba.go` (attachment legality), `rules/stack.go` (an Aura resolving: attach then enter), `rules/legal.go` (Equip is an ordinary sorcery-speed ability — nothing new; confirm), `rules/acceptance_test.go` (four entries)

**Interfaces:**
- Produces: `api:Attach` — `Object$` (what attaches; `Self` default) onto `Defined$`/targets (the first legal one); refuses (Note) when the target is not a permanent, is the object itself, or is protected from the attachment's colours (Task 15 adds the protection check — leave a hook `attachable(g, obj, target) bool`). Predicates: `EquippedBy` / `EnchantedBy` / `AttachedBy` — the candidate is the permanent `source` is attached to (`g.Obj(source).AttachedTo == o.ID`), so `Affected$ Creature.EquippedBy` on an Equipment's static matches exactly the equipped creature. `Defined$ Equipped` (Task 5) already reads `AttachedTo`. SBAs (CR 704.5m/n): an Aura attached to nothing, to a non-permanent, or to something its `Enchant` spec no longer matches → graveyard; an Equipment attached to a non-creature, or whose bearer left → detached (`Attach` with no target); anything attached to an object that left the battlefield → detached. Aura casting: the expanded `SP$ Attach` targets per `Enchant`; `resolveTop` runs it (attaching the still-on-the-stack Aura) and then moves the Aura onto the battlefield — `Move` to the battlefield keeps `AttachedTo`. Registered: `kw:Equip`, `kw:Enchant`, `kw:Living Weapon`, `api:Attach`. Ratchet: 11.

- [ ] **Step 1: Write the failing tests**

`rules/attach_test.go` (the fixture texts are the four ratchet cards' own shapes, reduced):

```go
func TestEquipAttachesAndTheStaticFollowsTheBearer(t *testing.T) {
	sword := "Name:Sword\nManaCost:3\nTypes:Artifact Equipment\nK:Equip:2\n" +
		"S:Mode$ Continuous | Affected$ Creature.EquippedBy | AddPower$ 2 | AddToughness$ 2 | AddKeyword$ Vigilance | Description$ x\nOracle:x\n"
	e, cfg, sw := newFixtureDeck(t, 61, sword)
	e.emit(events.Event{Kind: events.MoveZone, Obj: sw, From: state.ZHand, To: state.ZBattlefield})
	bear := putCreature(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	addMana(t, e, 0, "GG")
	e.Advance()
	opt := abilityOption(t, e, sw, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || len(d.Options) != 1 || d.Options[0].Obj != bear {
		t.Fatalf("equip target %+v", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(sw).AttachedTo != bear || e.Power(bear) != 4 || !e.HasKeyword(bear, "Vigilance") {
		t.Fatalf("attached to %d, bear power %d", e.G.Obj(sw).AttachedTo, e.Power(bear))
	}
	// The bearer dies: the Equipment stays, detached.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	e.checkStateBased()
	if e.G.Obj(sw).Zone != state.ZBattlefield || e.G.Obj(sw).AttachedTo != 0 {
		t.Fatalf("sword %s attached %d", e.G.Obj(sw).Zone, e.G.Obj(sw).AttachedTo)
	}
	replayCheck(t, e, cfg)
}

func TestAuraTargetsOnCastAttachesOnResolutionAndDiesWithItsBearer(t *testing.T) {
	rancor := "Name:Rancor\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"S:Mode$ Continuous | Affected$ Creature.EnchantedBy | AddPower$ 2 | AddKeyword$ Trample | Description$ x\n" +
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ TrigChangeZone | TriggerDescription$ x\n" +
		"SVar:TrigChangeZone:DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | Defined$ TriggeredNewCardLKICopy\nOracle:x\n"
	e, cfg, ra := newFixtureDeck(t, 62, rancor)
	bear := putCreature(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	addMana(t, e, 0, "G")
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Options[0].Obj != bear {
		t.Fatalf("aura target %+v", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(ra).Zone != state.ZBattlefield || e.G.Obj(ra).AttachedTo != bear || e.Power(bear) != 4 || !e.HasKeyword(bear, "Trample") {
		t.Fatalf("rancor %s on %d; bear power %d", e.G.Obj(ra).Zone, e.G.Obj(ra).AttachedTo, e.Power(bear))
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	e.checkStateBased()
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(ra).Zone != state.ZHand {
		t.Fatalf("Rancor should have died and returned to hand, is in %s", e.G.Obj(ra).Zone)
	}
	replayCheck(t, e, cfg)
}

func TestLivingWeaponCreatesAGermAndAttaches(t *testing.T) {
	skull := "Name:Skull\nManaCost:5\nTypes:Artifact Equipment\nK:Living Weapon\nK:Equip:5\n" +
		"S:Mode$ Continuous | Affected$ Creature.EquippedBy | AddPower$ 4 | AddToughness$ 4 | Description$ x\nOracle:x\n"
	e, cfg, sk := newFixtureDeckWithTokens(t, 63, skull) // Tokens includes b_0_0_phyrexian_germ
	addMana(t, e, 0, "GGGGG")
	castFirst(t, e, "cast")
	passUntilStackEmpty(t, e, 30)
	bf := e.G.Zone(state.ZBattlefield, 0)
	var germ state.ObjID
	for _, id := range bf {
		if e.G.Obj(id).IsToken {
			germ = id
		}
	}
	if germ == 0 || e.G.Obj(sk).AttachedTo != germ || e.Power(germ) != 4 || e.Toughness(germ) != 4 {
		t.Fatalf("germ %d attached %d power %d", germ, e.G.Obj(sk).AttachedTo, e.Power(germ))
	}
	replayCheck(t, e, cfg)
}

func TestIllegalAttachmentsAreCleanedUp(t *testing.T) {
	e, _, aura := newFixtureDeck(t, 64, "Name:Aura\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: aura, From: state.ZHand, To: state.ZBattlefield}) // attached to nothing
	e.checkStateBased()
	if e.G.Obj(aura).Zone != state.ZGraveyard {
		t.Fatal("an Aura attached to nothing must go to the graveyard")
	}
}
```

`effects/attach_test.go`: `Attach` with `Object$ Self` onto a `Defined$ Remembered` target emits `Attach{Obj: source, IDs: [target]}`; onto a player or a non-permanent emits a `Note` and no `Attach`; `EquippedBy`/`EnchantedBy` predicates match only the attached-to permanent. Run — FAIL.

- [ ] **Step 2: Implement**

`effects/attach.go`: `effAttach` — `obj := c.Source` (or `Object$` forms `Self`/`Remembered` first object); for the first target from `Defined(h, c, sa)` that is an object on the battlefield, not `obj`, and `Attachable(h, obj, target)` (a hook: true for now; Task 15 adds the protection check) → `h.Emit(Attach{Obj: obj, IDs: [target]})`; otherwise a `Note`. `filter.go` predicates:

```go
	"EquippedBy":  attachedBy, "EnchantedBy": attachedBy, "AttachedBy": attachedBy,
// attachedBy: the candidate is what the source is attached to.
func attachedBy(g *state.Game, o *state.Object, _ state.PlayerID, src state.ObjID) bool {
	s := g.Obj(src)
	return s != nil && s.AttachedTo == o.ID && s.Zone == state.ZBattlefield
}
```

`rules/attach.go`: `attachmentSBAs()` called from `checkStateBased`'s loop — for every battlefield permanent with `AttachedTo != 0`: target gone/not on the battlefield → Aura to graveyard (Text "unattached"), Equipment detached; Aura whose `Enchant` spec (`KeywordParam("Enchant")`) the bearer no longer matches → graveyard; Equipment on a non-creature → detached; an Aura on the battlefield with `AttachedTo == 0` → graveyard. `stack.go` `resolveTop`: no change is needed for the Aura — its `SP$ Attach` runs like any spell ability, then `IsPermanent()` moves it to the battlefield, and `Move` to the battlefield does not clear `AttachedTo` (Task 4). Confirm with the test; if the fizzle path is hit (no legal target at resolution), the Aura goes to the graveyard — correct (CR 704.5m). Registrations in `rules/attach.go`'s `init`: `kw:Equip`, `kw:Enchant`, `kw:Living Weapon`. Delete Batterskull, Sword of Fire and Ice, Umezawa's Jitte, Rancor from the table.

- [ ] **Step 3: Run, gates, commit**

Ratchet `11 of 136`; heads move (equipment and Rancor now attach); `make sim` 20; `make report` (api:Attach + kw:Equip + kw:Enchant unlock ~2 000 corpus cards).

```bash
git add effects/ rules/
git commit -m "feat(rules): attachments — Attach, Equip, Enchant, Living Weapon

Chain heads: <old -> new>. Cause: <cards>. Ratchet 15 -> 11. make report: <n>."
```

---

### Task 15: protection

**Files:**
- Create: `rules/protection.go`, `rules/protection_test.go`
- Modify: `rules/combat.go` (`canBlock`; the damage assignment carries its source), `rules/stack.go` (`askTarget`, `legalTargets`), `rules/engine.go` (`emit` drops prevented damage), `effects/attach.go` (`Attachable` consults the host: add `Host.ProtectedFrom`? — no: keep `Host` small; rules re-checks protection at `Attach` emit time in `emit` and drops the event with a Note), `rules/acceptance_test.go` (one entry)

**Interfaces:**
- Produces: `func (e *Engine) protectedFrom(target, source state.ObjID) bool` — true when `target`'s derived keywords include `Protection from <quality>` and `source` has that quality: a colour word (`white`…`green`) against `effects.ColorsOf(source)`; `artifacts`/`creatures`/`enchantments`/`instants`/`sorceries` against the source's types; `everything`. `e.damageSourceFor` — the engine records, on `Engine`, the object whose resolution or combat assignment is emitting damage (`e.damaging state.ObjID`, set around `resolveAbility` calls and each combat assignment). In `emit`: a `Damage` event whose `Obj` is protected from `e.damaging` is replaced by a `Note` ("prevented: protection"); an `Attach` whose target is protected from the attachment is dropped the same way. `canBlock`: a blocker the attacker is protected from cannot block. `askTarget`/`legalTargets`: an object protected from the targeting spell/ability's source is never offered and is illegal at resolution. Registered: `kw:Protection from white`, `… blue`, `… black`, `… red`, `… green`. Ratchet: 10.

- [ ] **Step 1: Write the failing tests**

```go
func TestProtectionFromBlueBlocksTargetingBlockingAndDamage(t *testing.T) {
	pile := "Name:Piledriver\nManaCost:1 R\nTypes:Creature Goblin Warrior\nPT:1/2\nK:Protection from blue\nOracle:x\n"
	e, cfg, pd := newFixtureDeck(t, 71, pile)
	e.emit(events.Event{Kind: events.MoveZone, Obj: pd, From: state.ZHand, To: state.ZBattlefield})
	// Targeting: a blue spell cannot target it.
	shock := addToHand(t, e, 1, "Name:BlueShock\nManaCost:U\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2\nOracle:x\n")
	redShock := addToHand(t, e, 1, "Name:RedShock\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ 2\nOracle:x\n")
	passToPlayerOne(t, e)
	addMana(t, e, 1, "UR")
	if opts := castTargetsFor(t, e, shock); len(opts) != 0 {
		t.Fatalf("blue spell offered targets %+v", opts)
	}
	if opts := castTargetsFor(t, e, redShock); len(opts) != 1 || opts[0].Obj != pd {
		t.Fatalf("red spell targets %+v", opts)
	}
	// Damage: a blue source's damage is prevented.
	blueGuy := putCreature(t, e, 1, "Name:Merfolk\nManaCost:U\nTypes:Creature Merfolk\nPT:2/2\nOracle:x\n")
	e.damaging = blueGuy
	e.emit(events.Event{Kind: events.Damage, Obj: pd, Amount: 2})
	e.damaging = 0
	if e.G.Obj(pd).Damage != 0 {
		t.Fatal("blue damage was not prevented")
	}
	// Blocking: a blue creature cannot block it.
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{pd}})
	if e.canBlock(blueGuy, pd) {
		t.Fatal("a blue creature blocked a creature with protection from blue")
	}
	redGuy := putCreature(t, e, 1, "Name:Goblin\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n")
	if !e.canBlock(redGuy, pd) {
		t.Fatal("a red creature should be able to block")
	}
	replayCheck(t, e, cfg)
}

func TestGrantedProtectionCountsAndDevoidIsColourless(t *testing.T) {
	e, _, bear := newFixtureDeck(t, 72, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZHand, To: state.ZBattlefield})
	effects.Resolve(e, &effects.Ctx{Source: bear, Controller: 0, Targets: []state.Target{{Obj: bear}}},
		&cards.SA{Kind: "DB", API: "Pump", Params: map[string]string{"KW": "Protection from red"}})
	red := putCreature(t, e, 1, "Name:Goblin\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n")
	devoid := putCreature(t, e, 1, "Name:Breaker\nManaCost:6 G\nTypes:Creature Eldrazi\nPT:5/7\nK:Devoid\nOracle:x\n")
	if !e.protectedFrom(bear, red) || e.protectedFrom(bear, devoid) {
		t.Fatal("granted protection from red must apply to a red source and not to a devoid one")
	}
}
```

`castTargetsFor(t, e, obj)` chooses the cast option for `obj` and returns the target decision's options without answering (an empty slice when the cast completed with no target decision, as a no-legal-target fizzle does). Run — FAIL.

- [ ] **Step 2: Implement**

`rules/protection.go`: `protectedFrom` as specified (case-insensitive; keywords via `e.Keywords(target)`; qualities split on " and " / "from" prefixes — `Protection from red and from blue` arrives as two keywords via `&`, so parse one quality per keyword after "Protection from "). `engine.go`: `damaging state.ObjID` field; `emit`: before replacements, `if ev.Kind == events.Damage && ev.Obj != 0 && e.damaging != 0 && e.protectedFrom(ev.Obj, e.damaging) { return e.emit(Note{Obj: ev.Obj, Text: "prevented: protection"}) }` (same for `Attach` with `e.protectedFrom(target, ev.Obj)`). Set `e.damaging` around `resolveAbility` in `resolveTop` (the resolving object's `Source` for abilities, the spell itself otherwise) and per combat `assignment` (record `from state.ObjID` on `assignment`, set `e.damaging = x.from` before each damage emit, clear after). `combat.go` `canBlock`: `if e.protectedFrom(attacker, blocker) { return false }`. `stack.go`: `askTarget` skips objects protected from `source`; `legalTargets` gains a `source` parameter and drops them. Registrations in `protection.go`'s `init`. Delete Goblin Piledriver from the table (Knight of Infamy stays: Exalted).

- [ ] **Step 3: Run, gates, commit**

Ratchet `10 of 136`; heads may move (Piledriver, Knight, Sword bearers); `make sim` 20; `make report`.

```bash
git add rules/
git commit -m "feat(rules): protection — targeting, blocking, damage prevention, attachment

Chain heads: <old -> new>. Cause: <cards>. Ratchet 11 -> 10. make report: <n>."
```

---

### Task 16: keyword triggers — Undying, Evolve, Exalted, Prowess

**Files:**
- Modify: `rules/trigger_match.go` (`counters_EQ0_P1P1` against LKI; `Evolve$ True` condition; `Alone$ True`), `effects/filter.go` (`MatchesObjectCtx` — the same matcher over an object value, for LKI), `effects/zone.go` (`ChangeZone` honours `WithCountersType$`/`WithCountersAmount$`), `effects/count.go` (`counters_<CMP><N>_<KIND>` numeric predicate form), `rules/keyword_registration_test.go`, `rules/acceptance_test.go` (five entries)

**Interfaces:**
- Produces: in `zoneChangeMatches`, when the trigger's source is the moving object (`ev.Obj == source`) and `Ctx.LKI` is available, `ValidCard$` is evaluated against the LKI object (`effects.MatchesObjectCtx`); the predicate `counters_EQ0_P1P1` (general form `counters_<LE|GE|EQ|LT|GT><n>_<KIND>`) is added to `filter.go`. `Evolve$ True`: the trigger fires only when the entering creature's derived power or toughness exceeds the source's. `Alone$ True`: fires only when exactly one attacker was declared. `ChangeZone … | WithCountersType$ K | WithCountersAmount$ N` emits a `CounterChange` after the move onto the battlefield. Registered: `kw:Undying`, `kw:Evolve`, `kw:Exalted`, `kw:Prowess`. Ratchet: 5.

- [ ] **Step 1: Write the failing tests**

```go
func TestUndyingReturnsOnceWithACounter(t *testing.T) {
	geist := "Name:Geist\nManaCost:G G\nTypes:Creature Spirit\nPT:2/1\nK:Haste\nK:Undying\nOracle:x\n"
	e, cfg, g := newFixtureDeck(t, 81, geist)
	e.emit(events.Event{Kind: events.MoveZone, Obj: g, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.Damage, Obj: g, Amount: 3})
	e.checkStateBased()
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(g); o.Zone != state.ZBattlefield || o.Counter("P1P1") != 1 || e.Power(g) != 3 {
		t.Fatalf("after first death: %s, counters %d", o.Zone, o.Counter("P1P1"))
	}
	e.emit(events.Event{Kind: events.Damage, Obj: g, Amount: 5})
	e.checkStateBased()
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(g).Zone != state.ZGraveyard {
		t.Fatal("undying returned a creature that had a +1/+1 counter")
	}
	replayCheck(t, e, cfg)
}

func TestEvolveGrowsOnlyForBiggerCreatures(t *testing.T) {
	e, cfg, one := newFixtureDeck(t, 82, "Name:One\nManaCost:G\nTypes:Creature Human Ooze\nPT:1/1\nK:Evolve\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: one, From: state.ZHand, To: state.ZBattlefield})
	small := putCreature(t, e, 0, "Name:Small\nManaCost:1\nTypes:Creature\nPT:1/1\nOracle:x\n")
	_ = small
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(one).Counter("P1P1") != 0 {
		t.Fatal("evolved for an equal-size creature")
	}
	putCreature(t, e, 0, "Name:Big\nManaCost:2\nTypes:Creature\nPT:2/2\nOracle:x\n")
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(one).Counter("P1P1") != 1 {
		t.Fatal("did not evolve for a bigger creature")
	}
	putCreature(t, e, 1, "Name:Theirs\nManaCost:5\nTypes:Creature\nPT:5/5\nOracle:x\n")
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(one).Counter("P1P1") != 1 {
		t.Fatal("evolved for an opponent's creature")
	}
	replayCheck(t, e, cfg)
}

func TestExaltedPumpsALoneAttackerAndProwessPumpsOnNoncreatureSpells(t *testing.T) {
	knight := "Name:Knight\nManaCost:1 B\nTypes:Creature Human Knight\nPT:2/1\nK:Exalted\nOracle:x\n"
	e, cfg, k := newFixtureDeck(t, 83, knight)
	e.emit(events.Event{Kind: events.MoveZone, Obj: k, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{k}})
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.Power(k) != 3 {
		t.Fatalf("lone attacker power %d", e.Power(k))
	}
	e.cleanupStep()
	other := putCreature(t, e, 0, "Name:Other\nManaCost:1\nTypes:Creature\nPT:1/1\nOracle:x\n")
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{k, other}})
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.Power(k) != 2 {
		t.Fatal("exalted fired for two attackers")
	}
	replayCheck(t, e, cfg)

	e2, cfg2, sw := newFixtureDeck(t, 84, "Name:Swift\nManaCost:R\nTypes:Creature Human Monk\nPT:1/2\nK:Haste\nK:Prowess\nOracle:x\n")
	e2.emit(events.Event{Kind: events.MoveZone, Obj: sw, From: state.ZHand, To: state.ZBattlefield})
	bolt := addToHand(t, e2, 0, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	addMana(t, e2, 0, "R")
	e2.Advance()
	castObj(t, e2, bolt)
	passUntilStackEmpty(t, e2, 20)
	if e2.Power(sw) != 2 || e2.Toughness(sw) != 3 {
		t.Fatalf("prowess: %d/%d", e2.Power(sw), e2.Toughness(sw))
	}
	replayCheck(t, e2, cfg2)
}
```

Run — FAIL (Undying and Evolve conditions, Alone).

- [ ] **Step 2: Implement**

`effects/filter.go`: export `MatchesObjectCtx(g, spec, o *state.Object, sc SpecContext) bool` (the existing loop body over an object value; `MatchesSpecCtx` becomes a lookup + call); numeric predicate family `counters_<CMP><n>_<KIND>` (`counters_EQ0_P1P1`) reading `o.Counter(kind)`. `trigger_match.go` `zoneChangeMatches`: when `ev.Obj == source` and the pending LKI is available (thread it: `checkTriggers` passes `lki` into `triggerMatches(t, source, ev, lki)`), evaluate `ValidCard$` with `MatchesObjectCtx(g, spec, lki, …)`; `Evolve$ True`: after the ordinary match, require `ev.To == Battlefield` and (`e.Power(ev.Obj) > e.Power(source) || e.Toughness(ev.Obj) > e.Toughness(source)`); `attacksMatches`: `Alone$ True` → `len(ev.IDs) == 1`. `effects/zone.go` `effChangeZone`: after a move to the battlefield, if `WithCountersType$` is set, emit `CounterChange{Obj, Counter: kind, Amount: WithCountersAmount$ (default 1)}`. Registrations in `trigger_match.go`'s `init`. Delete Geralf's Messenger, Strangleroot Geist, Experiment One, Knight of Infamy, Monastery Swiftspear.

- [ ] **Step 3: Run, gates, commit**

Ratchet `5 of 136`; heads move; `make sim` 20; `make report`.

```bash
git add rules/ effects/
git commit -m "feat(rules): Undying, Evolve, Exalted and Prowess through expanded triggers

Chain heads: <old -> new>. Cause: <cards>. Ratchet 10 -> 5. make report: <n>."
```

---

### Task 17: Storm and CopySpellAbility

**Files:**
- Create: `effects/copy.go`, `effects/copy_test.go`, `rules/storm_test.go`
- Modify: `effects/registry.go` (`Host.CastThisTurn() int`), `effects/count.go` (`Count$ThisTurnCast`), `rules/stack.go` (Engine implements `CastThisTurn`; a copy resolving leaves to exile), `rules/acceptance_test.go` (three entries)

**Interfaces:**
- Produces: `api:CopySpellAbility` — copies the object named by `Defined$` (`Parent`/`TriggeredSpellAbility` → the spell on the stack) `Amount$` times (default 1) with `StackCopy` events; copies keep their targets (`MayChooseTarget$` is a player choice this build cannot ask mid-resolution — R-8/R-9 — so they keep them, with a `Note` per copy); `UnlessCost$`/`UnlessPayer$` (Chain Lightning) is declined with a `Note` and no copy (R-8). `Host.CastThisTurn()` = spells cast this turn by anyone (from the log; `Count$ThisTurnCast` reads it). A copy that leaves the stack goes to exile (`spellRestZone` returns exile for `IsCopy`). Registered: `api:CopySpellAbility`, `kw:Storm`. Ratchet: 2.

- [ ] **Step 1: Write the failing tests**

```go
func TestStormCopiesTheSpellOncePerSpellCastBefore(t *testing.T) {
	tendrils := "Name:Tendrils\nManaCost:2 B B\nTypes:Sorcery\nK:Storm\nA:SP$ LoseLife | ValidTgts$ Player | LifeAmount$ 2 | SubAbility$ DBGainLife\nSVar:DBGainLife:DB$ GainLife | Defined$ You | LifeAmount$ 2\nOracle:x\n"
	e, cfg, td := newFixtureDeck(t, 91, tendrils)
	for i := 0; i < 2; i++ {
		b := addToHand(t, e, 0, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n")
		addMana(t, e, 0, "R")
		castObj(t, e, b)
		passUntilStackEmpty(t, e, 20)
	}
	if e.CastThisTurn() != 2 {
		t.Fatalf("cast this turn %d", e.CastThisTurn())
	}
	addMana(t, e, 0, "BBGG")
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life
	castObj(t, e, td) // target: the bot-style helper picks the opponent
	e.Advance()
	passUntilStackEmpty(t, e, 40)
	if e.G.Players[1].Life != life1-6 || e.G.Players[0].Life != life0+6 {
		t.Fatalf("life %d/%d: want original + two copies", e.G.Players[0].Life, e.G.Players[1].Life)
	}
	copies := 0
	for _, o := range e.G.Objs {
		if o.IsCopy {
			copies++
			if o.Zone != state.ZExile {
				t.Fatalf("a resolved copy sits in %s", o.Zone)
			}
		}
	}
	if copies != 2 {
		t.Fatalf("%d copies", copies)
	}
	replayCheck(t, e, cfg)
}

func TestChainLightningDeclinesTheMayPay(t *testing.T) {
	chain := "Name:Chain\nManaCost:R\nTypes:Sorcery\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3 | SubAbility$ DBCopy1\n" +
		"SVar:DBCopy1:DB$ CopySpellAbility | Defined$ Parent | Controller$ TargetedOrController | UnlessPayer$ TargetedOrController | UnlessCost$ R R | UnlessSwitched$ True | MayChooseTarget$ True\nOracle:x\n"
	e, cfg, ch := newFixtureDeck(t, 92, chain)
	addMana(t, e, 0, "R")
	addMana(t, e, 1, "RR")
	life := e.G.Players[1].Life
	castObj(t, e, ch)
	passUntilStackEmpty(t, e, 20)
	if e.G.Players[1].Life != life-3 {
		t.Fatal("chain lightning did not resolve")
	}
	for _, o := range e.G.Objs {
		if o.IsCopy {
			t.Fatal("a may-pay copy was created: UnlessCost is declined in this build (R-8)")
		}
	}
	if !hasNote(e, "declined") {
		t.Fatal("no Note recorded the declined may-pay")
	}
	replayCheck(t, e, cfg)
}
```

Run — FAIL.

- [ ] **Step 2: Implement**

`effects/copy.go`: `effCopySpellAbility` — if `UnlessCost$` present → `Note{"may pay declined (UnlessCost is not asked in this build)"}` and return; source spell = for `Defined$ Parent` the resolving spell (`c.Source`), for `TriggeredSpellAbility` the first object in `c.Remembered`; require it on the stack; `n := Num(h, c, sa, "Amount", 1)`; emit `StackCopy{Obj: spell, Player: controller}` n times, each followed by a `Note` "copy keeps its targets" when `MayChooseTarget$ True`. `Host.CastThisTurn()`; `Count$ThisTurnCast` → `h.CastThisTurn()`. `rules`: `func (e *Engine) CastThisTurn() int` (log walk, all players); `spellRestZone` → exile for `IsCopy`. Every `Host` double gains `CastThisTurn() int { return 0 }`. Registration `kw:Storm` in `trigger_match.go`'s `init` (the expansion exists since Task 11; the semantics complete here). Delete Tendrils of Agony, Empty the Warrens, Chain Lightning.

- [ ] **Step 3: Run, gates, commit**

Ratchet `2 of 136`; heads move; `make sim` 20; `make report`.

```bash
git add effects/ rules/
git commit -m "feat(effects): CopySpellAbility and Storm; may-pay clauses are declined

Chain heads: <old -> new>. Cause: <cards>. Ratchet 5 -> 2. make report: <n>."
```

---

### Task 18: Miracle

**Files:**
- Create: `rules/miracle.go`, `rules/miracle_test.go`
- Modify: `rules/trigger_match.go` (`checkTriggers` queues a miracle offer on a first draw), `rules/trigger_queue.go` (`pendingTrigger.Miracle`; `optionalDecider`/`triggerLabel`/`pushTrigger` for it), `rules/cast.go` (`"miracle"` mode), `rules/acceptance_test.go` (the last two entries; the table becomes empty)

**Interfaces:**
- Produces: when a `Draw` event is the first card its player has drawn this turn (count `Draw` events for that player since the last `TurnChange` in the log) and the drawn card has `Miracle`, `checkTriggers` queues `pendingTrigger{Source: card, Controller: owner, Miracle: true, Ctx: {Source: card, Controller: owner}}`. The queue treats it as an optional trigger whose decider is the owner: label `"Miracle — reveal <name> and cast it for <cost>?"`; a **yes** reveals the card (`Note{Player, IDs: [card], Text: "reveals <name> (miracle)"}`) and enters the cast flow with `Mode: "miracle"` and cost `KeywordParam("Miracle")` (X asked as usual); a **no** drops it. The card must still be in the owner's hand when the offer is placed, else the offer is dropped silently. Registered: `kw:Miracle`. Ratchet: 0.

- [ ] **Step 1: Write the failing tests**

```go
func TestMiracleOffersOnTheFirstDrawOnly(t *testing.T) {
	terminus := "Name:Terminus\nManaCost:4 W W\nTypes:Sorcery\nK:Miracle:W\nA:SP$ ChangeZoneAll | ChangeType$ Creature | Origin$ Battlefield | Destination$ Library | LibraryPosition$ -1\nOracle:x\n"
	e, cfg, term := newFixtureDeck(t, 101, terminus)
	// Put Terminus on top of seat 0's library and give them a creature to wipe.
	moveToLibraryTop(t, e, term)
	bear := putCreature(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	addMana(t, e, 0, "W")
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: term, From: state.ZLibrary, To: state.ZHand, Secret: true})
	if len(e.pendingTriggers) != 1 || !e.pendingTriggers[0].Miracle {
		t.Fatalf("no miracle offer: %+v", e.pendingTriggers)
	}
	e.Advance()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional || d.Player != 0 {
		t.Fatalf("offer decision %+v", d)
	}
	submitChoices(t, e, 0) // yes
	if e.G.Obj(term).Zone != state.ZStack || e.G.Obj(term).CastFlags&state.FlagMiracle == 0 || e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("terminus %s flags %d pool %d", e.G.Obj(term).Zone, e.G.Obj(term).CastFlags, e.G.Players[0].Pool.Total())
	}
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(bear).Zone != state.ZLibrary {
		t.Fatal("Terminus did not resolve")
	}
	replayCheck(t, e, cfg)

	// A second Miracle card drawn the same turn gets no offer.
	e2, _, t2 := newFixtureDeck(t, 102, terminus)
	moveToLibraryTop(t, e2, t2)
	e2.emit(events.Event{Kind: events.Draw, Player: 0, Obj: e2.G.Zone(state.ZLibrary, 0)[1], From: state.ZLibrary, To: state.ZHand, Secret: true})
	e2.pendingTriggers = nil
	e2.emit(events.Event{Kind: events.Draw, Player: 0, Obj: t2, From: state.ZLibrary, To: state.ZHand, Secret: true})
	if len(e2.pendingTriggers) != 0 {
		t.Fatal("miracle offered on a second draw")
	}
}

func TestMiracleWithXAsksX(t *testing.T) {
	entreat := "Name:Entreat\nManaCost:X X W W W\nTypes:Sorcery\nK:Miracle:X W W\nA:SP$ Token | TokenAmount$ X | TokenScript$ w_4_4_angel_flying | TokenOwner$ You\nSVar:X:Count$xPaid\nOracle:x\n"
	e, cfg, en := newFixtureDeckWithTokens(t, 103, entreat) // Tokens includes w_4_4_angel_flying
	moveToLibraryTop(t, e, en)
	addMana(t, e, 0, "WWWW")
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: en, From: state.ZLibrary, To: state.ZHand, Secret: true})
	e.Advance()
	submitChoices(t, e, 0) // yes
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "x" || len(d.Options) != 3 { // X = 0,1,2 with WWWW
		t.Fatalf("X after miracle: %+v", d)
	}
	submitChoices(t, e, 2)
	passUntilStackEmpty(t, e, 30)
	angels := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).IsToken {
			angels++
		}
	}
	if angels != 2 {
		t.Fatalf("%d angels", angels)
	}
	replayCheck(t, e, cfg)
}
```

`moveToLibraryTop(t, e, id)` — a helper that `SetZone`s the card to index 0 of its owner's library (direct genesis-style setup, before any log-relevant play). Run — FAIL.

- [ ] **Step 2: Implement**

`pendingTrigger` gains `Miracle bool`. `checkTriggers`: after the face loop, `if ev.Kind == events.Draw { e.offerMiracle(ev) }` — `offerMiracle` (in `miracle.go`) checks `KeywordParam("Miracle")` on the drawn card and `e.drawsThisTurn(ev.Player) == 1` (log walk). `optionalDecider`: a `Miracle` pending trigger is optional with decider = controller. `triggerLabel`: the miracle prompt. `pushTrigger`: `if pt.Miracle { e.castMiracle(pt); return }` — `castMiracle` verifies the card is still in the owner's hand, emits the reveal Note, and calls `beginCast(owner, decision.Option{Kind: "cast", Obj: card, Mode: "miracle"})`; the flow then asks X if needed (`chooseCast`) and, because the drain was interrupted, `handleChoose`'s cast case must resume the drain after the commit exactly as Task 7 did for targets (reuse `drainAwaitsTarget` renamed `drainAwaits`). `cast.go`: `"miracle"` mode → cost `ParseCost(KeywordParam("Miracle"))`, flag `FlagMiracle`. Registration `kw:Miracle` in `miracle.go`'s `init`. Delete Terminus and Entreat the Angels: the table is now `map[string][]string{}`; extend the test with `if len(measured) != 0 { t.Errorf("%d cards regressed", len(measured)) }` — the existing loops already name each one.

- [ ] **Step 3: Run, gates, commit**

Ratchet `0 of 136` — the log line reads `ratchet: 0 of 136 distinct cards … not fully supported`. Heads move (Terminus/Entreat fire in the games that draw them first). `make sim` 20. `make report`.

```bash
git add rules/
git commit -m "feat(rules): Miracle

Chain heads: <old -> new>. Cause: <cards>. Ratchet 2 -> 0: every card in
the 12 repo decks plays with its printed text. make report: <n>."
```

---

## Phase 4 — booked engine items and the close

### Task 19: `Defined$ ReplacedCard`

**Files:**
- Modify: `effects/registry.go` (`Ctx.Replaced state.ObjID`), `effects/context.go` (`Defined` form `ReplacedCard`), `rules/replacement.go` (sets `Replaced` on the effect's Ctx), `rules/replacement_test.go`

**Interfaces:**
- Produces: a replacement's `ReplaceWith$` effect resolves with `Ctx.Replaced` = the object the replaced event was about; `Defined$ ReplacedCard` yields it. Rest in Peace–class cards (`R:Event$ Moved | ValidCard$ Card | Destination$ Graveyard | ReplaceWith$ Exile` + `DB$ ChangeZone | Defined$ ReplacedCard | Destination$ Exile`) exile the card instead of the ensureLeftTheStack backstop parking it. The coverage report is unaffected (`Defined$` values are not primitives).

- [ ] **Step 1: Write the failing test**

```go
func TestRestInPeaceShapedReplacementExilesTheReplacedCard(t *testing.T) {
	rip := "Name:Peace\nManaCost:1 W\nTypes:Enchantment\n" +
		"R:Event$ Moved | ValidCard$ Card | Destination$ Graveyard | ReplaceWith$ Exile | Description$ If a card would be put into a graveyard from anywhere, exile it instead.\n" +
		"SVar:Exile:DB$ ChangeZone | Defined$ ReplacedCard | Destination$ Exile\nOracle:x\n"
	e, cfg, peace := newFixtureDeck(t, 111, rip)
	e.emit(events.Event{Kind: events.MoveZone, Obj: peace, From: state.ZHand, To: state.ZBattlefield})
	bear := putCreature(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 5})
	e.checkStateBased()
	if e.G.Obj(bear).Zone != state.ZExile {
		t.Fatalf("bear in %s, want exile", e.G.Obj(bear).Zone)
	}
	bolt := addToHand(t, e, 0, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n")
	addMana(t, e, 0, "R")
	e.Advance()
	castObj(t, e, bolt)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(bolt).Zone != state.ZExile {
		t.Fatalf("bolt in %s, want exile (the replacement relocated it; no backstop needed)", e.G.Obj(bolt).Zone)
	}
	if hasNote(e, "fully discarded") {
		t.Fatal("ensureLeftTheStack fired: the replacement should have relocated the card itself")
	}
	replayCheck(t, e, cfg)
}
```

Run — FAIL.

- [ ] **Step 2: Implement**

`Ctx.Replaced state.ObjID` with the doc "the object the replaced event was about (Defined$ ReplacedCard); zero outside a replacement". `Defined`: `case "ReplacedCard": if c.Replaced != 0 && g.Obj(c.Replaced) != nil { return []state.Target{{Obj: c.Replaced}} }; return nil`. `applyReplacements`: set `Replaced: ev.Obj` on the Ctx it builds. Update `ensureLeftTheStack`'s doc comment: the Rest in Peace shape no longer reaches it; it stays as the totality backstop for a replacement that relocates nothing.

- [ ] **Step 3: Run, gates, commit**

Heads may move if any deck card has this shape (Rest in Peace is not in the 12 decks; Dryad Militant/Leyline are not either — check `grep -l 'ReplaceWith' .cards/cardsfolder` against the deck lists; expect no movement). `make sim` 20.

```bash
git add effects/ rules/
git commit -m "feat(effects): Defined\$ ReplacedCard for replacement effects"
```

---

### Task 20: hygiene minors with a rules consequence

**Files:**
- Modify: `effects/misc.go` (`Repeat` honours `MaxRepeat$`; bounded), `effects/count.go` (`applyCountOp` in int64 with clamping), `rules/mana.go` (`ParseCost` generic sum clamped), tests beside each

**Interfaces:**
- Produces: `Repeat` runs `RepeatSubAbility$` `MaxRepeat$` times (the corpus's real parameter; `RepeatNum$` still honoured), capped at 1 000 so a malformed script cannot spin; `applyCountOp` computes in `int64` and clamps to `[math.MinInt32, math.MaxInt32]`; `ParseCost` clamps `Generic` at `math.MaxInt32` across tokens.

- [ ] **Step 1: Write the failing tests**

```go
// effects/misc_test.go
func TestRepeatHonoursMaxRepeatAndIsBounded(t *testing.T) {
	h, c := fixtureHost(t)
	c.SVars = map[string]string{"DBLife": "DB$ GainLife | Defined$ You | LifeAmount$ 1"}
	life := h.Game().Players[c.Controller].Life
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat", Params: map[string]string{"RepeatSubAbility": "DBLife", "MaxRepeat": "3"}})
	if h.Game().Players[c.Controller].Life != life+3 {
		t.Fatalf("MaxRepeat 3 ran %d times", h.Game().Players[c.Controller].Life-life)
	}
	Resolve(h, c, &cards.SA{Kind: "DB", API: "Repeat", Params: map[string]string{"RepeatSubAbility": "DBLife", "MaxRepeat": "999999"}})
	if got := h.Game().Players[c.Controller].Life - life - 3; got != 1000 {
		t.Fatalf("unbounded repeat ran %d times, want the 1000 cap", got)
	}
}
// effects/count_test.go
func TestCountOpsDoNotOverflow(t *testing.T) {
	if applyCountOp(math.MaxInt32, "Twice") != math.MaxInt32 || applyCountOp(math.MinInt32, "Negative") != math.MaxInt32 || applyCountOp(math.MaxInt32, "Plus5") != math.MaxInt32 {
		t.Fatal("count ops overflow")
	}
}
// rules/mana_test.go
func TestParseCostClampsAbsurdGeneric(t *testing.T) {
	if c := ParseCost("2147483647 2147483647"); c.Generic != math.MaxInt32 {
		t.Fatalf("generic %d", c.Generic)
	}
}
```

Run — FAIL.

- [ ] **Step 2: Implement** each as specified (three small edits; the `Repeat` doc comment's "RepeatNum$ … falls back to one run" paragraph is rewritten to the truth).

- [ ] **Step 3: Run, gates, commit**

Heads unchanged (no deck card uses Repeat or absurd numbers). `make sim` 20.

```bash
git add effects/ rules/
git commit -m "fix(effects): Repeat honours MaxRepeat and is bounded; count and cost arithmetic cannot overflow"
```

---

### Task 21: the end state — ratchet closed, documentation, merge

**Files:**
- Modify: `rules/acceptance_test.go` (final comment), `rules/heads_test.go` (final table), `AGENTS.md`, `README.md`, `docs/superpowers/specs/2026-09-04-gorge-post-m1-roadmap.md` (M2r row → done, with the final numbers)

- [ ] **Step 1: The full gate run**

```sh
make lint && go build ./... && go test -count=1 ./... && go test -race -count=1 ./rules/ ./effects/ ./events/ ./cards/
go test ./rules/ -run 'TestEveryRepoDeck|TestHeads|TestRepoDecks|TestRepoDeckGames' -count=1 -v | grep -E 'ratchet|chain head|^(ok|FAIL|---)'
make sim | grep -c 'replay OK'          # 20
make report | grep -E '^cards:|^tokens:'
git ls-files | grep -c '\.txt$'         # 0
```

Expected: everything green; `ratchet: 0 of 136`; the four heads equal the table; `20`; `0`. Record the final `playable` count.

- [ ] **Step 2: Documentation**

`AGENTS.md`: replace the "Status" section's ratchet paragraph with the end state (0 of 136; the final heads; the final report line; the corpus commit pin); add the six event kinds to the "Hard rules" paragraph about `events.Apply`; add a **Known approximations** list, one line each with the milestone that removes it:

- `UnlessCost$` ("may pay … if they do") is a real mid-resolution KModes ask — M2d-2 (R-8 closed).
- `Sacrifice` with `SacValid$` for a player target sacrifices that player's lowest-id matching permanent; `Discard` discards from the front of the hand; `Mana | Produced$ Any` makes colourless — a later milestone.
- `Charm` asks a real KModes decision when an engine host is present; a no-engine context keeps its deterministic first-mode stand-in with a Note — M2d-2.
- "As this enters, choose …" is asked at cast/play time, so opponents learn the choice a resolution early — resumable-resolution machinery now exists (M2d-2); migrating these asks is a later milestone.
- Copies keep the original's targets (`MayChooseTarget$`) — a later milestone.
- Cavern of Souls' second mana ability adds colourless with no spend restriction — M4 (mana restrictions).
- `Regenerate` grants a shield the state-based actions do not consume — M4.
- `Effect` (Vines of Vastwood's can't-be-targeted) records a Note only — M4.

`README.md`: the package list gains `deck` and the token scripts sentence in "Licensing boundary". Roadmap spec: M2r's row reads "done — 0 of 136; heads …; playable …".

- [ ] **Step 3: Commit, then finish the branch**

```bash
git add AGENTS.md README.md docs/superpowers/specs/2026-09-04-gorge-post-m1-roadmap.md rules/
git commit -m "docs: M2r closed — every repo-deck card plays; known approximations listed

Final chain heads: <2/4/6/8>. make report: <n>. Ratchet 0 of 136."
```

Then `superpowers:finishing-a-development-branch` on `m2r/ratchet`: full suite green, merge to `main` (fast-forward or a merge commit), push. If M2a has landed on `main` meanwhile, rebase `m2r/ratchet` onto `main` first and re-run the whole gate list; the only shared file is `view/view.go`'s `cardViews` skip (Task 4 here, Task 4 there) — resolve by keeping both conditions in one predicate.

---

## Self-review checklist (run by the plan author before execution)

1. **Scope coverage.** Roadmap M2r items: (1) split → Task 1; (2) the 21 primitives → Tasks 3, 9, 12–18 (the ratchet schedule lists which task retires each card, and every one of the 22 `kw:`/`api:` names in the original table is registered by exactly one task); (3) `Defined$ ReplacedCard` → Task 19; (4) T19c-b: Equip/Attach → 14, `TriggeredCard$` forms → 5, non-mana activated abilities → 10; (5) N2 `TargetMin$ 0` → 7; (6) hygiene → 20 (Repeat, overflow) and 1 (the replacement split). "Not M2r" (N4/800.4a, T21-h) stays booked. Machinery once booked-for-M2b is named in R-6/R-8/R-9 and the AGENTS.md list; M2d-2 closed R-8 (UnlessCost$ real, Charm asked).
2. **Placeholders.** Every `<old -> new>`, `<cards>`, `<n>` in a commit template is a measurement the implementer fills in at commit time, not a plan gap; every code step shows code; helpers named in tests are defined in the same task or named as existing (`newFixtureDeck`, `castFirst`, `passToPlayerOne`, `passUntilStackEmpty`, `replayFromLog`, `diffGames`, `writeCardFile`).
3. **Type consistency.** `pendingCast`/`chooseFor`/`beginCast`/`continueCast`/`commitCast` (9, 10, 12, 18); `SpecContext`/`MatchesSpecCtx`/`MatchesObjectCtx`/`specCtx` (5, 12, 16); `Ctx.LKI`/`Ctx.Replaced`/`Ctx.Remembered` (6, 16, 19); `Host.HasKeyword`/`Host.CastThisTurn` (3, 17); `Option.Mode`/`Option.Ability` (9, 10); event kinds and `state.Flag*` (4 onward); `KeywordParam` (9, 14, 18); `drainAwaits` (7, 18).
4. **Determinism.** Every option list and event sequence introduced walks `AliveFrom(0)`, zone slices, sorted names/types, or the log — never a map. Token minting, copies and abilities are events. No `time`, no global rand.
5. **Chain-head discipline.** Tasks 5, 7, 9–18 each say "heads move; name the cause"; Tasks 1–4, 6, 8, 19, 20 say "unchanged". The golden test makes silence impossible.
