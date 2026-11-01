package rules

// CR 605.1a through the SubAbility$ chain. Witch Engine ("{T}: Add
// {B}{B}{B}{B}. Target opponent gains control of Witch Engine.") and Chandra,
// Bold Pyromancer's +1 ("Add {R}{R}. Chandra deals 2 damage to target
// player.") are AB$ Mana heads with no ValidTgts$ of their own; the target
// lives on the DB$ GainControl / DB$ DealDamage sub-ability. An ability that
// requires a target is not a mana ability, so both take the ordinary
// activated-ability path (target ask, stack) instead of the instant mana
// "activate" option, and the mana arrives when the ability resolves.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// requireChainTargetingManaAbility asserts the precondition both tests share:
// ability idx is an AB$ Mana whose head does not target but whose SubAbility$
// does, and the classifier keeps it off the mana path.
func requireChainTargetingManaAbility(t *testing.T, e *Engine, id state.ObjID, idx int) {
	t.Helper()
	f := e.G.Obj(id).Face()
	if idx >= len(f.Abilities) {
		t.Fatalf("precondition: %s has no ability %d", f.Name, idx)
	}
	ab := f.Abilities[idx]
	if ab == nil || ab.API != "Mana" || !ab.IsActivated() {
		t.Fatalf("precondition: %s ability %d is not an activated Mana ability: %+v", f.Name, idx, ab)
	}
	if effects.TargetsOf(ab).Targeted() {
		t.Fatalf("precondition: %s's Mana head itself targets (this is the head-gate case): %s", f.Name, ab.Line)
	}
	if ab.Sub == nil || !effects.TargetsOf(ab.Sub).Targeted() {
		t.Fatalf("precondition: %s's SubAbility$ does not target: %s", f.Name, ab.Line)
	}
	if cards.IsManaAbilitySA(ab) {
		t.Fatalf("%s's chain-targeting Mana ability classifies as a mana ability (CR 605.1a)", f.Name)
	}
	for _, ma := range f.ManaAbilities() {
		if ma == ab {
			t.Fatalf("%s lists its targeting ability in Face.ManaAbilities()", f.Name)
		}
	}
}

// TestWitchEngineTargetsThroughItsSubAbilityAndUsesTheStack drives the real
// offer: the tap ability is an "ability" option, not a mana "activate"; it
// asks for the opponent, waits on the stack, and on resolution the activator
// gets {B}{B}{B}{B} and the opponent gains control.
func TestWitchEngineTargetsThroughItsSubAbilityAndUsesTheStack(t *testing.T) {
	t.Parallel()
	e, cfg, ids := edrBoard(t, testutil.CorpusRegistry(t), 103, map[string]state.Zone{
		"Witch Engine": state.ZBattlefield})
	engine := ids["Witch Engine"]
	if o := e.G.Obj(engine); o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Witch Engine is not seat 0's on the battlefield: zone=%s controller=%d", o.Zone, o.Controller)
	}
	requireChainTargetingManaAbility(t, e, engine, 0)
	if hasOptionKindFor(e, "activate", engine) {
		t.Fatalf("Witch Engine offered as a mana-ability activation: %+v", e.Pending().Options)
	}
	e.G.Obj(engine).SummonSick = false
	submitChoices(t, e, abilityOption(t, e, engine, 0).Index)
	if d := e.Pending(); d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the target-opponent ask, got %+v", d)
	}
	submitChoices(t, e, gateOptionByLabel(t, e, "player", "b").Index)
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack depth after activation = %d, want the ability on the stack", len(e.G.Stack))
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("mana arrived before the ability resolved: %v", e.G.Players[0].Pool)
	}
	passToResolution(t, e)
	if got := e.G.Players[0].Pool[state.MB]; got != 4 || e.G.Players[0].Pool.Total() != 4 {
		t.Fatalf("activator's pool %v, want exactly {B}{B}{B}{B}", e.G.Players[0].Pool)
	}
	if c := e.G.Obj(engine).Controller; c != 1 {
		t.Fatalf("Witch Engine's controller = %d, want 1 (the targeted opponent gained control)", c)
	}
	replayCheck(t, e, cfg)
}

// TestChandraBoldPyromancerPlusOneTargetsThroughItsSubAbility is the loyalty
// carrier: the +1 is offered as an ordinary loyalty "ability", asks for the
// target player, and adds {R}{R} with the 2 damage on resolution.
func TestChandraBoldPyromancerPlusOneTargetsThroughItsSubAbility(t *testing.T) {
	t.Parallel()
	e, cfg, ids := edrBoard(t, testutil.CorpusRegistry(t), 104, map[string]state.Zone{
		"Chandra, Bold Pyromancer": state.ZBattlefield})
	chandra := ids["Chandra, Bold Pyromancer"]
	if o := e.G.Obj(chandra); o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Chandra is not seat 0's on the battlefield: zone=%s controller=%d", o.Zone, o.Controller)
	}
	requireChainTargetingManaAbility(t, e, chandra, 0)
	if hasOptionKindFor(e, "activate", chandra) {
		t.Fatalf("Chandra's +1 offered as a mana-ability activation: %+v", e.Pending().Options)
	}
	life := e.G.Players[1].Life
	submitChoices(t, e, abilityOption(t, e, chandra, 0).Index)
	if d := e.Pending(); d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected the target-player ask, got %+v", d)
	}
	submitChoices(t, e, gateOptionByLabel(t, e, "player", "b").Index)
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack depth after activation = %d, want the ability on the stack", len(e.G.Stack))
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("mana arrived before the ability resolved: %v", e.G.Players[0].Pool)
	}
	passToResolution(t, e)
	if got := e.G.Players[0].Pool[state.MR]; got != 2 || e.G.Players[0].Pool.Total() != 2 {
		t.Fatalf("activator's pool %v, want exactly {R}{R}", e.G.Players[0].Pool)
	}
	if got := e.G.Players[1].Life; got != life-2 {
		t.Fatalf("opponent's life = %d, want %d (2 damage from the sub-ability)", got, life-2)
	}
	replayCheck(t, e, cfg)
}

// TestChainTargetingManaRiderStaysDeferredWhenPlannedDirectly keeps the
// payment planner's "source:target" tier covered after the gate: Witch Engine
// was its only corpus producer, and it no longer reaches the planner through
// Face.ManaAbilities(). The tier is the planner's own defence for a Mana
// ability whose rider targets, so an inline Witch-Engine-shaped card is
// classified directly (never through the mana list) and must stay deferred.
func TestChainTargetingManaRiderStaysDeferredWhenPlannedDirectly(t *testing.T) {
	t.Parallel()
	const src = "Name:Chain Engine\nManaCost:5 B\nTypes:Creature Horror\nPT:4/4\n" +
		"A:AB$ Mana | Cost$ T | Produced$ B | Amount$ 4 | SubAbility$ ChangeControl | SpellDescription$ Add {B}{B}{B}{B}.\n" +
		"SVar:ChangeControl:DB$ GainControl | Defined$ Self | ValidTgts$ Opponent | SpellDescription$ Target opponent gains control of CARDNAME.\n" +
		"Oracle:x\n"
	e, _, _ := newFixtureDeck(t, 9843, src)
	id := onBoardCard(t, e, 0, card(t, src))
	f := e.G.Obj(id).Face()
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source is not on the battlefield (zone %s)", o.Zone)
	}
	if len(f.Abilities) == 0 || f.Abilities[0].API != "Mana" || f.Abilities[0].Sub == nil || !effects.TargetsOf(f.Abilities[0].Sub).Targeted() {
		t.Fatalf("precondition: ability 0 is not an AB$ Mana whose SubAbility$ targets: %+v", f.Abilities)
	}
	if n := len(f.ManaAbilities()); n != 0 {
		t.Fatalf("precondition: a chain-targeting ability is in Face.ManaAbilities() (%d)", n)
	}
	tier, c, detail := pay.PaymentPlanAbilityTier(asPayer(e), 0, id, f.Abilities[0])
	if tier != pay.TierDeferred || detail != "source:target" || c != (pay.Consequence{}) {
		t.Fatalf("planner tier = (%d, %+v, %q), want (Deferred, {}, source:target)", tier, c, detail)
	}
}
