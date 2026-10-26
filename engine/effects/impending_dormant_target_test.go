package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestImpendingDormantIsNotCreatureTarget pins CR 702.176a on the effects-side
// type fallback the target walk reaches: while Overlord of the Mistmoors is
// impending-dormant (cast for its impending cost, a time counter on it), the
// ValidTgts$ of Murder, Unsummon and Shock ("Creature", "Creature", "Any")
// admit no creature -- it is an Enchantment only -- and with the last time
// counter gone the same three specs find it again.
func TestImpendingDormantIsNotCreatureTarget(t *testing.T) {
	reg := choiceCorpusRegistry(t)
	overlord, ok := reg.Lookup("Overlord of the Mistmoors")
	if !ok {
		t.Fatal("corpus has no Overlord of the Mistmoors")
	}
	specs := map[string]string{}
	for _, name := range []string{"Murder", "Unsummon", "Shock"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("corpus has no %q", name)
		}
		sa := c.Faces[0].Abilities[0]
		spec, _ := sa.Param(cards.PKValidTgts)
		if spec == "" {
			t.Fatalf("%s has no ValidTgts$", name)
		}
		specs[name] = spec
	}

	h, c := fixtureHost(t)
	o := h.g.AddObject(overlord, 1)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	eq := h.g.AddObject(mkCard(t, "Name:Test Blade\nTypes:Artifact Equipment\nOracle:x\n"), 0)
	live := h.g.Obj(o.ID)
	live.CastFlags |= state.FlagImpending
	live.AddCounter("TIME", 4)
	if !live.ImpendingDormant() {
		t.Fatal("setup: the Overlord is not impending-dormant")
	}

	offers := func(name string) bool {
		for _, tg := range battlefieldValidTargets(h, c, specs[name]) {
			if tg.Obj == o.ID {
				return true
			}
		}
		return false
	}
	for name := range specs {
		if offers(name) {
			t.Errorf("%s (%s) offers the dormant Overlord as a target", name, specs[name])
		}
	}
	if attachableTo(h.g, h.g.Obj(eq.ID), live) {
		t.Error("an Equipment is attachable to the dormant Overlord")
	}
	for _, w := range []string{"Creature", "Avatar", "Horror"} {
		if hasType(live, w) {
			t.Errorf("hasType(dormant Overlord, %s) = true, want false", w)
		}
	}
	if !hasType(live, "Enchantment") {
		t.Error("hasType(dormant Overlord, Enchantment) = false, want true")
	}

	live.AddCounter("TIME", -4)
	if live.ImpendingDormant() {
		t.Fatal("setup: the Overlord is still dormant with no time counters")
	}
	if !attachableTo(h.g, h.g.Obj(eq.ID), live) {
		t.Error("an Equipment is not attachable to the awakened Overlord")
	}
	for name := range specs {
		if !offers(name) {
			t.Errorf("%s (%s) does not offer the awakened Overlord", name, specs[name])
		}
	}
}
