package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/cost"
	pay "github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// An indeterminate-Amount$ mana ability prices its potential production
// unbounded in exactly the faces its Produced$ names (CR 107.4a): it never
// claims a colour it cannot make, so a trigger's own optional cost of that
// other colour keeps its decline-only ask (CR 118.3; tail-67 pair 87, where
// a Fountain-of-Cho-style {W}-per-counter battery made a Frillback's "you
// may pay {G}" look payable).
const fixedFacesCradleSrc = "Name:Fixture Cradle\n" +
	"ManaCost:no cost\n" +
	"Types:Land\n" +
	"A:AB$ Mana | Cost$ T | Produced$ G | Amount$ Count$CreaturesYouControl | SpellDescription$ Add {G} for each creature you control.\n" +
	"Oracle:{T}: Add {G} for each creature you control.\n"

// TestPotentialFixedFacesOnly: an indeterminate amount is unbounded in the
// produced faces alone. The storage battery (Amount$ X, Produced$ B) and the
// Count$ land (Produced$ G) both claim their one face unbounded and no other
// colour, and the trigger-cost gate (resPayNeedsMana) offers pay only for the
// colour they make.
func TestPotentialFixedFacesOnly(t *testing.T) {
	t.Parallel()

	oneBlack := cost.Cost{Colored: state.Mana{state.MB: 1}}
	oneGreen := cost.Cost{Colored: state.Mana{state.MG: 1}}

	storageSetup := func(t *testing.T) (*Engine, state.ObjID) {
		t.Helper()
		e, _, _ := newFixtureDeck(t, 6311, storageXSpellFixtureSrc)
		land := onBoard(t, e, 0, storageXLandFixtureSrc)
		putCountersOn(t, e, 0, "Fixture Storage Vault", "STORAGE", 2)
		driveToStep(t, e, 1, 0, state.StepMain1)
		return e, land
	}
	e, land := storageSetup(t)
	battery := e.PotentialMana(0)
	if battery[state.ManaIndex('B')] != potentialUnbounded {
		t.Fatalf("the battery's {B} potential = %d, want unbounded", battery[state.ManaIndex('B')])
	}
	if battery[state.ManaIndex('R')] != 0 {
		t.Fatalf("the battery claims {R} = %d, want 0 (it produces only {B})", battery[state.ManaIndex('R')])
	}
	if !e.resPayNeedsMana(0, land, oneBlack, pay.PurposeOther) {
		t.Fatal("the trigger-cost gate does not offer pay for the battery's own {B}")
	}
	if e.resPayNeedsMana(0, land, oneGreen, pay.PurposeOther) {
		t.Fatal("the trigger-cost gate offers pay for a {G} the battery cannot make")
	}

	e.G.Obj(land).Tapped = true // the battery stays out of the second read
	onBoard(t, e, 0, fixedFacesCradleSrc)
	cradlePot := e.PotentialMana(0)
	if cradlePot[state.ManaIndex('G')] != potentialUnbounded {
		t.Fatalf("the Count$ land's {G} potential = %d, want unbounded", cradlePot[state.ManaIndex('G')])
	}
	if cradlePot[state.ManaIndex('R')] != 0 {
		t.Fatalf("the Count$ land claims {R} = %d, want 0 (it produces only {G})", cradlePot[state.ManaIndex('R')])
	}
	if o := e.G.Obj(land); o == nil || !o.Tapped {
		t.Fatalf("fixture: the battery is %+v, want tapped out of the second read", o)
	}
}
