package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestCloneCarriesManaRelevantFold: a clone inherits its parent's
// mana-relevant event fold (manaMemberCarry.advanceRelevant) instead of
// re-folding the shared log prefix from zero, and the count it then reports
// is exactly a fresh fold over its own log, appends included.
func TestCloneCarriesManaRelevantFold(t *testing.T) {
	e, _ := fdnCensusEngine(t, "Grizzly Bears")
	e.ownManaCarry()
	want := e.manaBoardStamp().manaRelevant
	if want == 0 || e.manaCarry.scanPos != len(e.L.Events) {
		t.Fatalf("parent fold %d at %d of %d events: nothing to carry", want, e.manaCarry.scanPos, len(e.L.Events))
	}
	for _, c := range []*Engine{e.Clone(), e.hypClone(e)} {
		if c.manaCarry.owner != c || c.manaCarry.scanPos != e.manaCarry.scanPos || c.manaCarry.relevant != want {
			t.Fatalf("clone fold at %d count %d (owner ok %v), parent at %d count %d",
				c.manaCarry.scanPos, c.manaCarry.relevant, c.manaCarry.owner == c, e.manaCarry.scanPos, want)
		}
		c.emit(events.Event{Kind: events.Note, Text: "quiet"})
		c.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: -1})
		fresh := (&manaMemberCarry{}).advanceRelevant(c.L.Events)
		if got := c.manaBoardStamp().manaRelevant; got != fresh || got != want+1 {
			t.Fatalf("clone count %d, fresh fold %d, want %d", got, fresh, want+1)
		}
	}
}
