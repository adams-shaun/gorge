package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file pins the opponent ask of a `Chooser$ Opponent` library search
// (CR 608.2d, the CR 801.5b example): with two or more live opponents the
// controller chooses which one answers, once per searched library, and that
// seat answers every prompt of the search. With one live opponent nothing is
// asked.

// searchOppTapeHost answers the asks such a search poses through the ask
// seam, recording each served ask in order.
type searchOppTapeHost struct {
	*fakeHost
	oppIdx int // the opponent-ask option index answered
	served []*decision.Decision
}

func (h *searchOppTapeHost) AskCount() uint64 { return uint64(h.askCount) }

func (h *searchOppTapeHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	if len(d.Options) == 0 {
		return decision.Intent{}, false
	}
	var choices []int
	switch d.Options[0].Kind {
	case "player":
		choices = []int{h.oppIdx}
	case "yes":
		choices = []int{0}
	case "search":
		choices = []int{1, 2}
	default:
		return decision.Intent{}, false
	}
	h.served = append(h.served, d)
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}, true
}

// searchOppResolve resolves the search over seat 0's five-card library at
// the given seat count: an Optional$ confirmation and a MaxRevealed$ look
// window before the pick, so three of the search's prompts follow the answer.
func searchOppResolve(t *testing.T, seats, oppIdx int) (*searchOppTapeHost, []state.ObjID) {
	t.Helper()
	h := &searchOppTapeHost{fakeHost: newHost(t, seats), oppIdx: oppIdx}
	var ids []state.ObjID
	for _, n := range []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon", "Source"} {
		c := mkCard(t, "Name:"+n+"\nTypes:Creature\nPT:1/1\nOracle:x\n")
		ids = append(ids, h.g.AddObject(c, 0).ID)
	}
	lib := append([]state.ObjID(nil), ids[:5]...)
	h.g.SetZone(state.ZLibrary, 0, lib)
	for _, id := range lib {
		h.g.Obj(id).Zone = state.ZLibrary
	}
	if len(h.g.Players) != seats {
		t.Fatalf("precondition: %d players, want %d", len(h.g.Players), seats)
	}
	Resolve(h, NewCtxPtr(ids[5], 0, CtxInit{}), sa(t, "DB$ ChangeZone | Origin$ Library | Destination$ Graveyard | ChangeType$ Card | ChangeNum$ 2 | Chooser$ Opponent | Mandatory$ True | Optional$ True | MaxRevealed$ 3 | Shuffle$ False"))
	return h, ids
}

func TestSearchChooserOpponentAskedAtThreeSeats(t *testing.T) {
	h, ids := searchOppResolve(t, 3, 1)
	var kinds []string
	for _, d := range h.served {
		kinds = append(kinds, d.ResumeKind)
	}
	if len(h.served) != 3 || kinds[0] != "search_opponent" || kinds[1] != "search_confirm" || kinds[2] != "search" {
		t.Fatalf("asks = %v, want [search_opponent search_confirm search]", kinds)
	}
	ask := h.served[0]
	if ask.Player != 0 || ask.Min != 1 || ask.Max != 1 || len(ask.Options) != 2 {
		t.Fatalf("opponent ask = player %d, %d..%d, %d options, want the controller 0 picking 1 of 2", ask.Player, ask.Min, ask.Max, len(ask.Options))
	}
	for i, want := range []state.PlayerID{1, 2} {
		if o := ask.Options[i]; o.Kind != "player" || o.Player != want {
			t.Fatalf("opponent ask option %d = %q player %d, want \"player\" %d", i, o.Kind, o.Player, want)
		}
	}
	for _, d := range h.served[1:] {
		if d.Player != 2 {
			t.Errorf("%s asked of seat %d, want the chosen opponent 2", d.ResumeKind, d.Player)
		}
	}
	looks, searched := 0, 0
	for _, e := range h.log {
		switch {
		case e.Kind == events.Note && e.Secret:
			looks++
			if e.Player != 2 {
				t.Errorf("the look window is shown to seat %d, want the chosen opponent 2", e.Player)
			}
		case e.Kind == events.SearchedLibrary:
			searched++
			if e.Amount != 3 {
				t.Errorf("SearchedLibrary searcher = %d, want seat 2 + 1", e.Amount)
			}
		}
	}
	if looks != 1 || searched != 1 {
		t.Fatalf("looks %d, searched records %d, want 1 and 1", looks, searched)
	}
	for _, id := range ids[1:3] {
		if z := h.g.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("picked card %d zone = %d, want graveyard", id, z)
		}
	}
}

func TestSearchChooserOpponentNotAskedAtTwoSeats(t *testing.T) {
	h, _ := searchOppResolve(t, 2, 0)
	if len(h.served) != 2 || h.served[0].ResumeKind != "search_confirm" || h.served[1].ResumeKind != "search" {
		t.Fatalf("served %d asks, want only the confirmation and the pick", len(h.served))
	}
	for _, d := range h.served {
		if d.Player != 1 {
			t.Errorf("%s asked of seat %d, want the one opponent 1", d.ResumeKind, d.Player)
		}
	}
}
