# definedrem3 — `Defined$ Remembered` player-selection leak

Ticket `agent-20260922T160116Z-ca8c201c`. Status: DONE.

## What changed and why

The plain `Remembered` family must contribute remembered **players** only. A
remembered **card** contributes its controller/owner solely for the
`RememberedController`/`RememberedOwner` spellings (Forge
`AbilityUtils.getDefinedPlayers`/`addPlayer`). `Defined` deliberately still
returns the raw mixed set — object effects (`ChangeZone`, `Destroy`, …) read
the remembered CARDS — but every **player-selection** site mapped that set
with `PlayerOf`, turning a remembered card into its controller. In a
`RepeatEach` loop whose `Ctx.Remembered` holds a previous iteration's
`RememberChosen$` card beside the current subject (Summon: Valefor's Sonic
Wings chapter), iteration 2 therefore re-offered the previous opponent.

### One structural home (`effects/context.go`)

Added next to `plainRememberedSelector` (the existing predicate for "plain
Remembered family"):

- `playerForTarget(h, c, selector, t) (PlayerID, bool)` — drops a non-player
  target for the plain family; otherwise `PlayerOf` (unchanged contract).
- `playerIDsFromTargets(h, c, selector, ts) []PlayerID` — dedup + bound check,
  first-seen order, `seen` map is a membership test only (no map range reaches
  output order).
- `definedPlayerIDs(h, c, selector)` — resolves through a temp SA keyed on the
  selector **spelling** (the `searchPlayers` trick), applying the rule above.
- `definedPlayers(h, c, sa)` — the SA-level spelling; resolves `Defined`
  through the full SA so a `ValidTgts$` fallback still applies.

Every migrated site goes through one of these, so the next sibling cannot miss
the rule unless it bypasses the helper.

### Migrated sites

| file | site | mechanism |
|---|---|---|
| `effects/context.go` | new helper + predicate | — |
| `effects/zone.go` | `searchPlayers` | `definedPlayerIDs` (its private `RememberedController` special case removed, subsumed by `Defined`) |
| `effects/zone.go` | `handMoveOwners` plain branch | `definedPlayers` (previously failed closed, dropping a legitimate remembered player when a remembered card coexisted) |
| `effects/zone.go` | `chooserPlayer` (hidden-hand `Chooser$`) | plain-guard — **not in the brief's table**, see Deviations |
| `effects/vote.go` | `effPlayerVote` | `voters := definedPlayers`; second walk reads `PlayerID` |
| `effects/misc.go` | `effVote`, `effCardVote`, `askFixedVote`, `askCardVote` | `voters := definedPlayers`; ask helpers now take `[]state.PlayerID` |
| `effects/misc.go` | `ManaRecipients`, `changeTargetChooser` (in `choose_control.go`) | helper / `definedPlayerIDs` |
| `effects/cardflow.go` | `actingPlayers` → `[]state.PlayerID` + **all** callers (Draw/Discard/Mill/Rearrange/Surveil) | `definedPlayers`; the `ValidTgts` branch maps via `playerIDsFromTargets("", …)` |
| `effects/cardflow.go` | `effDig`, `effDigUntil`, `effReveal` | `definedPlayers` / `playerIDsFromTargets`; `effReveal` keeps `RevealDefined` object handling behind a plain-guard |
| `effects/life.go`, `effects/investigate.go` | `effGainLife`, `effExchangeLifeVariant`, `effLoseLife`, `effInvestigate` | via `actingPlayers` |
| `effects/choose_control.go` | `repeatPlayers` | `playerIDsFromTargets` per case — **not in the brief's table**, see Deviations |
| `effects/combatfx.go` | `allPlayersFor` (mass tap/untap/damage-all) | `definedPlayers` — **not in the brief's table**, see Deviations |
| `effects/trigger_referents.go` | `controlReferent` + `controlReferentPlayers` | added `RememberedController`/`RememberedOwner`, resolution-only; tail maps players directly and objects by `op` (`OwnedBy→Owner`, `ControlledBy→Controller`) |

`Defined` itself was **not** changed (out of scope, correct as-is).

## Gates (real output)

```
$ go test -run 'TestDefinedPlayers|TestSearchPlayersPlainRemembered|TestVoteVotersPlainRemembered|TestActingPlayersPlainRemembered|TestChangeTargetChooserPlainRemembered|TestManaRecipientsPlainRemembered|TestControlledByRemembered|TestTriggerReferent|TestPutCounterControlledByRememberedPlayerBridge|TestHandMoveOwnersPlainRemembered|TestChooserPlayerPlainRemembered|TestAllPlayersForPlainRemembered|TestRememberedControllerOwnerControlReferents' ./effects/
ok  	github.com/adams-shaun/gorge/effects	0.037s
```
(the brief's exact command, with the three extra new tests appended to the
`-run` alternation)

```
$ go test -run 'TestSummonValefor|TestGreatestCMCControlledByRemembered' ./rules/
ok  	github.com/adams-shaun/gorge/rules	(cached)
```

Verbose confirmation the nine new tests actually run (not a vacuous
`ok`): `errors: 9 × --- PASS`, including
`TestRememberedControllerOwnerControlReferents` and every
`…PlainRememberedExcludesCardControllers`. Full `./effects/` package:

```
$ go test ./effects/
ok  	github.com/adams-shaun/gorge/effects	2.570s
```

```
$ go test ./internal/archtest/          → ok  ... 4.134s   (run once; later cached)
$ go test -run TestConstructedDefaultIsByteIdentical ./cmd/botbench/
ok  	github.com/adams-shaun/gorge/cmd/botbench	1.269s
$ gofmt -l <changed files>              → (empty)
$ go run ./cmd/gentypes -check          → (empty)
$ go vet ./effects/                     → (clean)
```

**Botbench did NOT move** — the pinned 20-game split is unchanged, so no
re-pin/attribution was needed. `.cards` was present (symlink to
`/home/sadams/projects/gorge/.cards`), so the corpus-backed tests ran rather
than skipping.

Corpus census re-measured (brief's numbers held; only the *report it quotes*
was wrong):

```
plain "Defined$ Remembered" (not Controller/Owner): 1764
Defined$ RememberedController: 34   Defined$ RememberedOwner: 9
ControlledBy/OwnedBy RememberedController/Owner carriers: 3
```

## Fails without the fix

Reverted the plain-Remembered filter in `playerForTarget` **and** the two new
cases in `controlReferent`/`controlReferentPlayers` (backed up to
`.ds4/scratch/revert/`, restored byte-identically via `cmp`):

```
--- FAIL: TestDefinedPlayersRememberedFamilyIsPlayersOnly (0.00s)
    defined_players_test.go:72: definedPlayerIDs(Remembered) = [1 2], want [2]
--- FAIL: TestSearchPlayersPlainRememberedExcludesCardControllers (0.00s)
    defined_players_test.go:93: searchPlayers(Remembered) = [1 2], want [2]
--- FAIL: TestActingPlayersPlainRememberedExcludesCardControllers (0.00s)
    defined_players_test.go:105: actingPlayers(Remembered) = [1 2], want [2]
--- FAIL: TestVoteVotersPlainRememberedExcludesCardControllers (0.00s)
    defined_players_test.go:119: effVote voters = 2 notes, want 1 (only remembered player 2)
--- FAIL: TestChangeTargetChooserPlainRememberedExcludesCardControllers (0.00s)
    defined_players_test.go:149: changeTargetChooser(Chooser$ Remembered) = 1, want 2
--- FAIL: TestManaRecipientsPlainRememberedExcludesCardControllers (0.00s)
    defined_players_test.go:162: ManaRecipients(Remembered) = [1 2], want [2]
--- FAIL: TestRememberedControllerOwnerControlReferents (0.00s)
    defined_players_test.go:192: the remembered card's controller's creature did not match ControlledBy RememberedController
FAIL	github.com/adams-shaun/gorge/effects	0.019s
```

`handMoveOwners` (reverted only the `playerForTarget` filter):

```
--- FAIL: TestHandMoveOwnersPlainRememberedKeepsRememberedPlayer (0.00s)
    defined_players_test.go:176: handMoveOwners(Remembered) = [1 2], want [2]
```

`allPlayersFor` (reverted its own hunk only):

```
--- FAIL: TestAllPlayersForPlainRememberedExcludesCardControllers (0.00s)
    defined_players_test.go:171: allPlayersFor(Remembered) = [1 2], want [2]
```

`chooserPlayer` (reverted its own hunk only):

```
--- FAIL: TestChooserPlayerPlainRememberedExcludesCardControllers (0.00s)
    defined_players_test.go:172: chooserPlayer(Remembered) = 1,true, want 2,true
```

Every new test asserts its precondition explicitly (`mixedRememberedHost`
checks the remembered card exists, maps to seat 1, the remembered player is
seat 2, and that 0/1/2 are distinct; the referent test checks the two
creatures have different controllers and the remembered card's controller
matches one of them).

## Deviations from the brief

1. **Three sites closed that the brief's table did not list** (all the same
   class, all in `effects/`, none touching `rules/` or `web/`):
   - `repeatPlayers` (`choose_control.go`) — the `RepeatEach` player loop,
     `"Remembered"`/`"RememberedController"` cases now selector-aware.
   - `allPlayersFor` (`combatfx.go`) — mass tap/untap/damage-all player walk.
   - `chooserPlayer` (`zone.go`) — the hidden-hand `Chooser$` resolver; the
     brief listed only `changeTargetChooser`, but `chooserPlayer` is the more
     widely used `Chooser$` reader and has 7 corpus `Chooser$ Remembered`
     carriers. Leaving it would be "patch the instance, not the class".
   These are the structural-fix direction the brief itself asks for; each has
   its own leaf test and fail-before-fix proof above. Blast radius: all are
   `effects`-local and exercised by the full `./effects/` package (green) and
   `cmd/botbench` (split unchanged).
2. The `RememberedPlayer` spelling was **not** asserted in the helper test:
   the engine's `Defined` does not resolve `RememberedPlayer` (only the
   `RememberedPlayerCtrl` *predicate* exists), so `definedPlayerIDs(…,
   "RememberedPlayer")` is `[]` both before and after — there is no leak to
   fix there. The helper still applies the plain rule to any
   `Remembered…`-prefixed, non-`Controller`/`Owner` selector.

No AGENTS.md "Known approximations" row covered this leak, so none was deleted
and `knownApproximationRows` stays 21. No `web/` change.

## Commits

```
27719e7f fix(effects): `Defined$ Remembered` contributes players only to player-selection sites
ea4f57c1 fix(effects): close the allPlayersFor mass-effect player walk to the plain-Remembered rule
8f25837a fix(effects): chooserPlayer honours the plain-Remembered rule
91682582 test(effects): pin handMoveOwners keeps a remembered player in a mixed set
```

## Issues (found, not fixed)

Filed as `.ds4/new-tickets/defined-remembered-remaining-player-walks.md`.
Three same-class sites still map a selector → `PlayerOf` without the plain
guard; each is a small follow-up with measured corpus prevalence:

1. `Tapper$` reads — `effects/combatfx.go:195` (`effTap`) and
   `effects/taporuntap.go:47` (`effTapOrUntap`). `Tapper$ Remembered` = 1
   corpus file.
2. `Controller$` read — `effects/play.go:279` (`effPlay`).
   `Controller$ Remembered` = **14** corpus files.
3. `effBecomeMonarch` first-target read — `effects/misc.go:4218`.
   `BecomeMonarch` + `Defined$ Remembered` = 1 corpus file.

Verified **not** leaks (left alone, per the brief and my own check):
`counters.go` (`IsPlayer`/`!IsPlayer` guards), `damage.go validPlayers`
(reads `c.Targets`), `sacrificeall.go` (reads `c.Targets`), `zone.go
chooserChosenPlayer` (players-only guard), `zone.go
handMoveOwners`/`hiddenPickPlayers` targeted branches (players-only guard),
`flipperPlayers`, `effectRememberedPlayers`.

No CR-lane test suggestion: this is a `getDefinedPlayers` filter-bridge
semantic, already covered by the effects leaf tests above rather than a CR
rule; no new conformance test is warranted.
