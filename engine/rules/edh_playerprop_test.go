package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The PlayerCount<Group>$<Property> heads of the you/defined/remembered
// player (the EDH deck-census class): each test plays the printed text of the
// card through a real game and reads the count through the evaluator the
// card's own SVar goes through.

func edhCount(e *Engine, src state.ObjID, who state.PlayerID, svars map[string]string, body string) (int32, bool) {
	return effects.EvalCountOK(e, &effects.Ctx{Source: src, Controller: who, SVars: svars}, body)
}

// edhDrive passes priority until the game reaches (turn, active, step),
// answering a declare-attackers ask by attacking with the first option when
// attack is true (otherwise declining), declaring no blockers, and taking
// naive cleanup discards -- so the combat damage it produces is real.
func edhDrive(t *testing.T, e *Engine, turn int32, active state.PlayerID, step state.Step, attack bool) {
	t.Helper()
	for range 4000 {
		if e.G.Turn == turn && e.G.Active == active && e.G.Step == step {
			return
		}
		if e.G.Over {
			t.Fatalf("game ended before turn %d seat %d step %s", turn, active, step)
		}
		if answerIfDiscard(t, e) {
			continue
		}
		d := e.Pending()
		if d == nil {
			t.Fatalf("nil decision driving to turn %d seat %d step %s", turn, active, step)
		}
		switch d.Kind {
		case decision.KPriority:
			passOnce(t, e)
		case decision.KAttackers:
			if attack {
				submitChoices(t, e, d.Options[0].Index)
			} else {
				submitChoices(t, e)
			}
		case decision.KBlockers:
			submitChoices(t, e)
		default:
			t.Fatalf("unexpected %s decision driving to turn %d: %+v", d.Kind, turn, d)
		}
	}
	t.Fatalf("did not reach turn %d seat %d step %s", turn, active, step)
}

// Master of Waves: cast with one other blue permanent out, the ETB makes
// devotion-to-blue Elemental tokens, and the "Valid" property head counts the
// resolving controller's own Elementals (all / token-only), the AI SVars'
// /Times.N suffix applied, and none for the other seat.
func TestEDHPlayerPropertyMasterOfWavesCountsElementals(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{
		lookup(t, reg, "Master of Waves"),
		lookup(t, reg, "Merfolk of the Pearl Trident"),
	}, nil)
	moveByName(t, e, 0, "Merfolk of the Pearl Trident", state.ZBattlefield)
	mow := moveByName(t, e, 0, "Master of Waves", state.ZHand)
	addMana(t, e, 0, "UUUU")
	castObj(t, e, mow)
	passUntilStackEmpty(t, e, 40)

	// Precondition: Master of Waves resolved and made its devotion's worth
	// of Elementals (its own {U} + the Merfolk's {U}).
	tokens := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken {
			tokens++
		}
	}
	if tokens != 2 {
		t.Fatalf("precondition: want 2 Elemental tokens from devotion 2, got %d", tokens)
	}
	svars := lookup(t, reg, "Master of Waves").Faces[0].SVars
	for body, want := range map[string]int32{
		"PlayerCountPropertyYou$Valid Elemental.YouCtrl":                2,
		"PlayerCountPropertyYou$Valid Elemental.token+YouCtrl":          2,
		"PlayerCountPropertyYou$Valid Elemental.YouCtrl/Times.25":       50,
		"PlayerCountPropertyYou$Valid Elemental.token+YouCtrl/Times.15": 30,
		"PlayerCountPropertyYou$Valid Creature.Merfolk+YouCtrl":         2,
	} {
		got, ok := edhCount(e, mow, 0, svars, body)
		if !ok || got != want {
			t.Errorf("%s = %d (ok %v), want %d", body, got, ok, want)
		}
	}
	if got, ok := edhCount(e, mow, 1, svars, "PlayerCountPropertyYou$Valid Elemental.YouCtrl"); !ok || got != 0 {
		t.Errorf("the other seat counted %d (ok %v) Elementals, want 0", got, ok)
	}
}

// marchesaWatcherSrc is a fixture with the same upkeep rule as Marchesa,
// Resolute Monarch's (the back face of Invasion of Fiora), on a plain body,
// so a real game drives it. Written for this test: the gate SVar is named Hit.
const marchesaWatcherSrc = "Name:Fiora Watcher\nManaCost:2 B\nTypes:Creature Human Noble\nPT:1/1\n" +
	"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ Refill | CheckSVar$ Hit | SVarCompare$ EQ0 | TriggerDescription$ Upkeep: if you were not dealt combat damage since your last turn, draw a card and lose 1 life.\n" +
	"SVar:Refill:DB$ Draw | SubAbility$ Pay\n" +
	"SVar:Pay:DB$ LoseLife | LifeAmount$ 1\n" +
	"SVar:Hit:PlayerCountPropertyYou$BeenDealtCombatDamageSinceLastTurn\n" +
	"Oracle:x\n"

const marchesaOppBearSrc = "Name:Fiora Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// Invasion of Fiora (Marchesa's upkeep gate): the count reads 1 only after
// the seat was dealt COMBAT damage since its last turn ended; noncombat
// damage never counts, and the window restarts when the seat's turn ends.
func TestEDHPlayerPropertyBeenDealtCombatDamageSinceLastTurn(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		combat     bool
		wantLife3  int32 // seat 0's life at turn 3 main (after the turn-3 upkeep)
		wantLife5  int32
		wantCount3 int32
	}{
		// Combat damage in turn 2: turn 3's upkeep sees it (no draw, no
		// loss); turn 5's window is clean (draw + lose 1).
		{"combat damage blocks the gate", true, 18, 17, 1},
		// Noncombat damage is not combat damage: the gate opens at turn 3.
		{"noncombat damage does not", false, 17, 16, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, cfg, _ := newFixtureDeckWithOpponentCard(t, 77, marchesaWatcherSrc, marchesaOppBearSrc, marchesaOppBearSrc)
			watcher := putCreature(t, e, 0, marchesaWatcherSrc)
			bear := moveSeeded(t, e, 1, marchesaOppBearSrc, state.ZBattlefield)
			e.priorityRound()
			svars := card(t, marchesaWatcherSrc).Faces[0].SVars
			const body = "PlayerCountPropertyYou$BeenDealtCombatDamageSinceLastTurn"

			if got, ok := edhCount(e, watcher, 0, svars, body); !ok || got != 0 {
				t.Fatalf("precondition: count before any damage = %d (ok %v), want 0", got, ok)
			}
			if tc.combat {
				// Seat 1 really attacks: the bear's combat damage lands in
				// turn 2's combat damage step.
				edhDrive(t, e, 2, 1, state.StepMain2, true)
			} else {
				edhDrive(t, e, 2, 1, state.StepMain2, false)
				burn(e, bear, "Opponent", 2)
			}
			if e.G.Players[0].Life != 18 {
				t.Fatalf("precondition: the hit did not land (life %d)", e.G.Players[0].Life)
			}
			if got, _ := edhCount(e, watcher, 0, svars, body); got != tc.wantCount3 {
				t.Fatalf("count in the turn after the hit = %d, want %d", got, tc.wantCount3)
			}
			if got, _ := edhCount(e, watcher, 1, svars, body); got != 0 {
				t.Fatalf("seat 1 reads %d, it was not dealt damage", got)
			}
			edhDrive(t, e, 3, 0, state.StepMain1, false)
			if got := e.G.Players[0].Life; got != tc.wantLife3 {
				t.Fatalf("turn 3: life %d, want %d", got, tc.wantLife3)
			}
			if got, _ := edhCount(e, watcher, 0, svars, body); got != tc.wantCount3 {
				t.Fatalf("turn 3: count %d, want %d (damage taken since the last turn ended)", got, tc.wantCount3)
			}
			edhDrive(t, e, 5, 0, state.StepMain1, false)
			if got := e.G.Players[0].Life; got != tc.wantLife5 {
				t.Fatalf("turn 5: life %d, want %d (a clean window opens the trigger)", got, tc.wantLife5)
			}
			if got, _ := edhCount(e, watcher, 0, svars, body); got != 0 {
				t.Fatalf("turn 5: count %d, want 0 after a damage-free window", got)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// Simulacrum / Tainted Sigil: the damage dealt to you, and the life lost by
// all players, this turn -- read after real damage lands.
func TestEDHPlayerPropertyDamageAndLifeLostThisTurn(t *testing.T) {
	t.Parallel()
	e, ids := pcdrEngine(t, "Grizzly Bears")
	bears := ids["Grizzly Bears"]
	svars := map[string]string{}
	count := func(who state.PlayerID, body string) int32 {
		t.Helper()
		n, ok := edhCount(e, bears, who, svars, body)
		if !ok {
			t.Fatalf("%s unresolvable", body)
		}
		return n
	}
	const dmg, lost = "PlayerCountPropertyYou$DamageThisTurn", "PlayerCountPlayers$LifeLostThisTurn"
	if count(0, dmg) != 0 || count(0, lost) != 0 {
		t.Fatal("precondition: nonzero before any damage")
	}
	burn(e, bears, "You", 3)
	burn(e, bears, "Opponent", 2)
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -4})
	if got := count(0, dmg); got != 3 {
		t.Errorf("seat 0 damage this turn = %d, want 3", got)
	}
	if got := count(1, dmg); got != 2 {
		t.Errorf("seat 1 damage this turn = %d, want 2", got)
	}
	if got := count(0, lost); got != 3+2+4 {
		t.Errorf("total life lost this turn = %d, want 9", got)
	}
}

// Celestial Convergence's tie check: players holding the highest life total.
func TestEDHPlayerPropertyTiedForHighestLife(t *testing.T) {
	t.Parallel()
	e, ids := pcdrEngine(t, "Grizzly Bears")
	bears := ids["Grizzly Bears"]
	const body = "PlayerCountPlayers$TiedForHighestLife"
	got, ok := edhCount(e, bears, 0, nil, body)
	if !ok || got != 2 {
		t.Fatalf("both at 20: %d (ok %v), want 2", got, ok)
	}
	burn(e, bears, "Opponent", 1)
	if got, _ := edhCount(e, bears, 0, nil, body); got != 1 {
		t.Fatalf("seat 0 alone highest: %d, want 1", got)
	}
}

// Garland, Royal Kidnapper: when it enters an opponent becomes the monarch
// and the Effect it creates reads X as PlayerCountRememberedPlayer$
// HasPropertyisMonarch over the remembered monarch. The count follows the
// monarchy, and the Effect's `Mode$ Always` ExileMe trigger (CheckSVar$ X
// EQ0) -- which reads the same count through its registration's own captured
// set -- neither livelocks while the opponent holds the monarchy nor once it
// moves away.
func TestEDHPlayerPropertyGarlandCountFollowsMonarch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Garland, Royal Kidnapper")},
		[]*cards.Card{lookup(t, reg, "Grizzly Bears")})
	moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	garland := moveByName(t, e, 0, "Garland, Royal Kidnapper", state.ZHand)
	addMana(t, e, 0, "UUBB")
	cast := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == garland {
			cast = o.Index
		}
	}
	if cast < 0 {
		t.Fatalf("Garland is not castable: %+v", e.Pending().Options)
	}
	submitChoices(t, e, cast)
	// Resolve the spell and both triggers, choosing the first target each
	// time, until nothing is on the stack.
	for range 60 {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			passOnce(t, e)
			continue
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	if e.G.Obj(garland).Zone != state.ZBattlefield || !e.G.IsMonarch(1) {
		t.Fatalf("precondition: Garland on battlefield %v, seat 1 monarch %v",
			e.G.Obj(garland).Zone, e.G.IsMonarch(1))
	}
	svars := lookup(t, reg, "Garland, Royal Kidnapper").Faces[0].SVars
	const body = "PlayerCountRememberedPlayer$HasPropertyisMonarch"
	count := func() int32 {
		ctx := &effects.Ctx{Source: garland, Controller: 0, SVars: svars,
			Remembered: []state.Target{{IsPlayer: true, Player: 1}}}
		n, ok := effects.EvalCountOK(e, ctx, body)
		if !ok {
			t.Fatalf("%s unresolvable", body)
		}
		return n
	}
	if got := count(); got != 1 {
		t.Fatalf("remembered opponent is the monarch: count %d, want 1", got)
	}
	// The monarchy moves to Garland's controller.
	e.emit(events.Event{Kind: events.MonarchChange, Player: 0})
	if got := count(); got != 0 {
		t.Fatalf("remembered opponent lost the monarchy: count %d, want 0", got)
	}
	// Play on past the change (a livelocking ExileMe would panic here).
	for range 30 {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			break
		}
		passOnce(t, e)
	}
	replayCheck(t, e, cfg)
}
