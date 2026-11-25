package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestCanEnchantSourcePredicate pins the CanEnchantSource filter predicate
// (Forge CardProperty.CanEnchantSource): the candidate Aura could legally
// enchant the ability's source. It is the hand/graveyard ChangeType read of
// Bruna, Light of Alabaster's ZoneAuras SVar and the battlefield
// RepeatCards$ read of her BrunaAttach SVar -- both name the ability's
// source as the enchant target, and both must see an off-battlefield Aura
// card as a candidate. An Enchant creature Aura matches a creature source and
// misses a land source; a Face-less ability wrapper unwraps to its Source
// permanent exactly as canEnchantEquippedBy does.
func TestCanEnchantSourcePredicate(t *testing.T) {
	h, _, ids := attachedToBoard(t)
	favor := h.g.Obj(ids["favor"])
	bear := h.g.Obj(ids["bear"])
	if favor == nil || bear == nil {
		t.Fatal("precondition: the fixture must have a Favor Aura and a Bear")
	}
	if favor.Zone == state.ZBattlefield {
		t.Fatal("precondition: Favor must be an off-battlefield card, the ChangeType$ Origin read")
	}
	if !hasType(favor, "Aura") || !hasType(favor, "Enchantment") {
		t.Fatalf("precondition: Favor is not an Aura (%v)", favor.Face().Types)
	}
	if bear.Zone != state.ZBattlefield || !hasType(bear, "Creature") {
		t.Fatalf("precondition: the Bear must be a battlefield creature, zone %v types %v", bear.Zone, bear.Face().Types)
	}
	c := &Ctx{Source: bear.ID, Controller: 0}
	if !MatchesSpecCtx(h.g, "Aura.CanEnchantSource", favor.ID, c.SpecContext(0)) {
		t.Fatal("Enchant creature Aura did not match a creature source")
	}
	// Bruna's ZoneAuras SVar spells the hand/graveyard read
	// Aura.CanEnchantSource+YouOwn: an owned, off-battlefield Aura card is a
	// candidate too.
	if !MatchesSpecCtx(h.g, "Aura.CanEnchantSource+YouOwn", favor.ID, c.SpecContext(0)) {
		t.Fatal("the ZoneAuras hand/graveyard spelling did not match a YouOwn Aura card")
	}

	// The same Aura cannot enchant a land: a legal source is not any source.
	land := h.g.AddObject(mkCard(t, "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"), 0)
	h.g.Obj(land.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(append([]state.ObjID(nil), h.g.Zone(state.ZBattlefield, 0)...), land.ID))
	if hasType(land, "Creature") {
		t.Fatal("precondition: the land source must not be a creature")
	}
	c.Source = land.ID
	if MatchesSpecCtx(h.g, "Aura.CanEnchantSource", favor.ID, c.SpecContext(0)) {
		t.Fatal("Enchant creature Aura matched a land source")
	}

	// The resolving source may be the Face-less ability/trigger wrapper a
	// TriggerPush minted: SpecContext.Source names the wrapper and its Source
	// field names the permanent. Unwrap before reading the enchant target.
	w := h.g.AddObject(nil, 0)
	wo := h.g.Obj(w.ID)
	wo.Zone = state.ZStack
	wo.Source = bear.ID
	if wo.Face() != nil {
		t.Fatal("precondition: the wrapper must be Face-less")
	}
	if !canEnchantSource(h.g, favor, 0, wo.ID) {
		t.Fatal("wrapper-sourced referent admitted nothing (unwrap missed)")
	}
}
