package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestRelativeTargetEmptySourceDealsNothing pins Aura Barbs' RelativeTarget$
// arm with no source: a DamageSource$ Valid filter that resolves to no object
// has no recipient to re-resolve per source, so the resolving spell must not
// stand in and damage its own controller (CR 609.7a has no provenance).
func TestRelativeTargetEmptySourceDealsNothing(t *testing.T) {
	t.Parallel()
	spell := "Name:Pairing Bolt\nManaCost:2 R\nTypes:Instant\n" +
		"A:SP$ DealDamage | NumDmg$ 2 | DamageSource$ Valid Enchantment | Defined$ CardController | " +
		"RelativeTarget$ True | SpellDescription$ Each enchantment deals 2 damage to its controller.\nOracle:x\n"
	bear := "Name:Probe Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, cfg, spellID := newFixtureDeckWithOpponentCard(t, 94, spell, bear, bear)
	if e.G.Obj(spellID).Zone != state.ZHand {
		t.Fatalf("spell not in hand: %s", e.G.Obj(spellID).Zone)
	}
	if e.G.Players[0].Life != 20 || e.G.Players[1].Life != 20 {
		t.Fatalf("life precondition: %d/%d, want 20/20", e.G.Players[0].Life, e.G.Players[1].Life)
	}
	addMana(t, e, 0, "CCR")
	castFirst(t, e, "cast")
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("seat 0 life = %d, want 20: no enchantment is no source", got)
	}
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("seat 1 life = %d, want 20", got)
	}
	replayCheck(t, e, cfg)
}

// TestRelativeTargetOneSourcePairsItsOwnController pins the one-source arm of
// the same RelativeTarget$ shape: with exactly one enchantment the recipient
// still re-resolves against THAT source, so the enchantment damages its own
// controller, not the resolving spell's controller.
func TestRelativeTargetOneSourcePairsItsOwnController(t *testing.T) {
	t.Parallel()
	spell := "Name:Pairing Bolt\nManaCost:2 R\nTypes:Instant\n" +
		"A:SP$ DealDamage | NumDmg$ 2 | DamageSource$ Valid Enchantment | Defined$ CardController | " +
		"RelativeTarget$ True | SpellDescription$ Each enchantment deals 2 damage to its controller.\nOracle:x\n"
	aura := "Name:Probe Sigil\nManaCost:1 W\nTypes:Enchantment\nOracle:x\n"
	e, cfg, spellID := newFixtureDeckWithOpponentCard(t, 95, spell, aura, aura)
	oppAura := moveSeeded(t, e, 1, aura, state.ZBattlefield)
	if o := e.G.Obj(oppAura); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("opponent aura not on the battlefield: %+v", o)
	}
	if e.G.Obj(spellID).Zone != state.ZHand {
		t.Fatalf("spell not in hand: %s", e.G.Obj(spellID).Zone)
	}
	if e.G.Players[0].Life != 20 || e.G.Players[1].Life != 20 {
		t.Fatalf("life precondition: %d/%d, want 20/20", e.G.Players[0].Life, e.G.Players[1].Life)
	}
	addMana(t, e, 0, "CCR")
	castFirst(t, e, "cast")
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("seat 0 life = %d, want 20: the opponent's enchantment is its own damager", got)
	}
	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("seat 1 life = %d, want 18: the one enchantment damages its controller", got)
	}
	replayCheck(t, e, cfg)
}
