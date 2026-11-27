package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Autopay planning of "sacrifice another permanent" mana abilities (the
// Ashnod's Altar family): the planner uses an other-sacrifice source only as
// a last resort (never when non-sacrifice sources pay), picks its victims by
// the fixed ranking (tokens first, then lowest mana value, ties by object
// id), carries them in the witness, and the executor answers the sacrifice
// ask internally from the plan's picks.

const (
	otherSacAltarSrc    = "Name:OtherSac Altar\nTypes:Artifact\nA:AB$ Mana | Cost$ Sac<1/Creature> | Produced$ C | Amount$ 2 | SpellDescription$ x\nOracle:x\n"
	otherSacSpellSrc    = "Name:OtherSac Spell\nManaCost:2\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"
	otherSacCreatureSrc = "Name:OtherSac Creature\nTypes:Creature\nPT:1/1\nOracle:x\n"
	otherSacTokenSrc    = "Name:OtherSac Token\nTypes:Creature\nPT:1/1\nOracle:x\n"
	otherSacWastesSrc   = "Name:OtherSac Wastes\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ x\nOracle:x\n"
)

// otherSacAction returns the spell's offered payment action, failing when no
// plan was offered at all.
func otherSacAction(t *testing.T, e *Engine, spell state.ObjID) decision.PaymentAction {
	t.Helper()
	d := ppPriority(t, e)
	d.PaymentActions = e.PaymentActionsForPriority(0, d.Seq)
	for _, a := range d.PaymentActions {
		if a.Cast.Object == spell {
			if len(a.Plans) == 0 {
				t.Fatalf("no plan offered for the cast: %+v", a)
			}
			return a
		}
	}
	t.Fatalf("no payment action for spell %d in %+v", spell, d.PaymentActions)
	return decision.PaymentAction{}
}

// TestAutopayPlansAltarSacrificeTokenFirst: an {2} instant payable only
// through the "Sacrifice a creature: Add {C}{C}" altar is planned, the step
// discloses the other-sacrifice consequence and names the token as the
// victim (tokens first beats the earlier-id nontoken creature of equal mana
// value), and accepting the plan pays and resolves the cast with the token --
// not the creature -- sacrificed.
func TestAutopayPlansAltarSacrificeTokenFirst(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 4711, otherSacSpellSrc)
	altar := onBoard(t, e, 0, otherSacAltarSrc)
	creature := onBoard(t, e, 0, otherSacCreatureSrc)
	token := putToken(t, e, 0, otherSacTokenSrc, state.ZBattlefield)

	// Preconditions: the ranking can actually discriminate. The nontoken
	// creature entered first (lower object id) and has the same mana value as
	// the token, so ONLY the tokens-first rule can pick the token.
	if e.G.Obj(creature).Zone != state.ZBattlefield || e.G.Obj(token).Zone != state.ZBattlefield {
		t.Fatalf("precondition: creature/token zones = %s/%s, want battlefield",
			e.G.Obj(creature).Zone, e.G.Obj(token).Zone)
	}
	if !(creature < token) {
		t.Fatalf("precondition: creature id %d, token id %d, want creature first", creature, token)
	}
	if e.G.Obj(creature).ManaValue != e.G.Obj(token).ManaValue {
		t.Fatalf("precondition: creature MV %d, token MV %d, want equal",
			e.G.Obj(creature).ManaValue, e.G.Obj(token).ManaValue)
	}
	if e.G.Obj(token).IsToken != true || e.G.Obj(creature).IsToken {
		t.Fatalf("precondition: token IsToken = %v, creature IsToken = %v", e.G.Obj(token).IsToken, e.G.Obj(creature).IsToken)
	}

	a := otherSacAction(t, e, spell)
	plan := a.Plans[0]
	if len(plan.Activations) != 1 || plan.Activations[0].Source != altar {
		t.Fatalf("witness %v, want the single altar step", plan.Activations)
	}
	act := plan.Activations[0]
	if act.Consequence == nil || !act.Consequence.SacrificeOthers {
		t.Fatalf("altar step consequence = %+v, want other-sacrifice disclosed", act.Consequence)
	}
	if len(act.Sacrifices) != 1 || act.Sacrifices[0] != token {
		t.Fatalf("altar step victims = %v, want the token %d", act.Sacrifices, token)
	}
	if plan.Activations[0].Produces[state.MC] != 2 {
		t.Fatalf("altar step produces %v, want two colorless", plan.Activations[0].Produces)
	}

	// Accept: the plan pays without a further ask, the spell resolves, and
	// the token -- never the creature -- is sacrificed.
	submitPayment(t, e, a)
	if z := e.G.Obj(spell).Zone; z == state.ZHand {
		t.Fatalf("spell zone = %s after Accept, want cast begun (on the stack or resolved)", z)
	}
	for i := 0; i < 20 && e.G.Obj(spell).Zone == state.ZStack; i++ {
		d := e.Pending()
		switch {
		case d == nil:
			e.Advance()
		case d.Kind == decision.KPriority:
			passPriorityOnce(t, e)
		default:
			t.Fatalf("unexpected ask while the planned cast resolves: %+v", d)
		}
	}
	if z := e.G.Obj(token).Zone; z != state.ZGraveyard {
		t.Fatalf("token zone = %s, want graveyard", z)
	}
	if z := e.G.Obj(creature).Zone; z != state.ZBattlefield {
		t.Fatalf("creature zone = %s, want still on the battlefield", z)
	}
	if z := e.G.Obj(spell).Zone; z != state.ZGraveyard {
		t.Fatalf("spell zone = %s, want graveyard (cast paid and resolved)", z)
	}
}

// TestAutopaySkipsAltarWhenLandsPay: with two untapped {C} sources on the
// board the same {2} cast is planned from the lands alone -- the altar is
// never a planned step, so an other-sacrifice source funds a plan only when
// non-sacrifice sources cannot pay.
func TestAutopaySkipsAltarWhenLandsPay(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 4712, otherSacSpellSrc)
	altar := onBoard(t, e, 0, otherSacAltarSrc)
	onBoard(t, e, 0, otherSacCreatureSrc)
	putToken(t, e, 0, otherSacTokenSrc, state.ZBattlefield)
	var lands []state.ObjID
	for _, src := range []string{otherSacWastesSrc, otherSacWastesSrc} {
		id := onBoard(t, e, 0, src)
		e.G.Obj(id).SummonSick = false
		lands = append(lands, id)
	}

	a := otherSacAction(t, e, spell)
	plan := a.Plans[0]
	for _, act := range plan.Activations {
		if act.Source == altar {
			t.Fatalf("the altar funds a plan although two untapped {C} sources pay: %v", plan.Activations)
		}
		if act.Consequence != nil && act.Consequence.SacrificeOthers {
			t.Fatalf("a planned step discloses an other-sacrifice: %v", plan.Activations)
		}
	}
	if len(plan.Activations) != 2 {
		t.Fatalf("witness %v, want the two lands", plan.Activations)
	}
	for _, act := range plan.Activations {
		if act.Source != lands[0] && act.Source != lands[1] {
			t.Fatalf("step source %d is not one of the lands %v", act.Source, lands)
		}
	}
}
