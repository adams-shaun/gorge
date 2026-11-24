package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMardu_NoExiledCard_AttackStillCreatesEndStepSacrifice adjudicates the
// xproof find xs:XP:mardu-siegebreaker:f0t0:s2 (XMage differs). Mardu
// Siegebreaker entered with no other creature to exile, then attacked: "for
// each opponent, create a tapped token that's a copy of the exiled card ...
// At the beginning of your end step, sacrifice those tokens." No token is
// made, but the delayed triggered ability is created by the resolving attack
// ability regardless (CR 603.7a), so it triggers at the beginning of the end
// step and goes on the stack, resolving with nothing to sacrifice. Forge
// registers it unconditionally and gorge matches; XMage never creates it.
func TestMardu_NoExiledCard_AttackStillCreatesEndStepSacrifice(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := ateotEngine(t, reg, "Mardu Siegebreaker")
	id := searchMoveByName(t, e, "Mardu Siegebreaker", state.ZBattlefield)
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Siegebreaker must be on the battlefield: %+v", o)
	}
	for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
		if oid != id && e.IsCreature(oid) {
			t.Fatalf("precondition: Siegebreaker must be the only creature p0 controls, found %d", oid)
		}
	}
	if n := len(e.G.Zone(state.ZExile, 0)) + len(e.G.Zone(state.ZExile, 1)); n != 0 {
		t.Fatalf("precondition: nothing may be exiled, found %d cards", n)
	}

	// Pass priority until the attackers are asked for.
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KAttackers {
			break
		}
		submitChoices(t, e, passIndex(t, d))
	}
	submitAttackers(t, e, id)
	passUntilStackEmpty(t, e, 20)
	if len(e.G.Delayed) != 1 {
		t.Fatalf("delayed registrations %d after the attack trigger resolved with no exiled card, want 1", len(e.G.Delayed))
	}
	replayCheck(t, e, cfg)

	ateotDriveToStep(t, e, e.G.Turn, e.G.Active, state.StepEnd)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		if len(e.G.Stack) != 0 {
			break
		}
	}
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack depth %d at the beginning of the end step, want the one delayed sacrifice trigger", len(e.G.Stack))
	}
	if top := e.G.Obj(e.G.Stack[0]); top == nil || top.Ability == nil || top.Source != id {
		t.Fatalf("stack top %+v, want Siegebreaker's delayed ability", top)
	}
	replayCheck(t, e, cfg)
	passUntilStackEmpty(t, e, 20)
	replayCheck(t, e, cfg)
}

// TestKatara_SummoningSickSourceTapsItselfToWaterbendX adjudicates the
// xproof find xs:XP:katara-water-tribe-s-hope:f0t0:s2 (Forge differs).
// Katara has just resolved, every land is tapped and her ETB token is still
// on the stack, so the only way to pay "Waterbend {X}" is tapping Katara
// herself. CR 701.67a lets a player tap any untapped creature they control
// for each generic mana of the waterbend cost; the {T}/summoning-sickness
// restriction (CR 302.6) applies only to a {T} cost on the creature's own
// ability, and Katara's ability has none. So with X = 1 the activation is
// legal, Katara ends up tapped and the ability is on the stack. Forge
// computes the waterbend cap while X is still unannounced (X = 0), so it
// never offers the tap and silently drops the activation.
func TestKatara_SummoningSickSourceTapsItselfToWaterbendX(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Katara, Water Tribe's Hope")
	id := searchMoveByName(t, e, "Katara, Water Tribe's Hope", state.ZBattlefield)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || o.Tapped || !o.SummonSick {
		t.Fatalf("precondition: Katara must be an untapped, summoning-sick permanent: %+v", o)
	}
	for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
		if oid != id && !e.G.Obj(oid).Tapped && (e.IsCreature(oid) || e.IsArtifact(oid)) {
			t.Fatalf("precondition: Katara must be the only possible waterbend tap, found %d", oid)
		}
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: pool must be empty, got %d", got)
	}
	opt, ok := findAbilityOption(e, id, 0)
	if !ok {
		t.Fatalf("Katara's Waterbend<X> ability not offered with only herself to tap: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the waterbend announcement, got %+v", d)
	}
	pick := -1
	for _, op := range d.Options {
		if op.Kind == "waterbend_generic" && op.Obj == id {
			pick = op.Index
		}
	}
	if pick < 0 {
		t.Fatalf("Katara herself not offered as a waterbend tap: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the X announcement, got %+v", d)
	}
	x1 := -1
	for _, op := range d.Options {
		if op.Kind == "x" && op.Amount == 1 {
			x1 = op.Index
		}
	}
	if x1 < 0 {
		t.Fatalf("X = 1 not offered: %+v", d.Options)
	}
	submitChoices(t, e, x1)
	if o := e.G.Obj(id); !o.Tapped {
		t.Fatal("Katara was not tapped to pay the waterbend cost")
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("the activated ability is not on the stack")
	}
	if top := e.G.Obj(e.G.Stack[len(e.G.Stack)-1]); top == nil || top.Ability == nil || top.Source != id {
		t.Fatalf("stack top %+v, want Katara's waterbend ability", top)
	}
}
