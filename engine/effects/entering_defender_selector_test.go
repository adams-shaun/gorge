package effects

// cr-g11b review finding (MAJOR): a non-True TokenAttacking$ selector names
// its own defender, so CopyPermanent must keep that seat for the CopyToken
// fold. The True arm alone owns CR 508.4's chooser.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCopyPermanentSelectorKeepsTheNamedDefender pins the selector arm:
// TokenAttacking$ RememberedPlayer with a remembered seat 1 mints the copy
// attacking seat 1 (not the fold's zero seat).
func TestCopyPermanentSelectorKeepsTheNamedDefender(t *testing.T) {
	h, c := fixtureHostWithTokens(t)
	src := h.g.Obj(c.Source)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: src.ID, From: src.Zone, To: state.ZBattlefield})
	if o := h.g.Obj(c.Source); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source = %+v, want battlefield", o)
	}
	c.Remembered = []state.Target{{Player: 1, IsPlayer: true}}

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
		t.Fatalf("no token copy minted; events=%+v", h.log)
	}
	if !copy.IsAttacking || copy.Attacking != 1 {
		t.Fatalf("copy attacking=%v defender=%d, want attacking the remembered seat 1", copy.IsAttacking, copy.Attacking)
	}
	if n := notesContaining(h, "TokenAttacking$ RememberedPlayer is not implemented"); n != 0 {
		t.Fatalf("%d degrade notes on a resolved selector, want 0; log=%+v", n, h.log)
	}
}
