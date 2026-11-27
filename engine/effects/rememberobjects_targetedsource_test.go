package effects

// RememberObjects$ TargetedSource names the SOURCE CARD of a targeted stack
// ability when the chosen target is an ability wrapper, and the target itself
// when it is already a card (a spell). Dromoka's Command's DBPrevent is the
// corpus carrier: it targets an instant/sorcery spell on the stack, and the
// Effect's Card.IsRemembered static must name the card that spell's damage
// comes from (the spell object itself, or the source card of a targeted stack
// ability). The translation is the same one ImprintCards$ TargetedSource
// applies, so both reads go through one helper.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestEffectRememberedTargetedSource(t *testing.T) {
	_, sa := corpusSA(t, "Dromoka's Command", "DBPrevent")
	if got := sa.Params["RememberObjects"]; got != "TargetedSource" {
		t.Fatalf("corpus selector changed: %q", got)
	}
	h, ctx, ids := validStackBoard(t)

	own := h.g.Obj(ids.own)
	triggered := h.g.Obj(ids.triggered)
	if own == nil || own.Zone != state.ZStack || triggered == nil || triggered.Zone != state.ZStack {
		t.Fatalf("precondition: both compared targets must be on the stack: own=%+v triggered=%+v", own, triggered)
	}
	if own.Source != 0 {
		t.Fatalf("precondition: the spell %d must carry no source card, got %d", ids.own, own.Source)
	}
	if triggered.Source == 0 || h.g.Obj(triggered.Source) == nil {
		t.Fatalf("precondition: the triggered ability %d must name a live source card, got %d", ids.triggered, triggered.Source)
	}
	if ids.own == ids.triggered || ids.own == triggered.Source || ids.triggered == triggered.Source {
		t.Fatalf("precondition: spell, ability and source card must be distinct: spell=%d ability=%d source=%d",
			ids.own, ids.triggered, triggered.Source)
	}

	// A plain spell target is already the card: remember the spell.
	ctx.PickedTargets = []state.Target{{Obj: ids.own}}
	if got := effectRemembered(h, ctx, sa); len(got) != 1 || got[0] != ids.own {
		t.Fatalf("TargetedSource captured %v for the spell %d, want the spell itself", got, ids.own)
	}

	// A targeted stack ABILITY resolves to the card that created it, not to
	// the wrapper: the static must name the source card.
	ctx.PickedTargets = []state.Target{{Obj: ids.triggered}}
	if got := effectRemembered(h, ctx, sa); len(got) != 1 || got[0] != triggered.Source {
		t.Fatalf("TargetedSource captured %v for the ability %d (source %d), want the source card", got, ids.triggered, triggered.Source)
	}
}
