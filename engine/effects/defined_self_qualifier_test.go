package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// ironManArmorAnimate is Iron Man Armor's Animate line, with the printed
// `Defined$ Self.nonCreature` ("If this Equipment isn't a creature").
const ironManArmorAnimate = "AB$ Animate | Cost$ 2 | Defined$ Self.nonCreature | Power$ 0 | Toughness$ 0 | " +
	"Types$ Creature,Artifact,Construct,Hero | Keywords$ Flying | RemoveCreatureTypes$ True"

// TestDefinedSelfQualifier pins `Self.<qualifier>`: the source only when it
// passes the qualifier, a known-empty set otherwise, never the source by the
// unmodelled-spelling fallback.
func TestDefinedSelfQualifier(t *testing.T) {
	h, c := fixtureHost(t) // the source is a Creature
	src := h.g.Obj(c.Source)
	if src == nil || !definedCardQualifierMatches(h, h.g, c, "Creature", src) {
		t.Fatalf("precondition: fixture source is not a Creature: %+v", src)
	}
	if got, ok := knownDefinedTargets(h, c, "Self.nonCreature"); !ok || len(got) != 0 {
		t.Errorf("Self.nonCreature on a creature source = %v ok=%v, want empty ok=true", got, ok)
	}
	if got := Defined(h, c, sa(t, "SP$ Animate | Defined$ Self.nonCreature | Power$ 0 | Toughness$ 0")); len(got) != 0 {
		t.Errorf("Defined Self.nonCreature on a creature source = %v, want empty (not the source)", got)
	}
	if got, ok := knownDefinedTargets(h, c, "Self.Creature"); !ok || len(got) != 1 || got[0].Obj != c.Source {
		t.Errorf("Self.Creature = %v ok=%v, want [source]", got, ok)
	}
}

// TestDefinedSelfQualifierIronManArmor resolves the real Animate line: a
// non-creature Armor animates; once it is a creature a second activation
// registers nothing, so the "+1/+1 for each artifact" static is not doubled.
func TestDefinedSelfQualifierIronManArmor(t *testing.T) {
	h := newHost(t, 2)
	armor := h.g.AddObject(mkCard(t, "Name:Iron Man Armor\nTypes:Artifact Equipment\nOracle:x\n"), 0)
	h.g.Obj(armor.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), armor.ID))
	c := &Ctx{Source: armor.ID, Controller: 0}
	line := sa(t, ironManArmorAnimate)

	Resolve(h, c, line)
	first := len(h.continuous)
	if first == 0 {
		t.Fatalf("precondition: a non-creature Armor registered no animate effect")
	}

	// Once animated the Armor is a creature: model it as a creature-faced
	// object, since the fake host applies no layers.
	creature := h.g.AddObject(mkCard(t, "Name:Iron Man Armor\nTypes:Artifact Creature Construct Hero\nPT:0/0\nOracle:x\n"), 0)
	h.g.Obj(creature.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), creature.ID))
	c2 := &Ctx{Source: creature.ID, Controller: 0}
	Resolve(h, c2, line)
	if len(h.continuous) != first {
		t.Errorf("second activation on a creature Armor registered %d more effects, want none", len(h.continuous)-first)
	}
}
