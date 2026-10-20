package rules

// CR 508.4 / CR 110.5b (cr-g11-entering-attacking-untapped): a creature put
// onto the battlefield attacking enters UNTAPPED unless the effect says
// "tapped and attacking". Sigiled Sword of Valeron (repo deck
// cavalry-charge.json) says "create a 2/2 white Knight creature token with
// vigilance that's attacking" and its script carries TokenAttacking$ True
// with NO TokenTapped$; Hero of Bladehold (same deck) says "tapped and
// attacking" and carries TokenTapped$ True. Forge corroborates:
// TokenEffectBase.java:131 taps only on TokenTapped, and
// SpellAbilityEffect.addToCombat (SpellAbilityEffect.java:758) never taps.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// enteringAttackersTokens drives one real Attacks trigger: the named source
// (and, for an Equipment, a Grizzly Bears it is attached to) is on seat 0's
// battlefield, the attacker attacks seat 1, and the stack drains. It returns
// the engine, its config and seat 0's battlefield tokens named tokenName.
func enteringAttackersTokens(t *testing.T, source, tokenName string, equip bool) (*Engine, Config, []*state.Object) {
	t.Helper()
	reg := searchTestRegistry(t)
	extras := []*cards.Card{lookup(t, reg, source)}
	if equip {
		extras = append(extras, lookup(t, reg, "Grizzly Bears"))
	}
	e, cfg := corpusEngineCfg(t, reg, extras, []*cards.Card{})
	src := moveByName(t, e, 0, source, state.ZBattlefield)
	attacker := src
	if equip {
		attacker = moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
		e.emit(events.Event{Kind: events.Attach, Obj: src, IDs: []state.ObjID{attacker}})
		if o := e.G.Obj(src); o == nil || o.AttachedTo != attacker {
			t.Fatalf("precondition: %s attached to %v, want the Bears %d", source, o, attacker)
		}
	}
	if o := e.G.Obj(attacker); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: attacker = %+v, want battlefield under seat 0", o)
	}
	// Player in a DeclareAttackers event is the DEFENDING seat
	// (trigger_referents.go binds DefendingPlayer = ev.Player).
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{attacker}})
	e.putTriggersOnStack()
	// Hero of Bladehold also has battle cry: two triggers ask an order.
	answerTriggerOrders(t, e)
	if len(e.G.Stack) == 0 {
		t.Fatalf("%s's Attacks trigger did not reach the stack", source)
	}
	passUntilStackEmpty(t, e, 40)
	var toks []*state.Object
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == tokenName {
			toks = append(toks, o)
		}
	}
	return e, cfg, toks
}

// TestCR508_4SigiledSwordKnightEntersAttackingUntapped is the failing pin:
// "that's attacking" (no "tapped") leaves the Knight untapped (CR 110.5b),
// so it attacks seat 1 and is still untapped.
func TestCR508_4SigiledSwordKnightEntersAttackingUntapped(t *testing.T) {
	t.Parallel()
	e, cfg, knights := enteringAttackersTokens(t, "Sigiled Sword of Valeron", "Knight Token", true)
	if len(knights) != 1 {
		t.Fatalf("Sigiled Sword minted %d Knight tokens, want 1", len(knights))
	}
	k := knights[0]
	if !k.IsAttacking || k.Attacking != 1 {
		t.Fatalf("Knight attacking=%v defender=%d, want attacking seat 1", k.IsAttacking, k.Attacking)
	}
	if k.Tapped {
		t.Fatalf("Knight is tapped; CR 508.4/110.5b: a token created \"attacking\" (not \"tapped and attacking\") enters untapped")
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Tap && ev.Obj == k.ID {
			t.Fatalf("Knight logged a Tap event %+v; the effect never taps it", ev)
		}
	}
	replayCheck(t, e, cfg)
}

// TestCR508_4HeroOfBladeholdSoldiersEnterTappedAndAttacking is the guard:
// "tapped and attacking" (TokenTapped$ True) still enters tapped, through
// the token's own entry Tap, and attacking seat 1.
func TestCR508_4HeroOfBladeholdSoldiersEnterTappedAndAttacking(t *testing.T) {
	t.Parallel()
	e, cfg, soldiers := enteringAttackersTokens(t, "Hero of Bladehold", "Soldier Token", false)
	if len(soldiers) != 2 {
		t.Fatalf("Hero of Bladehold minted %d Soldier tokens, want 2", len(soldiers))
	}
	for _, s := range soldiers {
		if !s.Tapped || !s.IsAttacking || s.Attacking != 1 {
			t.Fatalf("Soldier tapped=%v attacking=%v defender=%d, want tapped and attacking seat 1",
				s.Tapped, s.IsAttacking, s.Attacking)
		}
	}
	replayCheck(t, e, cfg)
}
