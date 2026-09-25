package rules

import (
	"maps"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func mayPlayAfterStackCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	c, ok := testutil.CorpusRegistry(t).Lookup(name)
	if !ok {
		t.Fatalf("%s missing from corpus", name)
	}
	if d := c.Link(); len(d) != 0 {
		t.Fatalf("link %s: %v", name, d)
	}
	return c
}

func afterStackOffer(d *decision.Decision, kind string, id state.ObjID) bool {
	if d == nil {
		return false
	}
	for _, opt := range d.Options {
		if opt.Kind == kind && opt.Obj == id {
			return true
		}
	}
	return false
}

func TestMayPlayValidAfterStackHaakonKnight(t *testing.T) {
	e, _, _ := newFixtureDeck(t, 8101, "Name:Blank\nTypes:Sorcery\nOracle:x\n")
	h := e.G.AddObject(mayPlayAfterStackCard(t, "Haakon, Stromgald Scourge"), 0)
	h.Zone = state.ZBattlefield
	k := e.G.AddObject(mayPlayAfterStackCard(t, "Knight of the White Orchid"), 0)
	k.Zone = state.ZGraveyard
	b := e.G.AddObject(mayPlayAfterStackCard(t, "Grizzly Bears"), 0)
	b.Zone = state.ZGraveyard
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{h.ID})
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{k.ID, b.ID})
	if h.Zone != state.ZBattlefield || k.Zone != state.ZGraveyard || b.Zone != state.ZGraveyard ||
		!slices.Contains(k.Face().Types, "Knight") || slices.Contains(b.Face().Types, "Knight") {
		t.Fatal("Haakon/Knight/non-Knight fixture precondition failed")
	}
	e.G.Players[0].Pool[state.MW] = 10
	e.G.Step, e.G.Active, e.G.Priority, e.G.Turn = state.StepMain1, 0, 0, 1
	e.pending = nil
	e.Advance()
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || !afterStackOffer(d, "cast", k.ID) || afterStackOffer(d, "cast", b.ID) {
		t.Fatalf("Haakon must offer Knight only: %+v", d)
	}
	// Broaden only Affected$: the non-Knight must still be excluded by
	// ValidAfterStack, not incidentally by the printed Affected$ filter.
	for _, st := range h.Face().Statics {
		if st.Params["ValidAfterStack"] != "Spell.Knight" {
			continue
		}
		params := maps.Clone(st.Params)
		params["Affected"] = "Card.YouOwn"
		if applies, grants, _, _, _, _ := e.mayPlayStatic(params, b.ID, 0, h.ID); applies || grants {
			t.Fatalf("non-Knight passed isolated ValidAfterStack gate: %v %v", applies, grants)
		}
		return
	}
	t.Fatal("Haakon carrier static missing")
}

func TestMayPlayValidAfterStackSerraManaValueGate(t *testing.T) {
	cheap := mayPlayAfterStackCard(t, "Grizzly Bears")
	expensive := mayPlayAfterStackCard(t, "Serra Angel")
	if cheap.Faces[0].ManaValue() > 3 || expensive.Faces[0].ManaValue() <= 3 ||
		!cheap.Faces[0].IsPermanent() || !expensive.Faces[0].IsPermanent() {
		t.Fatal("Serra fixture must contain permanent cards on opposite sides of cmcLE3")
	}
	e, _, _ := newFixtureDeck(t, 8102, "Name:Blank\nTypes:Sorcery\nOracle:x\n")
	s := e.G.AddObject(mayPlayAfterStackCard(t, "Serra Paragon"), 0)
	s.Zone = state.ZBattlefield
	c := e.G.AddObject(cheap, 0)
	c.Zone = state.ZGraveyard
	x := e.G.AddObject(expensive, 0)
	x.Zone = state.ZGraveyard
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{s.ID})
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{c.ID, x.ID})
	if s.Zone != state.ZBattlefield || c.Zone != state.ZGraveyard || x.Zone != state.ZGraveyard {
		t.Fatal("Serra Paragon/graveyard precondition not established")
	}
	e.G.Players[0].Pool[state.MG] = 10
	e.G.Step, e.G.Active, e.G.Priority, e.G.Turn = state.StepMain1, 0, 0, 1
	e.pending = nil
	e.Advance()
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority || !afterStackOffer(d, "cast", c.ID) || afterStackOffer(d, "cast", x.ID) {
		t.Fatalf("Serra must offer only cheap permanent: %+v", d)
	}
	found := false
	for _, st := range s.Face().Statics {
		if st.Params["ValidAfterStack"] != "Spell.cmcLE3" {
			continue
		}
		found = true
		params := maps.Clone(st.Params)
		params["Affected"] = "Card.YouOwn"
		if applies, grants, _, _, _, _ := e.mayPlayStatic(params, x.ID, 0, s.ID); applies || grants {
			t.Fatalf("high-MV card passed isolated ValidAfterStack gate: %v %v", applies, grants)
		}
	}
	if !found {
		t.Fatal("Serra carrier static missing")
	}
	// A prior cast of this same card this turn consumes its MayPlayLimit$ 1.
	// The return to the graveyard is an event too; it does not reset the log.
	e.emit(events.Event{Kind: events.PutOnStack, Obj: c.ID, Player: 0, From: state.ZGraveyard, To: state.ZStack})
	e.emit(events.Event{Kind: events.MoveZone, Obj: c.ID, Player: 0, From: state.ZStack, To: state.ZGraveyard})
	if c.Zone != state.ZGraveyard || !e.mayPlayLimitReached(c.ID, 1) {
		t.Fatal("precondition: cast not recorded or card not returned to graveyard")
	}
	e.pending = nil
	e.Advance()
	if d := e.Pending(); d == nil || afterStackOffer(d, "cast", c.ID) {
		t.Fatalf("Serra's once-per-turn grant offered again: %+v", d)
	}
}
