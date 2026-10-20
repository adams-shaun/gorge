package botpolicy

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// removalDecision builds the KTarget decision the A2 rules act on: a
// destroy-shaped removal over the given options, for seat 0.
func removalDecision(options []tgt) *decision.Decision {
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KTarget, Min: 1, Max: 1,
		TargetEffect: &decision.TargetEffect{API: "Destroy", Removal: &decision.RemovalEffect{Kind: "destroy"}}}
	for _, o := range options {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: o.kind, Obj: o.obj, Player: o.pl, Controller: o.pl})
	}
	return &d
}

// ruleBoard is a board with the given cards and creatures, both tables
// allocated, so a rule can look up either. public is the public-card colour
// census the A2 colour fold reads, keyed by object id.
func ruleBoard(cs map[state.ObjID]Card, crs []state.ObjID, public map[state.ObjID]PublicCard) Board {
	return ruleBoardKw(cs, crs, nil, public)
}

// ruleBoardKw is ruleBoard with per-creature keyword lists, for the copy
// rule's keyword-superset test.
func ruleBoardKw(cs map[state.ObjID]Card, crs []state.ObjID, kw map[state.ObjID][]string, public map[state.ObjID]PublicCard) Board {
	b := Board{
		Cards:       TableOf(map[state.ObjID]Card{}),
		Creatures:   TableOf(map[state.ObjID]Creature{}),
		Life:        TableOf(map[state.PlayerID]int32{}),
		PublicCards: TableOf(public),
	}
	for id, c := range cs {
		b.Cards.Set(id, c)
	}
	for _, id := range crs {
		c := cs[id]
		b.Creatures.Set(id, Creature{Power: c.Power, Toughness: c.Toughness, Controller: 1, Keywords: kw[id]})
	}
	return b
}

// TestLandPriorityPrefersTheDominantColoursLand is the core A2 land rule:
// with land priority armed, a removal targets the opponent's black land over
// an off-colour land of the same cost, and the unarmed default keeps the
// option index's order.
func TestLandPriorityPrefersTheDominantColoursLand(t *testing.T) {
	// Option 0 is the splash land, option 1 the black land: the plain
	// ranking takes option 0 (equal score, lower index). Both are CMC 0,
	// the score a land actually carries, so this pins that the A2 rules
	// are applied before removalRanker's zero-value early return.
	cs := map[state.ObjID]Card{
		10: {PrintedName: "Mountain", Land: true, Colour: cards.ColourMaskRed, CMC: 0},
		11: {PrintedName: "Swamp", Land: true, Colour: cards.ColourMaskBlack, CMC: 0},
	}
	public := map[state.ObjID]PublicCard{
		10: {Controller: 1, Colour: cards.ColourMaskRed},
		11: {Controller: 1, Colour: cards.ColourMaskBlack},
	}
	b := ruleBoard(cs, nil, public)
	d := removalDecision([]tgt{{kind: "permanent", obj: 10, pl: 1}, {kind: "permanent", obj: 11, pl: 1}})

	if got := Decide(b, d, rng(1)).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("default choices = %v, want [0] (unchanged ranking)", got)
	}
	b.Targets = TargetRules{LandPriority: true}
	if got := Decide(b, d, rng(1)).Choices; len(got) != 1 || got[0] != 1 {
		t.Fatalf("armed choices = %v, want [1] (the dominant-colour land)", got)
	}
}

// TestColourlessLandEarnsNoPriority: a colourless land is never the source a
// deck is built around, so it gains nothing.
func TestColourlessLandEarnsNoPriority(t *testing.T) {
	public := map[state.ObjID]PublicCard{
		11: {Controller: 1, Colour: cards.ColourMaskBlack},
	}
	b := ruleBoard(map[state.ObjID]Card{10: {PrintedName: "Wastes", Land: true, CMC: 1}}, nil, public)
	if got := b.landPriority(10, dominantColour(b.publicColourMix(1))); got != 0 {
		t.Fatalf("colourless land priority = %d, want 0", got)
	}
}

// TestProminentColourBoostsTheEnginesCreature: with the rule armed, a
// creature of the dominant colour outranks an off-colour creature of equal
// stats; the unarmed default takes the lower index.
func TestProminentColourBoostsTheEnginesCreature(t *testing.T) {
	cs := map[state.ObjID]Card{
		10: {PrintedName: "Goblin", Creature: true, Power: 2, Toughness: 2, Colour: cards.ColourMaskRed},
		11: {PrintedName: "Zombie", Creature: true, Power: 2, Toughness: 2, Colour: cards.ColourMaskBlack},
	}
	public := map[state.ObjID]PublicCard{
		10: {Controller: 1, Colour: cards.ColourMaskRed},
		11: {Controller: 1, Colour: cards.ColourMaskBlack},
	}
	b := ruleBoard(cs, []state.ObjID{10, 11}, public)
	d := removalDecision([]tgt{{kind: "permanent", obj: 10, pl: 1}, {kind: "permanent", obj: 11, pl: 1}})

	if got := Decide(b, d, rng(1)).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("default choices = %v, want [0]", got)
	}
	b.Targets = TargetRules{ProminentColour: true}
	if got := Decide(b, d, rng(1)).Choices; len(got) != 1 || got[0] != 1 {
		t.Fatalf("armed choices = %v, want [1] (the deck's engine)", got)
	}
}

// TestFilterFatalCopiesSkipsTheWorseCopy: a 3/3 beside a 4/4 of the same
// types is the worse copy, so the armed rule removes the 4/4. The unarmed
// default takes the lower index (the 3/3).
func TestFilterFatalCopiesSkipsTheWorseCopy(t *testing.T) {
	cs := map[state.ObjID]Card{
		10: {PrintedName: "Plain Goblin", Creature: true, Power: 2, Toughness: 2, TypeSig: TypeSigOf([]string{"Creature", "Goblin"})},
		11: {PrintedName: "Flying Goblin", Creature: true, Power: 2, Toughness: 2, TypeSig: TypeSigOf([]string{"Creature", "Goblin"})},
	}
	b := ruleBoardKw(cs, []state.ObjID{10, 11}, map[state.ObjID][]string{11: {"Flying"}}, nil)
	d := removalDecision([]tgt{{kind: "permanent", obj: 10, pl: 1}, {kind: "permanent", obj: 11, pl: 1}})

	// threat() already ranks the flier above the plain body, so the default
	// answer is the same one the rule wants; the test's point is that the
	// armed rule reaches it through the copy test and not merely by accident.
	if got := Decide(b, d, rng(1)).Choices; len(got) != 1 || got[0] != 1 {
		t.Fatalf("default choices = %v, want [1]", got)
	}
	b.Targets = TargetRules{FilterFatalCopies: true}
	if !b.worseCopy(10, 0) {
		t.Fatal("worseCopy(plain goblin) = false, want true beside a flier")
	}
	if b.worseCopy(11, 0) {
		t.Fatal("worseCopy(flier) = true, want false")
	}
	if got := Decide(b, d, rng(1)).Choices; len(got) != 1 || got[0] != 1 {
		t.Fatalf("armed choices = %v, want [1] (the better copy)", got)
	}
}

// TestWorseCopyStaysTheAnswerWhenAlone: the copy filter must never leave a
// decision unanswered -- a lone worse copy is still a legal kill.
func TestWorseCopyStaysTheAnswerWhenAlone(t *testing.T) {
	cs := map[state.ObjID]Card{
		10: {PrintedName: "Plain Goblin", Creature: true, Power: 2, Toughness: 2, TypeSig: TypeSigOf([]string{"Creature", "Goblin"})},
		11: {PrintedName: "Flying Goblin", Creature: true, Power: 2, Toughness: 2, TypeSig: TypeSigOf([]string{"Creature", "Goblin"})},
	}
	b := ruleBoard(cs, []state.ObjID{10, 11}, nil)
	b.Targets = TargetRules{FilterFatalCopies: true}
	d := removalDecision([]tgt{{kind: "permanent", obj: 10, pl: 1}})
	if got := Decide(b, d, rng(1)).Choices; len(got) != 1 || got[0] != 0 {
		t.Fatalf("choices = %v, want [0] (the only offered target)", got)
	}
}

// TestTypeSigIgnoresWordOrderAndCase: the signature is a set, so a
// differently-ordered type line still compares equal.
func TestTypeSigIgnoresWordOrderAndCase(t *testing.T) {
	a := TypeSigOf([]string{"Creature", "Goblin"})
	b := TypeSigOf([]string{"goblin", "CREATURE"})
	if a != b {
		t.Fatalf("signatures differ: %d vs %d", a, b)
	}
	if a == TypeSigOf([]string{"Creature", "Elf"}) {
		t.Fatal("goblin and elf signatures collide")
	}
	if TypeSigOf(nil) != 0 {
		t.Fatal("empty type line must be the unreadable zero")
	}
}

// TestZeroTargetRulesIsTheDefaultRanking pins the opt-in contract: a Board
// whose Targets is the zero value answers exactly as a Board that never
// carried the field.
func TestZeroTargetRulesIsTheDefaultRanking(t *testing.T) {
	cs := map[state.ObjID]Card{
		10: {PrintedName: "Wastes", Land: true, CMC: 1},
		11: {PrintedName: "Swamp", Land: true, CMC: 1, Colour: cards.ColourMaskBlack},
	}
	b := ruleBoard(cs, nil, nil)
	d := removalDecision([]tgt{{kind: "permanent", obj: 10, pl: 1}, {kind: "permanent", obj: 11, pl: 1}})
	a := Decide(b, d, rng(1)).Choices
	b.Targets = TargetRules{}
	if c := Decide(b, d, rng(1)).Choices; len(c) != 1 || c[0] != a[0] {
		t.Fatalf("zero rules changed the answer: %v vs %v", c, a)
	}
}
