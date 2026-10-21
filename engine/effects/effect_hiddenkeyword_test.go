package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEffectHiddenKeywordRegisters pins the Effect-delivery half of the
// AddHiddenKeyword$ read (Falter, Magmatic Chasm, Seismic Stomp, Barrage of
// Boulders, Order): the body's sentence must become a layer-6 keyword grant
// the combat restriction reader sees, not the unimplemented Note.
func TestEffectHiddenKeywordRegisters(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Controller: 0, Source: 1, SVars: map[string]string{
		"KWPump": "Mode$ Continuous | AffectedZone$ Battlefield | Affected$ Creature.withoutFlying | AddHiddenKeyword$ CARDNAME can't block. | Description$ Creatures without flying can't block this turn.",
	}}
	Resolve(h, c, sa(t, "DB$ Effect | StaticAbilities$ KWPump"))
	if len(h.continuous) != 1 {
		t.Fatalf("continuous = %+v, want one AddHiddenKeyword registration", h.continuous)
	}
	ce := h.continuous[0]
	if ce.Layer != state.LAbilities || len(ce.AddKeywords) != 1 || ce.AddKeywords[0] != "CARDNAME can't block." {
		t.Fatalf("registration = %+v, want layer-6 CARDNAME can't block.", ce)
	}
	if ce.Affects != "Creature.withoutFlying" || ce.AffectedZone != "Battlefield" {
		t.Fatalf("registration scope = %+v, want Creature.withoutFlying on the battlefield", ce)
	}
	if !ce.UntilEOT || ce.Permanent {
		t.Fatalf("absent-Duration registration = %+v, want this-turn (UntilEOT), not Permanent", ce)
	}
	if !ce.FromEffect {
		t.Fatalf("registration = %+v, want FromEffect", ce)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented") {
			t.Fatalf("readable AddHiddenKeyword still reports unimplemented: %q", ev.Text)
		}
	}
}

// TestEffectHiddenKeywordUnreadValueNotes is the fail-closed half: a sentence
// outside the shared vocabulary must register nothing and keep the honest
// unimplemented Note.
func TestEffectHiddenKeywordUnreadValueNotes(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Controller: 0, Source: 1, SVars: map[string]string{
		"KWPump": "Mode$ Continuous | Affected$ Creature | AddHiddenKeyword$ CARDNAME can't block unless it's your turn. | Description$ gated",
	}}
	Resolve(h, c, sa(t, "DB$ Effect | StaticAbilities$ KWPump"))
	if len(h.continuous) != 0 {
		t.Fatalf("unreadable AddHiddenKeyword registered: %+v", h.continuous)
	}
	found := false
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented") {
			found = true
		}
	}
	if !found {
		t.Fatalf("unreadable AddHiddenKeyword produced no unimplemented Note: %+v", h.log)
	}
}

// TestEffectAdjustLandPlaysRegisters pins the Effect-delivery half of the
// AdjustLandPlays$ read (Explore, Scale the Heights, Urban Evolution): the
// value must reach rules' additional-land-drops sum, not the Note.
func TestEffectAdjustLandPlaysRegisters(t *testing.T) {
	h := newHost(t, 2)
	c := &Ctx{Controller: 0, Source: 1, SVars: map[string]string{
		"Exploration": "Mode$ Continuous | Affected$ You | AdjustLandPlays$ 1 | Description$ You may play an additional land this turn.",
	}}
	Resolve(h, c, sa(t, "DB$ Effect | StaticAbilities$ Exploration"))
	if len(h.continuous) != 1 {
		t.Fatalf("continuous = %+v, want one AdjustLandPlays registration", h.continuous)
	}
	ce := h.continuous[0]
	if ce.AdjustLandPlays != 1 || ce.Affects != "You" {
		t.Fatalf("registration = %+v, want +1 land drop for You", ce)
	}
	if !ce.UntilEOT || ce.Permanent {
		t.Fatalf("absent-Duration registration = %+v, want this-turn (UntilEOT), not Permanent", ce)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented") {
			t.Fatalf("readable AdjustLandPlays still reports unimplemented: %q", ev.Text)
		}
	}
}
