// combat_damage_walker_creature_test.go pins CR 120.3c/120.3e for COMBAT
// damage: a planeswalker that is also a creature (a printed walker animated
// by a layer effect, as Gideon's 0 abilities and Sarkhan the Masterless do)
// loses that many loyalty counters AND has that much damage marked on it,
// the same split the DealDamage emitter makes for spell damage
// (TestAnimatedWalkerDamageAndCleanup).
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// walkerWallSrc is a printed planeswalker that animates itself into a 0/4
// creature through an ordinary Continuous static (CR 613.1d/613.4b), so it
// can block and the log replays.
const walkerWallSrc = "Name:Walker Wall\nManaCost:2\nTypes:Legendary Planeswalker Gideon\nLoyalty:5\n" +
	"S:Mode$ Continuous | Affects$ Card.Self | AddType$ Creature | SetPower$ 0 | SetToughness$ 4 | Description$ It's a 0/4 creature that's still a planeswalker.\nOracle:x\n"

// TestCombatDamageToAnimatedWalkerMarksDamageAndRemovesLoyalty: seat 0's
// 1/1 Ally Bear attacks and is blocked by seat 1's animated Walker Wall;
// the combat damage removes one loyalty counter (CR 120.3c) and marks one
// damage (CR 120.3e).
func TestCombatDamageToAnimatedWalkerMarksDamageAndRemovesLoyalty(t *testing.T) {
	t.Parallel()
	s0 := []*cards.Card{card(t, ninjutsuBearSrc)}
	s0 = append(s0, mountainDeck(t, 39)...)
	s1 := []*cards.Card{card(t, walkerWallSrc)}
	s1 = append(s1, mountainDeck(t, 39)...)
	cfg := seatZeroStart(Config{Seed: 1430, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{s0, s1}, Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	bear := putCreature(t, e, 0, ninjutsuBearSrc)
	wall := putCreature(t, e, 1, walkerWallSrc)
	if !e.IsCreature(wall) || e.Toughness(wall) != 4 || e.G.Obj(wall).Counter("LOYALTY") != 5 {
		t.Fatalf("precondition: Walker Wall is not an animated 0/4 with 5 loyalty (creature %v, toughness %d, loyalty %d)",
			e.IsCreature(wall), e.Toughness(wall), e.G.Obj(wall).Counter("LOYALTY"))
	}
	e.priorityRound()
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	// Attack the player (the walker blocks), not the walker itself.
	submitChoices(t, e, attackOptionAtPlayer(t, e, bear))
	passToKind(t, e, decision.KBlockers)
	submitBlockersOnly(t, e, wall)
	driveToStep(t, e, e.G.Turn, 0, state.StepEndCombat)
	o := e.G.Obj(wall)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Walker Wall left the battlefield: %+v", o)
	}
	if got := o.Counter("LOYALTY"); got != 4 {
		t.Fatalf("Walker Wall loyalty = %d after 1 combat damage, want 4 (CR 120.3c)", got)
	}
	if o.Damage != 1 {
		t.Fatalf("Walker Wall marked damage = %d after 1 combat damage, want 1 (CR 120.3e: it is a creature)", o.Damage)
	}
	replayCheck(t, e, cfg)
}

// attackOptionAtPlayer is the index of the pending attackers decision's
// option that attacks the defending player (no planeswalker or battle) with id.
func attackOptionAtPlayer(t *testing.T, e *Engine, id state.ObjID) int {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Obj == id && o.Battle == 0 {
			return o.Index
		}
	}
	t.Fatalf("no option attacks the player with %d", id)
	return -1
}

// TestHealingAnAnimatedWalkerClearsItsMarkedDamage is the same recipient
// form on the negative clear: HealMarkedDamage (the regeneration / umbra
// armor / HealDamage clear) removes the damage marked on a layer-animated
// planeswalker and leaves its loyalty alone.
func TestHealingAnAnimatedWalkerClearsItsMarkedDamage(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	walkerObj := e.G.AddObject(card(t, "Name:Animated Jace\nTypes:Planeswalker Jace\nLoyalty:4\nOracle:x\n"), 0)
	walker := walkerObj.ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: walker, From: walkerObj.Zone, To: state.ZBattlefield})
	e.AddContinuous(state.ContinuousEffect{Source: walker, Controller: 0,
		Affects: "Card.Self", Layer: LType, AddTypes: []string{"Creature"}})
	effects.Resolve(e, &effects.Ctx{Source: walker, Controller: 0,
		Targets: []state.Target{{Obj: walker}}}, &cards.SA{Kind: "DB", API: "DealDamage",
		Params: map[string]string{"Defined": "Targeted", "NumDmg": "2"}})
	if o := e.G.Obj(walker); o.Damage != 2 || o.Counter("LOYALTY") != 2 {
		t.Fatalf("precondition: animated walker after damage: marked=%d loyalty=%d, want 2/2", o.Damage, o.Counter("LOYALTY"))
	}
	effects.HealMarkedDamage(e, walker)
	if o := e.G.Obj(walker); o.Damage != 0 || o.Counter("LOYALTY") != 2 {
		t.Fatalf("animated walker after healing: marked=%d loyalty=%d, want 0/2 (the clear must reach a layer-animated creature)", o.Damage, o.Counter("LOYALTY"))
	}
}
