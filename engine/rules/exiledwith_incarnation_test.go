package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestExiledWithForgetsAfterZoneChange(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{lookup(t, reg, "Collector's Cage"), lookup(t, reg, "Grizzly Bears")}, mountainDeck(t, 38)...),
			mountainDeck(t, 40),
		},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	c := findAndMoveToHand(t, e, 0, "Collector's Cage")
	b := findAndMoveToHand(t, e, 0, "Grizzly Bears")
	_ = b
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	castCage := func() {
		addMana(t, e, 0, "WW")
		for _, o := range e.Pending().Options {
			if o.Kind == "cast" && o.Obj == c {
				submitChoices(t, e, o.Index)
				return
			}
		}
		t.Fatal("no cast option")
	}
	drain := func() {
		for i := 0; i < 20; i++ {
			d := e.Pending()
			if d == nil {
				e.Advance()
				continue
			}
			if d.Kind == decision.KPriority {
				if len(e.G.Stack) == 0 {
					return
				}
				for _, o := range d.Options {
					if o.Kind == "pass" {
						submitChoices(t, e, o.Index)
						break
					}
				}
				continue
			}
			t.Logf("decision %s resume=%q min=%d max=%d nopts=%d", d.Kind, d.ResumeKind, d.Min, d.Max, len(d.Options))
			for _, o := range d.Options {
				t.Logf("   opt %d kind=%s obj=%d label=%q", o.Index, o.Kind, o.Obj, o.Label)
			}
			submitChoices(t, e, 0)
		}
	}
	castCage()
	drain()
	t.Logf("cage id %d", c)
	for _, id := range e.G.Zone(state.ZExile, 0) {
		o := e.G.Obj(id)
		t.Logf("exile %d %s exiledWith=%d facedown=%v", id, o.Face().Name, o.ExiledWith, o.FaceDown)
	}
	first := append([]state.ObjID(nil), e.G.Zone(state.ZExile, 0)...)
	if len(first) != 1 {
		t.Fatalf("first hideaway: want 1 exiled card, got %d", len(first))
	}
	// bounce and return
	e.emit(events.Event{Kind: events.MoveZone, Obj: c, From: state.ZBattlefield, To: state.ZHand})
	t.Logf("cage zone %v id %d", e.G.Obj(c).Zone, c)
	castCage()
	drain()
	// CR 400.7: the recast Cage is a new object; only the card it hid itself
	// is its hidden card.
	var linked []state.ObjID
	for _, id := range e.G.Zone(state.ZExile, 0) {
		if o, src := e.G.Obj(id), e.G.Obj(c); o.ExiledWith == c && o.ExiledWithInc == src.Incarnation+1 || slices.Contains(src.ExiledCards, id) {
			linked = append(linked, id)
		}
	}
	if len(linked) != 1 || linked[0] == first[0] {
		t.Fatalf("recast Cage linked to %v; want exactly the new card (old %v)", linked, first)
	}
}
