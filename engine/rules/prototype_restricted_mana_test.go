package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// prototypeRestrictedSrc is a colourless printed artifact creature whose
// prototype cost is blue (CR 718.3a/718.3b: the prototyped spell is blue).
const prototypeRestrictedSrc = `Name:Proto Restricted
ManaCost:6
Types:Artifact Creature
PT:5/5
K:Prototype:1 U U:3:3
Oracle:x
`

// filterRockSrc is the inline twin of Sage of the Unknowable's mana ability:
// {C} restricted to colourless spells (engine/rules legal, RestrictValid$).
const filterRockSrc = `Name:Proto Filter Rock
ManaCost:2
Types:Artifact
A:AB$ Mana | Produced$ C | Amount$ 1 | Cost$ T | RestrictValid$ Spell.Colorless,Activated | SpellDescription$ Add one colourless mana.
Oracle:x
`

const protoIslandSrc = "Name:Proto Island\nTypes:Basic Land Island\nOracle:x\n"

// protoOffer builds the prototype card in hand over the given untapped
// sources and reports whether the autopay surface lists the prototype cast
// with a payment plan (the same potential walk the oracle prices).
func protoOffer(t *testing.T, seed uint64, sources ...string) bool {
	t.Helper()
	e, _, id := newFixtureDeck(t, seed, prototypeRestrictedSrc)
	addToHand(t, e, 0, prototypeRestrictedSrc)
	for _, src := range sources {
		onBoard(t, e, 0, src)
	}
	e.priorityRound()
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision: %+v", d)
	}
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Obj == id && pp.Action.Mode == "prototype" && pp.Reason == "" {
			return true
		}
	}
	return false
}

// TestPrototypeRestrictedManaReadsInsetColour: the {C} from "Spend this mana
// only to cast a colourless spell" cannot pay for a prototyped cast, because
// CR 718.3a/718.3b gives the spell its prototype face's blue colour. The
// offer must price that restriction against the prototype inset, not the
// colourless printed face.
func TestPrototypeRestrictedManaReadsInsetColour(t *testing.T) {
	t.Parallel()
	// The same three sources pay {1}{U}{U} when the {C} is unrestricted.
	unrestricted := "Name:Proto Rock\nManaCost:2\nTypes:Artifact\n" +
		"A:AB$ Mana | Produced$ C | Amount$ 1 | Cost$ T | SpellDescription$ Add one colourless mana.\nOracle:x\n"
	if !protoOffer(t, 8201, protoIslandSrc, protoIslandSrc, unrestricted) {
		t.Fatal("precondition: the prototype cast is offered over two Islands and an unrestricted {C} source")
	}
	if protoOffer(t, 8202, protoIslandSrc, protoIslandSrc, filterRockSrc) {
		t.Fatal("the colourless-only {C} paid for a blue prototyped spell (CR 718.3a/718.3b)")
	}
}
