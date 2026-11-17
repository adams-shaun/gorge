package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Nerd Rage's enchanted creature has "You have no maximum hand size": the
// granted static's Affected$ You is the RECIPIENT's controller, so the
// creature's controller loses the maximum, whoever cast the Aura.
func TestGrantedSetMaxHandSizeReadsRecipientController(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	nerd, ok := reg.Lookup("Nerd Rage")
	if !ok {
		t.Fatal("corpus fixture: Nerd Rage missing")
	}
	const bearSrc = "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	for _, tc := range []struct {
		name         string
		bearOwner    state.PlayerID
		want0, want1 int
	}{
		{"own creature", 0, unlimitedHandSize, maxHandSize},
		{"opponent's creature", 1, maxHandSize, unlimitedHandSize},
	} {
		e := combatEngine(t)
		nerdID := onBoardCard(t, e, 0, nerd)
		bear := onBoardReady(t, e, tc.bearOwner, bearSrc)
		if got := e.maxHandSizeFor(0); got != maxHandSize {
			t.Fatalf("%s: precondition: unattached max hand size = %d, want %d", tc.name, got, maxHandSize)
		}
		e.emit(events.Event{Kind: events.Attach, Obj: nerdID, IDs: []state.ObjID{bear}})
		if o := e.G.Obj(bear); o.Controller != tc.bearOwner {
			t.Fatalf("%s: precondition: bear controller %d, want %d", tc.name, o.Controller, tc.bearOwner)
		}
		if got := e.maxHandSizeFor(0); got != tc.want0 {
			t.Errorf("%s: seat 0 max hand size = %d, want %d", tc.name, got, tc.want0)
		}
		if got := e.maxHandSizeFor(1); got != tc.want1 {
			t.Errorf("%s: seat 1 max hand size = %d, want %d", tc.name, got, tc.want1)
		}
	}
}
