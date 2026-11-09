package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEffectRememberTriggeredAttacker pins an Effect's
// RememberObjects$ TriggeredAttacker (Gravelgill Scoundrel: tap another
// creature and the attacker can't be blocked this turn). Forge resolves every
// Triggered* word alike, so the live-attacker spelling must capture the
// creature the Attacks trigger fired for exactly as the LKI-copy spelling
// does, and the registered CantBlockBy restriction must carry it.
func TestEffectRememberTriggeredAttacker(t *testing.T) {
	card, sa := corpusSA(t, "Gravelgill Scoundrel", "TrigUnblockable")
	if sa.Params["RememberObjects"] != "TriggeredAttacker" {
		t.Fatalf("corpus selector changed: %q", sa.Params["RememberObjects"])
	}
	h := newHost(t, 2)
	srcObj := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: srcObj.ID, From: state.ZLibrary, To: state.ZBattlefield})
	other := h.g.AddObject(mkCard(t, "Name:Other\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: other.ID, From: state.ZLibrary, To: state.ZBattlefield})
	attacker := srcObj.ID
	if attacker == other.ID || h.g.Obj(attacker).Zone != state.ZBattlefield || h.g.Obj(other.ID).Zone != state.ZBattlefield {
		t.Fatal("precondition: attacker and tap-cost creature must be distinct battlefield objects")
	}

	// The trigger captured the attacker; a chosen target names a different
	// object, so a fallback to it (or to the source) is caught.
	ctx := &Ctx{Source: srcObj.ID, Controller: 0, SVars: h.g.Obj(attacker).Face().SVars,
		Remembered: []state.Target{{Obj: attacker}},
		Targets:    []state.Target{{Obj: other.ID}}}
	got := effectRemembered(h, ctx, sa)
	if len(got) != 1 || got[0] != attacker {
		t.Fatalf("remembered %v, want [%d] (the triggering attacker)", got, attacker)
	}

	// A capture-free context remembers nothing: no source or target guess.
	if got := effectRemembered(h, &Ctx{Source: srcObj.ID, Controller: 0, Targets: []state.Target{{Obj: other.ID}}}, sa); len(got) != 0 {
		t.Fatalf("no captured attacker remembered %v, want none", got)
	}

	// Resolving the Effect registers the restriction carrying the attacker.
	Resolve(h, ctx, sa)
	var reg *state.ContinuousEffect
	for i := range h.continuous {
		if h.continuous[i].Restriction == "CantBlockBy" {
			reg = &h.continuous[i]
		}
	}
	if reg == nil {
		t.Fatalf("no CantBlockBy restriction registered: %+v", h.continuous)
	}
	if len(reg.Remembered) != 1 || reg.Remembered[0] != attacker {
		t.Fatalf("restriction remembers %v, want [%d]", reg.Remembered, attacker)
	}
}
