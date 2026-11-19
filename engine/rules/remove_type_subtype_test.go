package rules

// stat:Continuous RemoveType$ Creature (CR 205.1a): removing the Creature
// card type also removes the subtypes correlated with it -- "If an object's
// card type is removed, the subtypes correlated with that card type will
// remain if they are also the subtypes of a card type the object currently
// has; otherwise, they are also removed for the entire time the object's
// card type is removed." Heliod, Sun-Crowned is the corpus carrier: its
// devotion static reads RemoveType$ Creature, and while the gate holds the
// object must read Legendary Enchantment, not Legendary Enchantment God.
// This is the farm fshadow class snapshot|priority|permanents|?Heliod,
// Sun-Crowned (MSH-s433242882-cmdr4): gorge's snapshot kept the God word
// while Forge dropped it with the card type.
//
// The devotion arithmetic and gate shape are pinned end to end on Purphoros
// in remove_type_static_test.go; this file pins the SUBTYPE follow-through
// the same grant owes.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// heliodShape asserts the audited corpus shape and returns the SVar body.
// The assertions ride on the exact static spelling, so a corpus pin change
// that hollows the pin out fails here rather than passing vacuously.
func heliodShape(t *testing.T, heliod *cards.Card) string {
	t.Helper()
	f := heliod.Faces[0]
	var stat *cards.Static
	for i := range f.Statics {
		s := &f.Statics[i]
		if s.Mode == "Continuous" && s.Params["Affected"] == "Card.Self" && s.Params["RemoveType"] != "" {
			stat = s
		}
	}
	if stat == nil {
		t.Fatalf("corpus Heliod, Sun-Crowned carries no Continuous Card.Self RemoveType$ static: %+v", f.Statics)
	}
	if got := stat.Params["RemoveType"]; got != "Creature" {
		t.Fatalf("Heliod static RemoveType = %q, want Creature (params %v)", got, stat.Params)
	}
	if got := stat.Params["CheckSVar"]; got != "X" {
		t.Fatalf("Heliod static CheckSVar = %q, want X (params %v)", got, stat.Params)
	}
	if got := stat.Params["SVarCompare"]; got != "LT5" {
		t.Fatalf("Heliod static SVarCompare = %q, want LT5 (params %v)", got, stat.Params)
	}
	body := f.SVars["X"]
	if body != "Count$Devotion.White" {
		t.Fatalf("corpus Heliod SVar:X = %q, want Count$Devotion.White", body)
	}
	return body
}

// TestRemoveTypeCreatureAlsoStripsCreatureSubtypes: Heliod, Sun-Crowned alone
// is devotion to white 1, below five, so its RemoveType$ Creature static
// applies and the object reads Legendary Enchantment -- the God subtype goes
// with the Creature card type (CR 205.1a). Four more one-white-pip
// permanents bring the devotion to exactly five, the gate releases, and the
// printed God comes back with the 5/5 creature body.
func TestRemoveTypeCreatureAlsoStripsCreatureSubtypes(t *testing.T) {
	hel := corpusCard(t, "Heliod, Sun-Crowned")
	body := heliodShape(t, hel)

	e := layerEngine(t)
	helID := onBoardCard(t, e, 0, hel)

	// Precondition: the god is on seat 0's battlefield and its devotion SVar
	// is the object's own.
	if got := e.G.Obj(helID).Zone; got != state.ZBattlefield {
		t.Fatalf("precondition broken: Heliod zone = %v, want ZBattlefield", got)
	}
	if got := e.G.Obj(helID).Face().SVars["X"]; got != body {
		t.Fatalf("precondition broken: battlefield Heliod SVar:X = %q, want %q", got, body)
	}
	ctx := &effects.Ctx{Controller: 0, Source: helID}
	n, ok := effects.EvalCountOK(e, ctx, body)
	if !ok || n != 1 {
		t.Fatalf("precondition broken: devotion to white with Heliod alone = (%d, %v), want (1, true)", n, ok)
	}

	// Devotion one: the LT5 gate holds, the RemoveType$ Creature grant
	// applies, and the God subtype leaves with the Creature card type.
	ty := e.Derived(helID).Types
	if hasTypeWord(ty, "Creature") {
		t.Fatalf("devotion-1 Heliod types include Creature, want none (types %v)", ty)
	}
	if hasTypeWord(ty, "God") {
		t.Fatalf("devotion-1 Heliod types include God, want none: a subtype whose card type is removed leaves with it (CR 205.1a) (types %v)", ty)
	}
	if !hasTypeWord(ty, "Enchantment") || !hasTypeWord(ty, "Legendary") {
		t.Fatalf("devotion-1 Heliod lost its printed Enchantment or Legendary (types %v)", ty)
	}

	// Guard: four one-white-pip permanents bring the devotion to exactly
	// five, the gate releases, and the printed God is a 5/5 creature again.
	for i := 0; i < 4; i++ {
		pip := onBoardCard(t, e, 0, card(t, "Name:White Pip Elf\nManaCost:W\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"))
		if got := e.G.Obj(pip).Zone; got != state.ZBattlefield {
			t.Fatalf("precondition broken: white-pip fixture zone = %v, want ZBattlefield", got)
		}
	}
	n, ok = effects.EvalCountOK(e, ctx, body)
	if !ok || n != 5 {
		t.Fatalf("precondition broken: devotion to white with four pips = (%d, %v), want (5, true)", n, ok)
	}
	ty = e.Derived(helID).Types
	if !hasTypeWord(ty, "Creature") || !hasTypeWord(ty, "God") {
		t.Fatalf("devotion-5 Heliod types lost Creature or God (types %v)", ty)
	}
	if p, tg := e.Power(helID), e.Toughness(helID); p != 5 || tg != 5 {
		t.Fatalf("devotion-5 Heliod P/T = %d/%d, want 5/5", p, tg)
	}
}
