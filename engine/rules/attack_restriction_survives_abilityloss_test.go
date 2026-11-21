package rules

// combatres-cantattack-vs-abilityloss: an attack/block restriction or
// requirement GRANTED by an effect ("Target creature can't attack or block
// this turn", Alchemist's Vial's activation, Off Balance) is a game-rule
// continuous effect, not an ability of the object (CR 613.11 names the
// class: "say that a creature must attack this turn if able" -- applied after
// all other continuous effects). A LATER "loses all abilities" (CR 613.1f
// layer-6 removal, Snakeform's Animate RemoveAllAbilities$ delivery) cannot
// end it: the restriction is not one of the abilities it removes. The
// kernel's override record keeps exactly this, and the lockstep pair in
// dirac/oracle/testdata/pump_cantattack_abilityloss.repro.json diverged on it:
// gorge wiped the restriction with the removal and offered the attack.
//
// Both leaves drive real corpus cards through the whole cast pipeline (the
// same one a game runs): Off Balance resolves its Pump `KW$ HIDDEN CARDNAME
// can't attack or block.` grant, then Snakeform resolves its Animate
// RemoveAllAbilities$ on the same creature. A granted ABILITY line (the
// control leaf) must still die to the removal, so the fix cannot simply
// preserve every granted keyword.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// castSpellPaying casts one corpus spell from seat 0's hand, answering the
// payment asks a mixed cost poses (Snakeform's {2}{G}{U}: the generic part
// is paid from a coloured pool one symbol at a time), then submits the
// creature target and drains the stack. castCorpusSpell cannot be reused for
// Snakeform because its passToTargetAsk refuses a payment ask.
func castSpellPaying(t *testing.T, e *Engine, name, symbols string, target state.ObjID) {
	t.Helper()
	ensureInHand(t, e, 0, name)
	addMana(t, e, 0, symbols)
	castCardNow(t, e, name)
	for i := 0; i < 24; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no pending decision")
		}
		switch d.Kind {
		case decision.KTarget:
			idx := -1
			for _, o := range d.Options {
				if o.Obj == target {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("%s target %d not offered: %+v", name, target, d.Options)
			}
			submitChoices(t, e, idx)
			passUntilStackEmpty(t, e, 40)
			return
		case decision.KChoose:
			submitChoices(t, e, 0) // the payment ask is Min1/Max1: pay the first offered symbol
		case decision.KPriority:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("no pass option on the priority ask: %+v", d.Options)
			}
			submitChoices(t, e, idx)
		default:
			t.Fatalf("unexpected decision kind while casting %s: %+v", name, d)
		}
	}
	t.Fatalf("%s never reached its target ask", name)
}

// restrictionSurvivalEngine seats Off Balance, Snakeform and Vines of the
// Recluse in seat 0's deck beside a vanilla bear, so every cast below is a
// real corpus cast the same way a game runs one.
func restrictionSurvivalEngine(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg,
		[]*cards.Card{
			lookup(t, reg, "Off Balance"),
			lookup(t, reg, "Snakeform"),
			lookup(t, reg, "Vines of the Recluse"),
			card(t, "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"),
		},
		[]*cards.Card{card(t, "Name:Test SeatOne Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")})
	return e, moveByName(t, e, 0, "Test Bear", state.ZBattlefield)
}

// TestPumpCantAttackGrantSurvivesLoseAllAbilities: Off Balance's can't-attack
// restriction stands after Snakeform removes all abilities from the same
// creature.
func TestPumpCantAttackGrantSurvivesLoseAllAbilities(t *testing.T) {
	t.Parallel()
	e, attacker := restrictionSurvivalEngine(t)
	if o := e.G.Obj(attacker); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: the attacker is not on the battlefield")
	} else {
		o.SummonSick = false
	}
	if e.attackBlocked(attacker, 1, 0) {
		t.Fatal("precondition: the vanilla creature cannot attack although nothing restricts it")
	}
	// Off Balance grants the restriction: the attack is now forbidden.
	castCorpusSpell(t, e, "Off Balance", "W", attacker)
	if !e.attackBlocked(attacker, 1, 0) {
		t.Fatal("precondition: Off Balance's can't-attack grant is not applying (the HIDDEN sentence is unread)")
	}
	// Snakeform removes all abilities from the same creature; the
	// restriction is a game-rule effect and must stand (CR 613.11).
	castSpellPaying(t, e, "Snakeform", "GGUU", attacker)
	if !e.attackBlocked(attacker, 1, 0) {
		t.Fatal("the can't-attack restriction ended when Snakeform removed all abilities (CR 613.11: the restriction is not an ability)")
	}
}

// TestGrantedKeywordStillDiesToLoseAllAbilities is the control: a granted
// ability line (reach) is exactly the class a layer-6 removal removes, so
// the restriction survival must not extend to it.
func TestGrantedKeywordStillDiesToLoseAllAbilities(t *testing.T) {
	t.Parallel()
	e, flyer := restrictionSurvivalEngine(t)
	if o := e.G.Obj(flyer); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: the creature is not on the battlefield")
	} else {
		o.SummonSick = false
	}
	// Vines of the Recluse is the corpus's keyword pump carrier (gains
	// reach, no ask beside the target), so a rename is one constant.
	castCorpusSpell(t, e, "Vines of the Recluse", "G", flyer)
	if !e.HasKeyword(flyer, "Reach") {
		t.Fatalf("precondition: the reach grant is not applying: %v", e.Derived(flyer).Keywords)
	}
	castSpellPaying(t, e, "Snakeform", "GGUU", flyer)
	if e.HasKeyword(flyer, "Reach") {
		t.Fatal("the granted reach survived the ability removal (the removal must still clear granted abilities)")
	}
}
