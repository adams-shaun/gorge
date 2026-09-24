package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestDigUntilRememberFoundDoesNotRetainTriggerCapture is The Tenth Doctor's
// shape: an AttackersDeclared trigger captures the attacking Doctor in both
// the resolution's Remembered and Captured sets, then its TrigExile SVar runs
// DB$ DigUntil | ... | RememberFound$ True, followed by DBPutCounter with
// Defined$ Remembered. RememberFound$ must make the FOUND card the
// resolution's Remembered set -- if it only appends, the trigger's captured
// Doctor stays in Remembered and the chained Defined$ Remembered puts the
// TIME counters on the Doctor (the player-visible failure) instead of the
// exiled nonland card. The trigger referent must remain reachable through
// the separate Captured channel.
func TestDigUntilRememberFoundDoesNotRetainTriggerCapture(t *testing.T) {
	h, ids := digUntilFixture(t)
	// The trigger's captured object: a distinct creature on the battlefield
	// (The Tenth Doctor itself, in the real card). It must be a DIFFERENT id
	// from the found card, or this test proves nothing.
	doctor := h.g.AddObject(mkCard(t, "Name:Doctor\nTypes:Legendary Creature Doctor\nPT:3/5\nOracle:x\n"), 0).ID
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), doctor))
	if doctor == ids[1] {
		t.Fatalf("fixture precondition failed: trigger object %d equals the found card %d", doctor, ids[1])
	}
	trigger := state.Target{Obj: doctor}
	c := &Ctx{Controller: 0, Remembered: []state.Target{trigger}, Captured: []state.Target{trigger}}
	// Precondition: the found card really matches the Valid$ Aura spec the
	// rule reads, and the fixture's scan must reach the SECOND card (the
	// nonmatching land is first), so a first-card match cannot pass by
	// coincidence.
	if !MatchesSpecCtx(h.g, "Aura", ids[1], c.SpecContext(0)) {
		t.Fatalf("fixture precondition failed: card %d does not match Valid$ Aura", ids[1])
	}
	if MatchesSpecCtx(h.g, "Aura", ids[0], c.SpecContext(0)) {
		t.Fatalf("fixture precondition failed: first library card %d matches Valid$ Aura; the match must be the second card", ids[0])
	}
	// Precondition: the trigger object is present in both channels before the
	// effect runs, so the assertions below are about removal/preservation and
	// not about an already-empty set.
	if len(c.Remembered) != 1 || c.Remembered[0] != trigger || len(c.Captured) != 1 || c.Captured[0] != trigger {
		t.Fatalf("fixture precondition failed: trigger not seeded in both channels: Remembered=%v Captured=%v", c.Remembered, c.Captured)
	}
	Resolve(h, c, sa(t, "SP$ DigUntil | Valid$ Aura | FoundDestination$ Exile | RevealedDestination$ Exile | RememberFound$ True"))
	// The found card went to exile (FoundDestination$ Exile) and is the ONLY
	// entry left in the resolution's Remembered set.
	if o := h.g.Obj(ids[1]); o.Zone != state.ZExile {
		t.Fatalf("found card zone = %s, want exile", o.Zone)
	}
	if len(c.Remembered) != 1 || c.Remembered[0].Obj != ids[1] {
		t.Fatalf("Remembered = %v, want exactly the found card %d (the trigger capture must NOT remain)", c.Remembered, ids[1])
	}
	for _, mem := range c.Remembered {
		if mem.Obj == doctor {
			t.Fatalf("Remembered still contains the trigger-captured object %d: chained Defined$ Remembered would act on the Doctor, not the found card", doctor)
		}
	}
	// The trigger referent is NOT lost: it survives in the separate Captured
	// channel the Ctx doc describes.
	if len(c.Captured) != 1 || c.Captured[0].Obj != doctor {
		t.Fatalf("Captured = %v, want the trigger object %d preserved", c.Captured, doctor)
	}
}
