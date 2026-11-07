package searchprobe

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCommanderRosterUsesObservationRefs pins the collector's remap of the
// public commander roster: a roster CardView must name the commander's
// observation ref -- the same object its zone CardView names -- never the raw
// engine ObjID. The roster is part of the canonical board encoding, so a raw
// id there would make a redealt world's board sum depend on arena allocation.
func TestCommanderRosterUsesObservationRefs(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	da, err := testutil.LoadRepoDeck(reg, "foundations-tramplesaurus-rex")
	if err != nil {
		t.Fatal(err)
	}
	db, err := testutil.LoadRepoDeck(reg, "foundations-wretched-ranks")
	if err != nil {
		t.Fatal(err)
	}
	fa, err := testutil.LoadRepoDeckFile("foundations-tramplesaurus-rex")
	if err != nil {
		t.Fatal(err)
	}
	fb, err := testutil.LoadRepoDeckFile("foundations-wretched-ranks")
	if err != nil {
		t.Fatal(err)
	}
	e := rules.New(rules.Config{
		Seed:         9001,
		Names:        []string{"foundations-tramplesaurus-rex", "foundations-wretched-ranks"},
		Decks:        [][]*cards.Card{da, db},
		Commanders:   [][]int{fa.CommanderIndices(), fb.CommanderIndices()},
		Tokens:       reg.Tokens,
		Format:       rules.FormatCommander,
		StartingLife: 40,
	})
	e.Advance()
	c := NewCollector(0)
	var a frameArena
	v, _, err := c.observe(e, nil, false, &a)
	if err != nil {
		t.Fatal(err)
	}
	for pl := range v.Players {
		live := e.G.Players[pl]
		if len(live.Commanders) == 0 {
			t.Fatalf("fixture: player %d has no commanders", pl)
		}
		for k, id := range live.Commanders {
			roster := v.Players[pl].Commanders[k]
			if roster.ID == 0 {
				t.Fatalf("player %d commander %d: roster carries no observation ref", pl, id)
			}
			if got := c.object(uint32(roster.ID)); got != id {
				t.Fatalf("player %d commander %d: roster ref %d maps to object %d", pl, id, roster.ID, got)
			}
			if o := e.G.Obj(id); o == nil || o.Zone != state.ZCommand {
				continue
			}
			var zone state.ObjID
			for _, cv := range v.Players[pl].Command {
				if cv.Name == roster.Name {
					zone = cv.ID
					break
				}
			}
			if zone == 0 {
				t.Fatalf("player %d commander %q is not in the projected command zone", pl, roster.Name)
			}
			if roster.ID != zone {
				t.Fatalf("player %d commander %q: roster id %d, command zone id %d", pl, roster.Name, roster.ID, zone)
			}
		}
	}
}
