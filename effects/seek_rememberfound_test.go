package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestSeekRememberFoundReplacesTriggerCapture is Goblin Trapfinder's shape: a
// ChangesZone trigger captures its own dying card in both the resolution's
// Remembered and Captured sets (rules/trigger_match.go seeds both), then its
// DBSeek SVar runs DB$ Seek | RememberFound$ True followed by DBAnimate with
// Defined$ Remembered. RememberFound$ must make the FOUND card the
// resolution's Remembered set -- if it only appends, the trigger's captured
// dying Trapfinder stays in Remembered and the chained perpetual
// haste/reduce-cost grant acts on the dying card as well as the found
// creature (the player-visible failure). Same defect shape as the merged
// DigUntil fix (c1d996d4), one primitive over. The trigger referent must
// remain reachable through the separate Captured channel.
func TestSeekRememberFoundReplacesTriggerCapture(t *testing.T) {
	h := newSeekHost(t, 2, 0)
	src := seekSource(t, h.fakeHost)
	creature := mkCard(t, "Name:Alpha\nTypes:Creature\nPT:1/1\nOracle:x\n")
	found := seekCards(t, h.fakeHost, 0, creature)[0]
	// The trigger's captured object: a distinct battlefield card (the dying
	// Goblin Trapfinder itself, in the real card). It must be a DIFFERENT id
	// from the found card, or this test proves nothing.
	referent := h.g.AddObject(mkCard(t, "Name:Doctor\nTypes:Legendary Creature\nPT:3/5\nOracle:x\n"), 0).ID
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), referent))
	if referent == found {
		t.Fatalf("fixture precondition failed: trigger object %d equals the found card %d", referent, found)
	}
	trigger := state.Target{Obj: referent}
	c := &Ctx{Controller: 0, Source: src, Remembered: []state.Target{trigger}, Captured: []state.Target{trigger}}
	// Precondition: the found card really matches the Type$ Creature spec
	// the seek reads.
	if !MatchesSpecCtx(h.g, "Creature", found, c.SpecContext(0)) {
		t.Fatalf("fixture precondition failed: library card %d does not match Type$ Creature", found)
	}
	// Precondition: the trigger object is present in BOTH channels before the
	// effect runs, so the assertions below are about replacement/preservation
	// and not about an already-empty set.
	if len(c.Remembered) != 1 || c.Remembered[0] != trigger || len(c.Captured) != 1 || c.Captured[0] != trigger {
		t.Fatalf("fixture precondition failed: trigger not seeded in both channels: Remembered=%v Captured=%v", c.Remembered, c.Captured)
	}
	Resolve(h, c, sa(t, "SP$ Seek | Type$ Creature | RememberFound$ True"))
	if o := h.g.Obj(found); o.Zone != state.ZHand {
		t.Fatalf("found card zone = %s, want hand", o.Zone)
	}
	// The found card is the ONLY entry left in the resolution's Remembered
	// set.
	if len(c.Remembered) != 1 || c.Remembered[0].Obj != found {
		t.Fatalf("Remembered = %v, want exactly the found card %d (the trigger capture must NOT remain)", c.Remembered, found)
	}
	for _, mem := range c.Remembered {
		if mem.Obj == referent {
			t.Fatalf("Remembered still contains the trigger-captured object %d: chained Defined$ Remembered would act on the dying card, not the found card", referent)
		}
	}
	// The trigger referent is NOT lost: it survives in the separate Captured
	// channel the Ctx doc describes.
	if len(c.Captured) != 1 || c.Captured[0].Obj != referent {
		t.Fatalf("Captured = %v, want the trigger object %d preserved", c.Captured, referent)
	}
}

// TestSeekRememberFoundEmptyFoundClearsCtxRemembered covers the empty-found
// case: with RememberFound$ True and nothing eligible, the resolution's
// Remembered set REPLACES to empty -- it must not retain the trigger capture,
// or a chained Defined$ Remembered would still act on the triggering object.
func TestSeekRememberFoundEmptyFoundClearsCtxRemembered(t *testing.T) {
	h := newSeekHost(t, 2)
	src := seekSource(t, h.fakeHost)
	land := mkCard(t, "Name:Waste\nTypes:Land\nOracle:x\n")
	landID := seekCards(t, h.fakeHost, 0, land)[0]
	referent := h.g.AddObject(mkCard(t, "Name:Doctor\nTypes:Legendary Creature\nPT:3/5\nOracle:x\n"), 0).ID
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), referent))
	trigger := state.Target{Obj: referent}
	c := &Ctx{Controller: 0, Source: src, Remembered: []state.Target{trigger}, Captured: []state.Target{trigger}}
	// Precondition: the library holds NO Creature, so the seek finds nothing.
	if MatchesSpecCtx(h.g, "Creature", landID, c.SpecContext(0)) {
		t.Fatalf("fixture precondition failed: library card %d matches Type$ Creature; the seek must find nothing", landID)
	}
	if len(c.Remembered) != 1 || c.Remembered[0] != trigger || len(c.Captured) != 1 || c.Captured[0] != trigger {
		t.Fatalf("fixture precondition failed: trigger not seeded in both channels: Remembered=%v Captured=%v", c.Remembered, c.Captured)
	}
	Resolve(h, c, sa(t, "SP$ Seek | Type$ Creature | RememberFound$ True"))
	if len(c.Remembered) != 0 {
		t.Fatalf("Remembered = %v, want EMPTY after a found-nothing seek (the trigger capture must NOT remain)", c.Remembered)
	}
	if len(c.Captured) != 1 || c.Captured[0].Obj != referent {
		t.Fatalf("Captured = %v, want the trigger object %d preserved", c.Captured, referent)
	}
}

// TestSeekRememberFoundMultiFoundAccumulates covers the multi-found case
// (Num$ 2): BOTH found cards are the resolution's Remembered set, still with
// no trigger referent.
func TestSeekRememberFoundMultiFoundAccumulates(t *testing.T) {
	h := newSeekHost(t, 2, 1, 0)
	src := seekSource(t, h.fakeHost)
	creA := mkCard(t, "Name:Alpha\nTypes:Creature\nPT:1/1\nOracle:x\n")
	creB := mkCard(t, "Name:Beta\nTypes:Creature\nPT:1/1\nOracle:x\n")
	creC := mkCard(t, "Name:Gamma\nTypes:Creature\nPT:1/1\nOracle:x\n")
	// Top first: A, B, C. Draws 1 then 0 pick B then A.
	ids := seekCards(t, h.fakeHost, 0, creA, creB, creC)
	referent := h.g.AddObject(mkCard(t, "Name:Doctor\nTypes:Legendary Creature\nPT:3/5\nOracle:x\n"), 0).ID
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), referent))
	if referent == ids[0] || referent == ids[1] {
		t.Fatalf("fixture precondition failed: trigger object %d equals a found card", referent)
	}
	trigger := state.Target{Obj: referent}
	c := &Ctx{Controller: 0, Source: src, Remembered: []state.Target{trigger}, Captured: []state.Target{trigger}}
	// Precondition: the trigger object is present in BOTH channels before the
	// effect runs.
	if len(c.Remembered) != 1 || c.Remembered[0] != trigger || len(c.Captured) != 1 || c.Captured[0] != trigger {
		t.Fatalf("fixture precondition failed: trigger not seeded in both channels: Remembered=%v Captured=%v", c.Remembered, c.Captured)
	}
	Resolve(h, c, sa(t, "SP$ Seek | Type$ Creature | Num$ 2 | RememberFound$ True"))
	if len(h.g.Zone(state.ZHand, 0)) != 2 {
		t.Fatalf("hand = %v, want exactly the two scripted found cards {%d,%d}", h.g.Zone(state.ZHand, 0), ids[1], ids[0])
	}
	// Both found cards, and nothing else, are the resolution's Remembered set.
	if len(c.Remembered) != 2 || c.Remembered[0].Obj != ids[1] || c.Remembered[1].Obj != ids[0] {
		t.Fatalf("Remembered = %v, want exactly the two found cards {%d,%d} (the trigger capture must NOT remain)", c.Remembered, ids[1], ids[0])
	}
	for _, mem := range c.Remembered {
		if mem.Obj == referent {
			t.Fatalf("Remembered still contains the trigger-captured object %d", referent)
		}
	}
	if len(c.Captured) != 1 || c.Captured[0].Obj != referent {
		t.Fatalf("Captured = %v, want the trigger object %d preserved", c.Captured, referent)
	}
}
