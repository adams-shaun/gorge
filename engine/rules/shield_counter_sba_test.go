package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestShieldCounterSpentOnSBALethal: CR 122.1c's destruction replacement
// applies to the CR 704.5g lethal-damage state-based action, not only to
// Destroy effects. Spider-Punk makes the damage unpreventable, so the shield
// counter's prevention cannot stop it: the damage is marked, the SBA finds
// the creature lethal, and the destruction is replaced by removing the
// shield counter. With no shield counter left the same SBA destroys it.
func TestShieldCounterSpentOnSBALethal(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	spider := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Spider-Punk"))
	shielded := onBoard(t, e, 1, "Name:Shielded Bear\nTypes:Creature Bear\nPT:2/3\nOracle:x\n")
	bare := onBoard(t, e, 1, "Name:Bare Bear\nTypes:Creature Bear\nPT:2/3\nOracle:x\n")
	seedObjectCounter(t, e, shielded, "SHIELD", 1)

	e.damaging = spider
	e.emit(events.Event{Kind: events.Damage, Obj: shielded, Amount: 3})
	e.emit(events.Event{Kind: events.Damage, Obj: bare, Amount: 3})
	e.damaging = 0

	// Precondition: the damage really landed (unpreventable) and is lethal
	// for both, and the shielded one still holds its counter when the SBA
	// runs. Without this the assertion below could pass vacuously.
	for _, id := range []state.ObjID{shielded, bare} {
		o := e.G.Obj(id)
		if o.Zone != state.ZBattlefield || o.Damage != 3 {
			t.Fatalf("precondition: creature %d zone %v damage %d, want battlefield and 3 unpreventable damage", id, o.Zone, o.Damage)
		}
	}
	if n := e.G.Obj(shielded).Counter("SHIELD"); n != 1 {
		t.Fatalf("precondition: shielded creature has %d shield counters, want 1", n)
	}

	e.checkStateBased()

	o := e.G.Obj(shielded)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("the lethal-damage SBA destroyed the shielded creature (zone %v) instead of removing its shield counter", o.Zone)
	}
	if n := o.Counter("SHIELD"); n != 0 {
		t.Fatalf("shield counters after the replaced SBA destruction = %d, want 0", n)
	}
	if z := e.G.Obj(bare).Zone; z != state.ZGraveyard {
		t.Fatalf("the creature with no shield counter: zone %v, want graveyard", z)
	}
}
