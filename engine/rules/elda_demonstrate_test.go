package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEldaFirstNonlegendaryCreatureSpellDemonstrates pins gorge's reading of
// Elda, Conjurer of Spectacle (a CheckSVar$ X EQ0 gated AffectedZone$ Stack
// Demonstrate grant): the first nonlegendary creature spell cast each turn has
// demonstrate (the election is posed), the second does not.
func TestEldaFirstNonlegendaryCreatureSpellDemonstrates(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	elda := searchCorpusCard(t, reg, "Elda, Conjurer of Spectacle")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	e, _ := demonstrateEngine(t, reg, []*cards.Card{elda, bear, bear}, nil)

	mobVerdictVoteMove(t, e, 0, "Elda, Conjurer of Spectacle", state.ZBattlefield)
	var bears []state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range append([]state.ObjID(nil), e.G.Zone(z, 0)...) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" {
				if z != state.ZHand {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZHand})
				}
				bears = append(bears, id)
			}
		}
	}
	e.pending = nil
	e.priorityRound()
	if len(bears) < 2 {
		t.Fatalf("want two bears in hand, got %d", len(bears))
	}
	addMana(t, e, 0, "GGGG")
	castFixture(t, e, bears[0], -1)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "demonstrate" {
		t.Fatalf("first creature spell: pending = %+v, want a demonstrate election", d)
	}
	submitChoices(t, e, d.Options[1].Index) // no
	drainStack(t, e, 200)
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after the first spell resolved: pending = %+v, want priority", d)
	}
	cast := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == bears[1] {
			cast = o.Index
		}
	}
	if cast < 0 {
		t.Fatalf("no cast option for the second bear: %+v", d.Options)
	}
	submitChoices(t, e, cast)
	if o := e.G.Obj(bears[1]); o == nil || o.Zone != state.ZStack {
		t.Fatalf("second bear is not on the stack: %+v", o)
	}
	// the second creature spell is not the first: no election is posed
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("second creature spell: pending = %+v, want priority (no demonstrate election)", d)
	}
}
