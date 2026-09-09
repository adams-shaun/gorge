package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// A stated-quality search may fail to find (701.23b); a quantity-only
// search may not (701.23d). The Evolving Wilds control cannot catch this.
func TestCR701QuantityOnlyTutorMustFindAvailableCard(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Vampiric Tutor")
	_, d := castSearchSpell(t, e, "Vampiric Tutor")
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "search" || len(d.Options) == 0 {
		t.Fatalf("CR 701.23d: real Vampiric Tutor must reach a nonempty search, got %+v", d)
	}
	if d.Min != 1 || d.Max != 1 {
		t.Fatalf("CR 701.23d: quantity-only search with %d available cards offers %d..%d; must find one, not fail to find", len(d.Options), d.Min, d.Max)
	}
}

// The 701.23b side of the same split, pinned so the two directions stay
// discriminated: a search for a stated QUALITY (Evolving Wilds names a basic
// land) keeps the fail-to-find allowance (Min 0) even when matching cards are
// present, and declining is honoured without moving anything but still
// shuffles. Forcing Min up to Max for every search makes this leaf red.
func TestCR701StatedQualitySearchMayFailToFind(t *testing.T) {
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Evolving Wilds")
	wilds := searchMoveByName(t, e, "Evolving Wilds", state.ZBattlefield)
	d := activateSearch(t, e, wilds)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "search" {
		t.Fatalf("CR 701.23b: real Evolving Wilds must reach a search, got %+v", d)
	}
	if len(d.Options) == 0 {
		t.Fatalf("CR 701.23b: fixture has basic lands, options must be nonempty")
	}
	if d.Min != 0 || d.Max != 1 {
		t.Fatalf("CR 701.23b: stated-quality search with %d available cards offers %d..%d; keep Min 0 so the player may fail to find", len(d.Options), d.Min, d.Max)
	}
	// Declining (choosing no cards) must be honoured: nothing moves, but the
	// unconditional shuffle still happens.
	start := len(e.L.Events)
	submitChoices(t, e)
	moves, shuffles := 0, 0
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.MoveZone && ev.From == state.ZLibrary {
			moves++
		}
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			shuffles++
		}
	}
	if moves != 0 || shuffles != 1 {
		t.Fatalf("CR 701.23b: fail-to-find emitted %d library moves and %d shuffles, want 0/1", moves, shuffles)
	}
}
