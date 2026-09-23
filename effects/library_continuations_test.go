package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// multiLibraryContinuationHost gives each of three players two distinct top
// cards. The distinct order is a precondition: a test that accidentally asks
// or resolves only one library must not pass by comparing identical values.
func multiLibraryContinuationHost(t *testing.T) (*askHost, [][]state.ObjID) {
	t.Helper()
	h := &askHost{}
	h.g = state.NewGame(names(3))
	cards := []*cards.Card{
		mkCard(t, "Name:Alpha\nTypes:Creature\nPT:2/2\nOracle:x\n"),
		mkCard(t, "Name:Beta\nTypes:Creature\nPT:3/3\nOracle:x\n"),
	}
	libs := make([][]state.ObjID, 3)
	for p := state.PlayerID(0); p < 3; p++ {
		for _, card := range cards {
			libs[p] = append(libs[p], h.g.AddObject(card, p).ID)
		}
		h.g.SetZone(state.ZLibrary, p, libs[p])
		if libs[p][0] == libs[p][1] {
			t.Fatal("precondition: library cards must have distinct object ids")
		}
	}
	return h, libs
}

func requireSecondLibraryAsk(t *testing.T, h *askHost, want decision.Kind) {
	t.Helper()
	if h.asked == nil || h.asked.Kind != want || h.asked.ResumeTarget != 1 {
		t.Fatalf("decision = %+v, want %v for library target 1", h.asked, want)
	}
	if len(h.asked.Options) == 0 || h.asked.Options[0].Obj == 0 {
		t.Fatal("precondition: the second library ask must offer a real card")
	}
}

func TestSearchContinuesWithTheNextLibraryAfterAnAnswer(t *testing.T) {
	h, libs := multiLibraryContinuationHost(t)
	effect := sa(t, "DB$ ChangeZone | Defined$ Player | Origin$ Library | Destination$ Hand | ChangeType$ Creature")
	ctx := &Ctx{Controller: 0}
	Resolve(h, ctx, effect)
	if h.asked == nil || h.asked.ResumeKind != "search" || h.asked.ResumeTarget != 0 {
		t.Fatalf("first decision = %+v, want the first library search", h.asked)
	}
	if h.asked.Options[0].Obj != libs[0][0] {
		t.Fatalf("first options = %+v, want seat 0's first card %d", h.asked.Options, libs[0][0])
	}
	picked := h.asked.Options[0].Obj
	h.asked = nil
	Resolve(h, &Ctx{Controller: 0, Search: []state.ObjID{picked}, SearchDone: true, LibraryTarget: 0}, effect)
	requireSecondLibraryAsk(t, h, decision.KChoose)
	if h.asked.Options[0].Obj != libs[1][0] {
		t.Fatalf("second options = %+v, want seat 1's first card %d", h.asked.Options, libs[1][0])
	}
}

func TestRearrangeContinuesWithTheNextLibraryAfterAnAnswer(t *testing.T) {
	h, libs := multiLibraryContinuationHost(t)
	effect := sa(t, "SP$ RearrangeTopOfLibrary | Defined$ Player | NumCards$ 1")
	Resolve(h, &Ctx{Controller: 0}, effect)
	if h.asked == nil || h.asked.Kind != decision.KArrange || h.asked.ResumeTarget != 0 {
		t.Fatalf("first decision = %+v, want the first arrange", h.asked)
	}
	if h.asked.Options[0].Obj != libs[0][0] {
		t.Fatalf("first arrange = %+v, want seat 0's first card %d", h.asked.Options, libs[0][0])
	}
	h.asked = nil
	Resolve(h, &Ctx{Controller: 0, Arrange: true, LibraryTarget: 0}, effect)
	requireSecondLibraryAsk(t, h, decision.KArrange)
	if h.asked.Options[0].Obj != libs[1][0] {
		t.Fatalf("second arrange = %+v, want seat 1's first card %d", h.asked.Options, libs[1][0])
	}
}

func TestScryContinuesWithTheNextLibraryAfterAnAnswer(t *testing.T) {
	h, libs := multiLibraryContinuationHost(t)
	effect := sa(t, "SP$ Scry | Defined$ Player | ScryNum$ 1")
	Resolve(h, &Ctx{Controller: 0}, effect)
	if h.asked == nil || h.asked.Kind != decision.KArrange || h.asked.ResumeTarget != 0 {
		t.Fatalf("first decision = %+v, want the first scry arrange", h.asked)
	}
	if h.asked.Options[0].Obj != libs[0][0] {
		t.Fatalf("first scry = %+v, want seat 0's first card %d", h.asked.Options, libs[0][0])
	}
	h.asked = nil
	Resolve(h, &Ctx{Controller: 0, Arrange: true, LibraryTarget: 0}, effect)
	requireSecondLibraryAsk(t, h, decision.KArrange)
	if h.asked.Options[0].Obj != libs[1][0] {
		t.Fatalf("second scry = %+v, want seat 1's first card %d", h.asked.Options, libs[1][0])
	}
}
