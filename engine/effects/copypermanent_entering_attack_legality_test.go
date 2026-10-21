// cr-g55 review finding: CopyPermanent's TokenAttacking$ rider is an
// "enters attacking" emitter too, so it must re-check a captured defender at
// the moment of entry (CR 506.3c, 508.4a-b). The battle half of
// EnteringAttackLegal is pinned here in both directions: an unprotected
// battle and a battle whose protector has left the game make no attacker.
package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// copyAttackingASeat mints one copy of the fixture source attacking the
// remembered seat 1, after lost marks seat 1 as departed. It returns the
// minted copy, whose precondition (a battlefield token) the caller asserts.
func copyAttackingASeat(t *testing.T, lost bool) *state.Object {
	t.Helper()
	h, c := fixtureHostWithTokens(t)
	src := h.g.Obj(c.Source)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: src.Zone, To: state.ZBattlefield})
	if o := h.g.Obj(c.Source); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source = %+v, want battlefield", o)
	}
	c.Remembered = []state.Target{{Player: 1, IsPlayer: true}}
	if lost {
		h.g.Players[1].Lost = true
	}
	effCopyPermanent(h, c, &cards.SA{API: "CopyPermanent", Params: map[string]string{
		"Defined": "Self", "TokenAttacking": "RememberedPlayer",
	}})
	var copy *state.Object
	for i := range h.g.Objs {
		o := &h.g.Objs[i]
		if o.IsToken && o.Zone == state.ZBattlefield {
			copy = o
		}
	}
	if copy == nil {
		t.Fatalf("precondition: no token copy minted; events=%+v", h.log)
	}
	return copy
}

// TestCopyPermanentTokenAttackingSkipsADepartedDefender: the copy enters but
// never becomes an attacking creature when its remembered defender has left
// the game. The live-seat guard proves the only difference is the departure.
func TestCopyPermanentTokenAttackingSkipsADepartedDefender(t *testing.T) {
	if copy := copyAttackingASeat(t, true); copy.IsAttacking {
		t.Fatalf("CR 508.4b: copy attacking=%v defender=%d at a departed player, want not attacking",
			copy.IsAttacking, copy.Attacking)
	}
	if copy := copyAttackingASeat(t, false); !copy.IsAttacking || copy.Attacking != 1 {
		t.Fatalf("guard: copy attacking=%v defender=%d, want attacking the live remembered seat 1",
			copy.IsAttacking, copy.Attacking)
	}
}

// TestEnteringAttackLegalBattleProtector pins the battle branch: a battle is
// a legal defender only while it is protected by the named seat. No recorded
// protector (the CR 310.10 choice not made), a protector that changed, and a
// protector that left the game all make the entry non-attacking.
func TestEnteringAttackLegalBattleProtector(t *testing.T) {
	h := newHost(t, 3)
	battle := ufBattlefield(t, h, "Name:Test Siege\nTypes:Battle\nOracle:x\n")
	if !containsFold(h.Chars(battle).Types, "Battle") {
		t.Fatalf("precondition: battle types = %v, want Battle", h.Chars(battle).Types)
	}
	// No CR 310.10 protector recorded yet: unprotected, so no attacker.
	if EnteringAttackLegal(h, 1, battle) {
		t.Fatal("unprotected battle accepted a defender")
	}
	h.Emit(events.Event{Kind: events.Choose, Obj: battle, Counter: "protector", Player: 1})
	o := h.g.Obj(battle)
	if o == nil || !o.ProtectorValid || o.Protector != 1 {
		t.Fatalf("precondition: battle protector valid=%v seat=%d, want true/1", o.ProtectorValid, o.Protector)
	}
	if !EnteringAttackLegal(h, 1, battle) {
		t.Fatal("battle protected by seat 1 rejected its own defender")
	}
	h.Emit(events.Event{Kind: events.Choose, Obj: battle, Counter: "protector", Player: 2})
	if EnteringAttackLegal(h, 1, battle) {
		t.Fatal("battle accepted seat 1 after its protector changed to seat 2")
	}
	if !EnteringAttackLegal(h, 2, battle) {
		t.Fatal("battle rejected its current protector seat 2")
	}
}

// TestEnteringAttackLegalBattleProtectorLeft: a battle whose recorded
// protector has left the game is no longer protected by a defending player.
func TestEnteringAttackLegalBattleProtectorLeft(t *testing.T) {
	h := newHost(t, 3)
	battle := ufBattlefield(t, h, "Name:Test Siege\nTypes:Battle\nOracle:x\n")
	h.Emit(events.Event{Kind: events.Choose, Obj: battle, Counter: "protector", Player: 1})
	if o := h.g.Obj(battle); o == nil || !o.ProtectorValid || o.Protector != 1 {
		t.Fatalf("precondition: battle protector valid=%v seat=%d, want true/1", o.ProtectorValid, o.Protector)
	}
	h.g.Players[1].Lost = true
	if EnteringAttackLegal(h, 1, battle) {
		t.Fatal("CR 508.4b: battle accepted a protector that left the game")
	}
}
