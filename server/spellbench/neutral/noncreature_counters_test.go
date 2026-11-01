package neutral_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testgame"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
)

// A +1/+1 counter on a noncreature moves no P/T (CR 208.3), and on a creature it
// does: the native view and the seat rebuilding it from an observation read one
// number for every card of a deck that holds both kinds.
func TestCountersOnANoncreatureMoveNoPT(t *testing.T) {
	t.Parallel()
	reg := testcorpus.Registry(t)
	d := catalog.Decks()[0]
	g := testgame.New(t, reg, d.CatalogID, d.CatalogID, 3, "none")
	seen := map[string]bool{}
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range g.E.G.Zone(z, 0) {
			o := g.E.G.Obj(id)
			if !seen[o.Face().Name] {
				seen[o.Face().Name] = true
				events.Emit(g.E.G, g.E.L, events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield})
				events.Emit(g.E.G, g.E.L, events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 2})
			}
		}
	}
	tr := identity.New(g.R, g.Secret)
	if err := tr.Sync(g.R); err != nil {
		t.Fatal(err)
	}
	holder := state.PlayerID(0)
	obs, err := (&observe.Projector{R: g.R, IDs: tr}).Observation(0, observe.State{PriorityHolder: &holder})
	if err != nil {
		t.Fatal(err)
	}
	got, err := neutral.NewBuilder(reg).View(&obs)
	if err != nil {
		t.Fatal(err)
	}
	want := view.Project(g.E.G, g.E, 0, nil).Players[0].Battlefield
	rebuilt := got.Players[0].Battlefield
	if len(want) != len(rebuilt) || len(want) < 2 {
		t.Fatalf("battlefields of %d and %d cards", len(want), len(rebuilt))
	}
	for i, w := range want {
		if h := rebuilt[i]; h.Power != w.Power || h.Toughness != w.Toughness {
			t.Errorf("%s (%s): native %d/%d, rebuilt %d/%d", w.Name, w.Types, w.Power, w.Toughness, h.Power, h.Toughness)
		}
	}
}
