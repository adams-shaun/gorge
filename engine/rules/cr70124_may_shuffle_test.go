package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g09 (ticket b): Put Away -- "Counter target spell. You may shuffle up
// to one target card from your graveyard into your library." The "may"
// covers the whole shuffle-in: declining leaves the card in the graveyard
// and the library untouched. (gorge used to move the card first and ask
// afterwards, so a decline left the known card on TOP of the library.) The
// real corpus card is loaded through the gitignored registry.
func TestCR70124PutAwayDeclinedMayShuffleLeavesTheCard(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Put Away")
	searchMoveByName(t, e, "Put Away", state.ZHand)
	searchMoveByName(t, e, "Grizzly Bears", state.ZHand)
	forest := searchMoveByName(t, e, "Forest", state.ZGraveyard)
	if o := e.G.Obj(forest); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: the graveyard card is in %v, want it in the graveyard", cr70124Zone(o))
	}
	addMana(t, e, 0, "CG")
	castNamedInHand(t, e, "Grizzly Bears")
	var bearStack state.ObjID
	for _, id := range e.G.Zone(state.ZStack, 0) {
		bearStack = id
	}
	if bearStack == 0 {
		t.Fatal("precondition: no spell on the stack after casting the bear")
	}
	addMana(t, e, 0, "CCUU")
	if d := castNamedInHand(t, e, "Put Away"); d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after casting Put Away: pending = %+v, want the counter target ask", d)
	}
	chooseObj(t, e, "spell", bearStack)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind != "cast_sub" {
		t.Fatalf("graveyard ask = %+v, want the sub's cast-time KTarget ask", d)
	}
	graveIdx := -1
	for _, o := range d.Options {
		if o.Obj == forest {
			graveIdx = o.Index
		}
	}
	if graveIdx < 0 {
		t.Fatalf("the graveyard ask does not offer %d: %+v", forest, d.Options)
	}
	submitChoices(t, e, graveIdx)
	libBefore := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...)
	start := len(e.L.Events)
	passUntilAskKind(t, e, decision.KChoose, 30)
	// The election comes BEFORE the move: the card is still in the graveyard
	// while it is posed.
	if o := e.G.Obj(forest); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("while the may-shuffle election is posed the card is in %v, want it still in the graveyard", cr70124Zone(o))
	}
	_, no := mayShuffleConfirm(t, e, 0)
	submitChoices(t, e, no)
	drainAfterAnswer(t, e)
	if o := e.G.Obj(forest); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("after a declined \"you may shuffle\" the card is in %v, want it still in the graveyard", cr70124Zone(o))
	}
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.MoveZone && ev.Obj == forest {
			t.Fatalf("a declined \"you may shuffle\" moved the card: %+v", ev)
		}
	}
	if got := shuffleCountFrom(e, start, 0); got != 0 {
		t.Fatalf("seat-0 shuffles after a decline = %d, want 0", got)
	}
	lib := e.G.Zone(state.ZLibrary, 0)
	if len(lib) != len(libBefore) {
		t.Fatalf("library size %d after a decline, want %d unchanged", len(lib), len(libBefore))
	}
	replayCheck(t, e, cfg)
}

// cr70124Zone names an object's zone for a failure message ("gone" if nil).
func cr70124Zone(o *state.Object) string {
	if o == nil {
		return "gone"
	}
	return o.Zone.String()
}
