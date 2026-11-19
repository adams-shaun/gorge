package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestCDAAddTypeIntrinsicEveryZone pins the type filter's intrinsic AddType$
// CDA tail (filter_type.go's cdaAddTypeHas): the party card's four-kind
// characteristic-defining ability is one, so CR 613.4a answers it in every
// zone — off the battlefield (in a hand, library or exile) where the rules
// layer walk publishes no derived type list, the printed-face fallback plus
// the intrinsic CDAs are the whole answer, and the printed face alone is not
// enough. The read is what a RestrictValid$ Spell.Creature+ChosenType batch
// (The Ringhart Crest's mana) evaluates the hand card with, so a miss here
// withheld a cast the restricted mana could have paid (tail-74 pair 127,
// seed 9802406336505234860, decision 498; the pinned repro is
// dirac/oracle/testdata/repro/cda_addtype.repro.json).
func TestCDAAddTypeIntrinsicEveryZone(t *testing.T) {
	va := mkCard(t, "Name:Party Veteran\nManaCost:5 G\nTypes:Creature Human\nPT:5/5\n"+
		"S:Mode$ Continuous | CharacteristicDefining$ True | Affected$ Card.Self | AddType$ Cleric & Rogue & Warrior & Wizard | Description$ Shares its creature kinds with every party.\n"+
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
	cond := mkCard(t, "Name:Turnbound Shapeshifter\nTypes:Planeswalker Shifter\nLoyalty:5\n"+
		"S:Mode$ Continuous | Condition$ PlayerTurn | AffectedDefined$ Self | AddType$ Creature & Ninja | Description$ On your turn it becomes a Ninja creature.\n"+
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
	grant := mkCard(t, "Name:Kindred Cogwork\nManaCost:3\nTypes:Artifact Construct\nPT:2/2\n"+
		"S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddType$ ChosenType | Description$ Grants its chosen kind to your creatures.\n"+
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

	// A ZONE-GATED CDA (Tidecaller's "As long as CARDNAME isn't on the
	// battlefield, it's a 1/1 Insect creature", the corpus's
	// ExcludeZone$ Battlefield shape) is answered against the object's zone:
	// granted off the battlefield, refused on it — there the static is
	// excluded and the card is its printed planeswalker self, so the
	// intrinsic read granting Creature/Insect in every zone was the wrong
	// direction. The every-zone derived list holds nothing for it; the
	// tail's zone-gated half evaluates the gate.
	grist := mkCard(t, "Name:Tidecaller of the Swarm\nManaCost:2 B G\nTypes:Legendary Planeswalker Swarmcaller\nLoyalty:4\n"+
		"S:Mode$ Continuous | ExcludeZone$ Battlefield | Affected$ Card.Self | SetPower$ 1 | SetToughness$ 1 | AddType$ Creature & Insect | CharacteristicDefining$ True | Description$ Off the battlefield CARDNAME is a 1/1 Insect creature in addition to its other types.\n"+
		"Oracle:x")
	gf := grist.Faces[0]
	// Precondition: the printed face carries neither word, so a pass against
	// a face that already printed them proves nothing.
	if gf.TypeLineHas("Creature", 0) || gf.TypeLineHas("Insect", 0) {
		t.Fatal("precondition: printed face already carries a zone-gated CDA type")
	}
	if got := gf.CDAAddTypes(); len(got) != 0 {
		t.Fatalf("zone-gated CDA AddType$ derived into the every-zone list: %v", got)
	}
	og := h.g.AddObject(grist, 1)
	og.Zone = state.ZBattlefield
	if hasType(og, "Creature") || hasType(og, "Insect") {
		t.Error("battlefield misread: zone-gated CDA AddType$ granted a type where its static is excluded")
	}
	og.Zone = state.ZHand
	if !hasType(og, "Creature") || !hasType(og, "Insect") {
		t.Error("hand card lost its zone-gated characteristic-defining type (CR 613.4a)")
	}
	if !hasType(og, "Swarmcaller") {
		t.Error("the zone-gated CDA read displaced the printed type line")
	}

	// A gate the intrinsic read cannot evaluate (the Dragonsage's
	// IsPresent$ loyalty threshold, the corpus's counter-condition shape) is
	// fail-closed in every zone: the rules layer walk evaluates the
	// condition on the battlefield, and nothing grants the words anywhere
	// else.
	gm := mkCard(t, "Name:Flower Dragonsage\nTypes:Planeswalker Sage\nLoyalty:4\n"+
		"S:Mode$ Continuous | IsPresent$ Card.Self+counters_GE7_LOYALTY | AddKeyword$ Flying & Indestructible | AddType$ Creature & Dragon & God | SetPower$ 7 | SetToughness$ 7 | CharacteristicDefining$ True | Description$ With seven or more loyalty counters on CARDNAME, it becomes a 7/7 Dragon God creature.\n"+
		"Oracle:x")
	if got := gm.Faces[0].CDAAddTypes(); len(got) != 0 {
		t.Fatalf("counter-conditioned CDA AddType$ derived intrinsically: %v", got)
	}
	om := h.g.AddObject(gm, 1)
	om.Zone = state.ZBattlefield
	if hasType(om, "Creature") || hasType(om, "Dragon") || hasType(om, "God") {
		t.Error("battlefield misread: counter-conditioned CDA AddType$ granted a type its condition does not admit")
	}
	om.Zone = state.ZHand
	if hasType(om, "Dragon") || hasType(om, "God") {
		t.Error("a counter-conditioned CDA's type leaked into the every-zone intrinsic read")
	}

	// Placeholder words name no card type (the Chosengearwork's ChosenType,
	// the battery shapes' AllBasicLandType/AllNonBasicLandType): a filter
	// resolves them to a real word before it queries a type, so the derived
	// list collects no literal.
	markers := []struct{ script, word string }{
		{"Name:Chosengearwork\nManaCost:3\nTypes:Artifact Construct\nPT:2/2\n" +
			"S:Mode$ Continuous | CharacteristicDefining$ True | Affected$ Card.Self | AddType$ ChosenType | Description$ CARDNAME is also its chosen kind.\n" +
			"Oracle:x", "ChosenType"},
		{"Name:Nexus Battery\nTypes:Land\n" +
			"S:Mode$ Continuous | CharacteristicDefining$ True | Affected$ Card.Self | AddType$ AllNonBasicLandType | Description$ CARDNAME is every nonbasic land kind.\n" +
			"Oracle:x", "AllNonBasicLandType"},
		{"Name:Orbital Planet\nTypes:Land\n" +
			"S:Mode$ Continuous | CharacteristicDefining$ True | Affected$ Card.Self | AddType$ AllBasicLandType & AllNonBasicLandType | Description$ CARDNAME is every basic and nonbasic land kind.\n" +
			"Oracle:x", "AllBasicLandType"},
	}
	for _, m := range markers {
		c := mkCard(t, m.script)
		if got := c.Faces[0].CDAAddTypes(); len(got) != 0 {
			t.Errorf("%s: placeholder word %q derived as a literal list member: %v", m.script[:20], m.word, got)
		}
		o2 := h.g.AddObject(c, 1)
		o2.Zone = state.ZHand
		if hasType(o2, m.word) {
			t.Errorf("placeholder word %q answered a type query as a CDA type", m.word)
		}
	}
}
