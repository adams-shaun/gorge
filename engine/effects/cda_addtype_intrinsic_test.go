package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestCDAAddTypeIntrinsicEveryZone pins the type filter's intrinsic AddType$
// CDA tail (filter_type.go's cdaAddTypeHas): Veteran Adventurer's
// "CARDNAME is also a Cleric, Rogue, Warrior, and Wizard" is a
// characteristic-defining ability, so CR 613.4a answers it in every zone —
// off the battlefield (in a hand, library or exile) where the rules layer
// walk publishes no derived type list, the printed-face fallback plus the
// intrinsic CDAs are the whole answer, and the printed face alone is not
// enough. The read is what a RestrictValid$ Spell.Creature+ChosenType batch
// (The Ringhart Crest's mana) evaluates the hand card with, so a miss here
// withheld a cast the restricted mana could have paid (tail-74 pair 127,
// seed 9802406336505234860, decision 498; the pinned repro is
// dirac/oracle/testdata/repro/cda_addtype.repro.json).
func TestCDAAddTypeIntrinsicEveryZone(t *testing.T) {
	va := mkCard(t, "Name:Veteran Adventurer\nManaCost:5 G\nTypes:Creature Human\nPT:5/5\n"+
		"S:Mode$ Continuous | Affected$ Card.Self | CharacteristicDefining$ True | AddType$ Cleric & Rogue & Warrior & Wizard | Description$ CARDNAME is also a Cleric, Rogue, Warrior, and Wizard.\n"+
		"S:Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | Amount$ X | EffectZone$ All | Description$ This spell costs {1} less to cast for each creature in your party.\n"+
		"Oracle:x")
	f := va.Faces[0]
	// Precondition: the printed face carries only Human, so a pass against a
	// face that already printed the CDA types proves nothing.
	if f.TypeLineHas("Rogue", 0) || f.TypeLineHas("Cleric", 0) {
		t.Fatal("precondition: printed face already carries a CDA type")
	}
	want := []string{"Cleric", "Rogue", "Warrior", "Wizard"}
	got := f.CDAAddTypes()
	if len(got) != len(want) {
		t.Fatalf("CDAAddTypes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("CDAAddTypes()[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	h := newHost(t, 2)
	o := h.g.AddObject(va, 1)
	o.Zone = state.ZHand
	for _, w := range want {
		if !hasType(o, w) {
			t.Errorf("hand card lost its characteristic-defining type %q (CR 613.4a)", w)
		}
	}
	if hasType(o, "Goblin") || hasType(o, "God") {
		t.Error("the CDA tail granted a type its AddType$ never listed")
	}

	// A conditional CDA (Karn's "During your turn ... is a 3/4 Ninja
	// creature") is fail-closed off the battlefield: the intrinsic read
	// cannot evaluate the condition, so the static stays the rules layer
	// walk's.
	cond := mkCard(t, "Name:Karn\nTypes:Planeswalker Karn\nLoyalty:5\n"+
		"S:Mode$ Continuous | AffectedDefined$ Self | Condition$ PlayerTurn | AddType$ Creature & Ninja | Description$ During your turn, as long as NICKNAME has one or more loyalty counters on him, he's a 3/4 Ninja creature and has hexproof.\n"+
		"Oracle:x")
	if got := cond.Faces[0].CDAAddTypes(); len(got) != 0 {
		t.Fatalf("conditional CDA AddType$ derived intrinsically: %v", got)
	}
	oc := h.g.AddObject(cond, 1)
	oc.Zone = state.ZHand
	if hasType(oc, "Ninja") {
		t.Error("a conditional CDA's type leaked into the every-zone intrinsic read")
	}

	// A static affecting OTHER permanents (Adaptive Automaton's
	// Affected$ Creature.YouCtrl AddType$ ChosenType) is a battlefield grant
	// for the layer walk, never the card's own type.
	grant := mkCard(t, "Name:Adaptive Automaton\nManaCost:3\nTypes:Artifact Construct\nPT:2/2\n"+
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddType$ ChosenType | Description$ As CARDNAME enters the battlefield, choose a creature type. Creatures you control of the chosen type are the chosen type in addition to their other types.\n"+
		"Oracle:x")
	if got := grant.Faces[0].CDAAddTypes(); len(got) != 0 {
		t.Fatalf("other-permanents AddType$ derived intrinsically: %v", got)
	}

	// A face-down permanent has no abilities (CR 708.2): the CDA tail is
	// behind the same gate the Changeling tail shares.
	vaObj := o
	vaObj.FaceDown = true
	vaObj.Zone = state.ZBattlefield
	if hasType(vaObj, "Rogue") {
		t.Error("face-down permanent matched a hidden face's CDA type")
	}
}
