# Your first card fix

This walkthrough takes one report, "a card plays wrong", from triage to a
merged fix. The worked example is the first outside contribution to gorge,
[`786510b3f`](https://github.com/adams-shaun/gorge/commit/786510b3f):
*Omniscience* in its owner's hand offered a free cast of itself, and *Future
Sight* on top of its library let its owner cast it from there. Both cards
were reading their own static ability in a zone where it does not function.

It is for a contributor fixing their first bug. You need the corpus fetched
and compiled first (`make fetch-cards compile-cards`), and if you work in a
git worktree, `.cards` linked into it; see [Contributing](index.md). Terms in
*italics* are defined in the [glossary](glossary.md).

## 1. Confirm and triage

**Read the card's script.** Every card is one text file under
`.cards/cardsfolder/<first letter>/<name>.txt`, lower-case with underscores:
`.cards/cardsfolder/o/omniscience.txt`, `.cards/cardsfolder/f/future_sight.txt`.
Each line is a characteristic (`Name:`, `ManaCost:`, `Types:`) or an ability:
`A:` activated or spell, `T:` trigger, `S:` static, `R:` replacement, `K:`
keyword, `SVar:` a named sub-ability or value. Abilities are `|`-separated
`Key$ Value` *parameters*. Omniscience's one `S:` line is a `Mode$ Continuous`
static with `MayPlay$ True`, `MayPlayWithoutManaCost$ True` and
`AffectedZone$ Hand`, and **no** `EffectZone$`. That last absence was the
whole bug.

The scripts are GPL-3.0. Read them as much as you like, but never paste one
into a commit, a test or an issue.

**Check that it is not already known or deliberate.**

- [`docs/agents/known-approximations/`](https://github.com/adams-shaun/gorge/tree/main/docs/agents/known-approximations)
  lists primitives that knowingly behave more narrowly than the card text.
- `docs/design/2026-09-22-engine-contracts.md` lists behaviour
  that is a deliberate contract, not a bug.
- `engine/rules/testdata/known-unsupported/` names repo-deck cards the build
  does not fully support.
- gorge-compliance's `scenarios/oracle/*/known-divergent/` lists audit
  scenarios already known to fail (and `known_bug` rows in its
  `scenarios/xmageport/` the XMage-ported ones).

**Decide whose bug it is.** Compare the script with the printed card (the
Oracle text on the card's `Oracle:` line, or Scryfall) and the
[Comprehensive Rules](https://magic.wizards.com/en/rules) ("CR"):

- **The script is wrong** (a wrong target restriction, a missing parameter,
  the wrong subtype on a token): it is a *Forge* bug. Fix it on the `gorge`
  branch of the fork [adams-shaun/forge](https://github.com/adams-shaun/forge),
  open the same fix as a PR to [Card-Forge/forge](https://github.com/Card-Forge/forge),
  and gorge picks it up when `FORGE_REF` in the `Makefile` is bumped. The
  `FORGE_REF` comment block lists past examples.
- **The script is right and gorge does the wrong thing**: an engine bug.
  Continue below. Omniscience's script is right: under CR 113.6, a static
  ability of a permanent card functions only on the battlefield unless it
  says otherwise, and Forge expresses "otherwise" with `EffectZone$`.

## 2. Reproduce it in a failing test

Pick whichever option fits. A, B and C all end in a test that fails before
your fix and passes after.

### Option A: an Oracle scenario (JSON, no Go)

The Oracle-text audit runs JSON scenarios that state what a card does
according to its **printed text and the CR**. You write the expected outcome
without reading the engine's opinion. The runner (gorge's
`engine/rules/oracle_run.go`, which also holds the schema: `oracleFile`,
`oracleScenario`) builds a real two-seat game around real corpus cards,
drives it only through player actions, and checks observable state. The
scenario corpus and its driver live in
[gorge-compliance](https://github.com/adams-shaun/gorge-compliance): the
contract is in the header of its `audit/oracle_audit_test.go`.

One file per card per family, in a gorge-compliance checkout:
`scenarios/oracle/<family>/<card-slug>.json`. Families are the existing
directories (`combat-damage`, `keywords`, `replacement`, …). Copy a
neighbour such as `scenarios/oracle/combat-damage/balefire-dragon.json` (trimmed
here to one scenario and two expectations):

```json
{
 "card": "Balefire Dragon",
 "family": "combat-damage",
 "author": "<you> (Oracle + CR only)",
 "scenarios": [
  {
   "name": "combat-damage-deals-that-much-to-each-creature",
   "cr": ["603.10a"],
   "why": "6 unblocked damage to p1 means each creature p1 controls takes 6 and dies.",
   "setup": {
    "p0": {"battlefield": ["Balefire Dragon"]},
    "p1": {"battlefield": ["Grizzly Bears", "Bronze Sable"]}
   },
   "steps": [
    {"op": "attack", "seat": 0, "attackers": ["p0:Balefire Dragon"], "defender": "p1"},
    {"op": "pass_to", "seat": 0, "step": "end"}
   ],
   "expect": [
    {"card": "p1:Grizzly Bears", "zone": "graveyard"},
    {"life": {"p1": 14}}
   ]
  }
 ]
}
```

- `why` and `cr` are required: every scenario states its reasoning.
- The step ops are a closed set (`oracleAuditOps` in gorge's
  `engine/rules/oracle_audit_export.go`): `mana`, `cast`, `activate`,
  `play`, `resolve`, `attack`, `block`, `pass`, `pass_to`, `move`, `life`.
  Unknown fields are rejected.
- `oracle_sha` is optional. When present it pins the card's printed text
  (`oracletext.Digest` in `engine/cards/oracletext`) so the scenario fails if the text changes.

Run it in gorge-compliance, with a `go.work` pointing at your gorge checkout
so it plays your engine change (subtest names replace spaces with `_`):

```sh
make oracle-audit ORACLE_RUN=Balefire_Dragon
# or: go test -run 'TestOracleAudit/Balefire_Dragon' ./audit/
```

`TestOracleScenarioFilesWellFormed` checks the schema without the corpus.
Every scenario must also replay from its event log alone, so the runner
doubles as a replay check. The scenario lands in gorge-compliance (its own
PR); the engine fix lands in gorge with a Go test (Option B) so gorge's own
suite holds it.

### Option B: a Go test in `engine/rules`

Use this when the bug is about what the engine *offers* (a cast option that
should not exist) or needs precise setup. The tests live in package `rules`
itself, so they can call unexported helpers. The ones you will reach for:

| Helper | Where | What it does |
|---|---|---|
| `handEngine(t, cards...)` | `engine/rules/legal_test.go` | Two-seat game, 40 Mountains each, exactly these cards in seat 0's hand, turn 1 main phase, seat 0 with priority. |
| `card(t, script)` | `engine/rules/turn_test.go` | Compiles a minimal inline script you write yourself (`"Name:Mountain\nTypes:Basic Land Mountain\nOracle:x\n"`). Never paste a corpus script. |
| `corpusCard(t, name)` | `engine/rules/life_draw_trigger_test.go` | Looks a real card up in the corpus registry (`testutil.CorpusRegistry`). |
| `corpusAlternativeCard(t, name)` | `engine/rules/alternative_costs_test.go` | The same lookup; the test below uses it. |
| `onBoard(t, e, p, script)` / `onBoardCard(t, e, p, c)` | `engine/rules/layers_test.go`, `engine/rules/signed_param_test.go` | Put a card straight onto a player's battlefield. |
| `castAndResolve(t, e, id)` | `engine/rules/defined_player_state_qualifier_test.go` | Submit the cast option for `id` and pass until the stack is empty. |
| `passUntil(t, e, cond)` | `engine/rules/explore_triggers_test.go` | Pass priority (and `Advance`) until `cond()` holds; fails on an unexpected non-priority ask. |
| `freeHandCastOption(e, p, id)` | `engine/rules/mayplay_free_hand_test.go` | Finds a free (no-mana-cost) cast option for a hand card. |

The contribution's test (`engine/rules/mayplay_self_effectzone_test.go`) is the shape to
copy:

```go
func TestOmniscienceInHandGrantsNothing(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Omniscience"), corpusAlternativeCard(t, "Craw Wurm"))
	omni, wurm := e.G.Zone(state.ZHand, 0)[0], e.G.Zone(state.ZHand, 0)[1]
	if _, ok := freeHandCastOption(e, 0, omni); ok {
		t.Errorf("Omniscience in hand offers its own free cast")
	}
	// ...and the control: on the battlefield it still works.
	e.emit(events.Event{Kind: events.MoveZone, Obj: omni, From: state.ZHand, To: state.ZBattlefield})
	if _, ok := freeHandCastOption(e, 0, wurm); !ok {
		t.Errorf("Omniscience on the battlefield offers Craw Wurm no free cast")
	}
}
```

Three habits worth copying:

- **Assert the control as well as the bug.** The fix must not break the
  card where it *should* work (on the battlefield).
- **Move things with events** (`e.emit(events.Event{...})`) once the game is
  running, so the log stays truthful.
- **Fixtures may write state directly**: the Future Sight test places the
  card with `e.G.AddObject`, sets `fs.Zone` and calls `e.G.SetZone`. That is
  allowed in test setup only. Engine code never writes `state.Game` outside
  `events.Apply`.

### Option C: a player's feedback capture

If the bug came from a game on a server, its feedback snapshot replays
exactly:

```sh
go run ./cmd/repro <feedback-dir>                        # replay, verify, print the board
go run ./cmd/repro -list <feedback-dir>                  # the intent timeline
go run ./cmd/repro -emit-test ./engine/rules <feedback-dir>   # copy the snapshot + write a failing test skeleton
```

The skeleton uses `feedback.EngineAt(t, dir, n)` from
`testkit/testutil/feedback`. Replace its TODO with your assertion.
[Contributing](index.md#reproduce-a-feedback-report) has the details,
including why a `DIVERGED` replay can be an expected corpus change.

## 3. Find the code that implements it

Card behaviour is dispatched by *primitive* name, so search for the name.

- **An `api:` effect** (`A:SP$ DealDamage`, `DB$ Pump`): `git grep -n
  'Register("DealDamage"' engine/effects` finds the implementation
  (`effects.Register` in an `init()`).
- **A keyword, trigger mode, static mode or replacement** (`kw:`, `trig:`,
  `stat:`, `repl:`): these are implemented in `engine/rules` and declared with
  `effects.RegisterNonAPI("kw:Flying", …)`. Search for the quoted name.
- **A keyword that expands into script lines** (Prowess becomes a trigger
  plus a pump): `engine/cards/kw_<name>.go`, registered with
  `registerKeyword`.
- **A parameter** (`EffectZone$`, `ValidTgts$`, `AffectedZone$`): `git grep -n
  '"EffectZone"' engine/` lists every reader. Omniscience's two wrong reads
  were in `engine/rules/mayplay.go` (`mayPlayStatic` and `mayPlayAltCosts`).

`make report` lists the primitives no implementation registers yet, ranked
by how many cards each would unlock.

## 4. Fix it, keeping the invariants

- Mutate state only by emitting events (`e.emit` in `engine/rules`;
  `h.Emit` and the other `effects.HostEmit` methods in `engine/effects`). Never assign to a `state.Game`
  field.
- No wall clock, no `math/rand` outside the game's seeded RNG, no `map` range
  whose order reaches an event.
- `engine/effects` must not import `engine/rules`. If an effect needs more
  than `effects.Host` offers, the logic belongs in `rules`.
- Comment the *rule*, with its CR number, at the line that enforces it, as
  the fix does.

## 5. Measure the class, not the card

A bug in one card is usually a bug in every card with the same shape. Before
calling it done, find the siblings. The example commit counted the 105 corpus
cards with a `Continuous MayPlay$ True` static and no `EffectZone$`, checked
placed each alone in each zone it affects, and counted the card/zone pairs
where the card granted itself a play permission: 45 before the fix, 0 after.
A control group (the 32 cards whose self-grant does name `EffectZone$`) had
to stay identical, and did. Write such a
*census* as a temporary test that walks `testutil.CorpusRegistry(t)`. Put the
numbers in the commit message. A census worth keeping becomes a committed
test with a per-card ratchet.

Then check that the existing ratchets agree (see [Testing](testing.md)):

- `TestEveryRepoDeckIsFullySupported`: if your fix makes a card in
  `engine/rules/testdata/known-unsupported/` supported, delete its file.
- `TestEveryRepoDeckParamsAreRead`: same, for
  `engine/rules/testdata/paramcensus/`.
- `TestHeads`: a moved chain head means the golden bot games now play
  differently. Re-pin it only if that is intended, and name the cause.

## 6. Gates and the PR

Run the [PR checklist](index.md#commits-and-pull-requests):
gofmt, `go vet` on the touched packages, `go run ./cmd/gentypes -check` if
`server/protocol` changed, the focused tests (with `.cards/` present), `make
sim`, and `TestHeads`. Then write the commit message the way `git show
786510b3f` does: the rule and its CR number, what was wrong and where, what
you measured, and the gates you ran. Open the pull request against
`develop`.
