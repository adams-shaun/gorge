package effects

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The Untap primitive's ConditionZone$ read (Animist's Awakening): the gate's
// ConditionPresent$ spec counts in the NAMED zone, not the battlefield. The
// corpus carrier is the spell-mastery tail -- "If there are two or more
// instant and/or sorcery cards in your graveyard, untap those lands" -- whose
// instants are in the graveyard, a zone the old hardcoded battlefield scan
// never saw, so the untap never fired. The shared conditionMet gate in
// conditions.go deliberately leaves ConditionZone$ unresolved (run-anyway), so
// untapBattlefieldCondition is the only gate that applies.
const untapCondLine = "DB$ Untap | Defined$ Remembered | ConditionPresent$ Instant.YouOwn,Sorcery.YouOwn | ConditionZone$ %s | ConditionCompare$ GE2"

// untapCondBoard builds a 2-seat game with two of seat 0's lands on the
// battlefield tapped and a Ctx remembering them (the Dig's RememberChanged$
// cargo), ready to resolve the SVar line.
func untapCondBoard(t *testing.T) (*fakeHost, *Ctx, []state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	landCard := mkCard(t, "Name:Fixture Land\nTypes:Land\nOracle:x\n")
	var lands []state.ObjID
	for i := 0; i < 2; i++ {
		o := h.g.AddObject(landCard, 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		h.Emit(events.Event{Kind: events.Tap, Obj: o.ID})
		lands = append(lands, o.ID)
	}
	c := &Ctx{Source: lands[0], Controller: 0, Remembered: []state.Target{{Obj: lands[0]}, {Obj: lands[1]}}}
	return h, c, lands
}

// untapCondInstants puts n seat-0 instant fixtures in the given zone and
// returns their IDs.
func untapCondInstants(t *testing.T, h *fakeHost, to state.Zone, n int) []state.ObjID {
	t.Helper()
	card := mkCard(t, "Name:Fixture Instant\nTypes:Instant\nOracle:x\n")
	var ids []state.ObjID
	for i := 0; i < n; i++ {
		o := h.g.AddObject(card, 0)
		h.Emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: to})
		ids = append(ids, o.ID)
	}
	return ids
}

func untapCondCount(h *fakeHost) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.Untap {
			n++
		}
	}
	return n
}

// TestUntapConditionZoneGraveyardMet pins the carrier shape: two instants in
// the graveyard meet GE2 counted in the graveyard, and the remembered tapped
// lands untap.
func TestUntapConditionZoneGraveyardMet(t *testing.T) {
	h, c, lands := untapCondBoard(t)
	untapCondInstants(t, h, state.ZGraveyard, 2)
	Resolve(h, c, sa(t, fmt.Sprintf(untapCondLine, "Graveyard")))
	if got := untapCondCount(h); got != 2 {
		t.Fatalf("untap events = %d, want 2", got)
	}
	for _, id := range lands {
		if h.g.Obj(id).Tapped {
			t.Fatalf("land %d stayed tapped", id)
		}
	}
}

// TestUntapConditionZoneGraveyardUnmet: one instant fails GE2, so the untap
// is skipped and the lands stay tapped -- the under-delivery the corpus
// carrier showed before the fix.
func TestUntapConditionZoneGraveyardUnmet(t *testing.T) {
	h, c, lands := untapCondBoard(t)
	untapCondInstants(t, h, state.ZGraveyard, 1)
	Resolve(h, c, sa(t, fmt.Sprintf(untapCondLine, "Graveyard")))
	if got := untapCondCount(h); got != 0 {
		t.Fatalf("untap events = %d, want 0", got)
	}
	for _, id := range lands {
		if !h.g.Obj(id).Tapped {
			t.Fatalf("land %d untapped despite an unmet condition", id)
		}
	}
}

// TestUntapConditionZoneAbsentCountsBattlefield: with no ConditionZone$ the
// gate keeps counting the battlefield -- the pre-fix default -- so two
// instants on the battlefield meet GE2. A graveyard full of instants alone
// does NOT (the old hardcoded shape, pinned so a future change cannot widen
// silently in the other direction either).
func TestUntapConditionZoneAbsentCountsBattlefield(t *testing.T) {
	line := "DB$ Untap | Defined$ Remembered | ConditionPresent$ Instant.YouOwn,Sorcery.YouOwn | ConditionCompare$ GE2"
	h, c, _ := untapCondBoard(t)
	untapCondInstants(t, h, state.ZBattlefield, 2)
	Resolve(h, c, sa(t, line))
	if got := untapCondCount(h); got != 2 {
		t.Fatalf("untap events = %d, want 2 (battlefield count)", got)
	}

	h2, c2, _ := untapCondBoard(t)
	untapCondInstants(t, h2, state.ZGraveyard, 2)
	Resolve(h2, c2, sa(t, line))
	if got := untapCondCount(h2); got != 0 {
		t.Fatalf("untap events = %d, want 0 (no ConditionZone$ does not see the graveyard)", got)
	}
}

// TestUntapConditionZoneBattlefieldExplicit: "Battlefield" through parseZone
// reproduces the absent behaviour exactly.
func TestUntapConditionZoneBattlefieldExplicit(t *testing.T) {
	h, c, _ := untapCondBoard(t)
	untapCondInstants(t, h, state.ZBattlefield, 2)
	Resolve(h, c, sa(t, fmt.Sprintf(untapCondLine, "Battlefield")))
	if got := untapCondCount(h); got != 2 {
		t.Fatalf("untap events = %d, want 2", got)
	}
}

// TestUntapConditionZoneUnparsableFailsClosed: an unparseable zone name fails
// closed like every other unresolvable gate input here -- no untap, not an
// ungated one.
func TestUntapConditionZoneUnparsableFailsClosed(t *testing.T) {
	h, c, _ := untapCondBoard(t)
	untapCondInstants(t, h, state.ZGraveyard, 2)
	Resolve(h, c, sa(t, fmt.Sprintf(untapCondLine, "GraveYard")))
	if got := untapCondCount(h); got != 0 {
		t.Fatalf("untap events = %d, want 0 (unparsable zone fails closed)", got)
	}
}
