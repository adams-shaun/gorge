package rules

// CR 701.31a: "clash with an opponent" -- the clashing player chooses which.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestClashChoosesOpponent(t *testing.T) {
	t.Parallel()
	e := New(seatZeroStart(Config{Seed: 1, Names: []string{"a", "b", "c"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}}))
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	src := onBoardCard(t, e, 0, card(t, "Name:Clash Opp Fixture\nManaCost:1 G\nTypes:Creature Beast\nPT:1/1\nSVar:Go:DB$ Clash\nOracle:x\n"))
	sa := resolveSourceFaceSA(t, e, src, "Go")
	if sa == nil || sa.API != "Clash" {
		t.Fatalf("fixture SA = %+v, want a bare Clash", sa)
	}
	mine := putTopOfLibrary(t, e, card(t, "Name:Mine\nManaCost:3\nTypes:Creature Beast\nPT:3/3\nOracle:x\n"), 0)
	// Seat 1 would beat seat 0, seat 2 loses to it: which opponent was chosen
	// is observable in the Clash markers and the winner.
	strong := putTopOfLibrary(t, e, card(t, "Name:Strong\nManaCost:9\nTypes:Creature Beast\nPT:9/9\nOracle:x\n"), 1)
	weak := putTopOfLibrary(t, e, card(t, "Name:Weak\nManaCost:0\nTypes:Creature Beast\nPT:1/1\nOracle:x\n"), 2)
	if e.G.Zone(state.ZLibrary, 1)[0] == e.G.Zone(state.ZLibrary, 2)[0] {
		t.Fatal("precondition: the opponents' tops are the same card")
	}
	resolveUnderProbe(e, func() *effects.Ctx { return &effects.Ctx{Source: src, Controller: 0} }, sa)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "clash_opponent" || d.Player != 0 {
		t.Fatalf("pending = %+v, want seat 0's clash_opponent choice", d)
	}
	if len(d.Options) != 2 || d.Options[0].Player != 1 || d.Options[1].Player != 2 {
		t.Fatalf("options = %+v, want opponents 1 and 2", d.Options)
	}
	submitChoices(t, e, d.Options[1].Index)
	answerClashPlacement(t, e, 0, "top")
	answerClashPlacement(t, e, 2, "top")
	var revealed []state.ObjID
	markers := map[state.PlayerID]int32{}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && len(ev.IDs) == 1 {
			revealed = append(revealed, ev.IDs[0])
		}
		if ev.Kind == events.Clash {
			markers[ev.Player] = ev.Amount + 10
		}
	}
	if len(revealed) != 2 || revealed[0] != mine || revealed[1] != weak {
		t.Fatalf("revealed %v, want seat 0's %d and the chosen seat 2's %d (not seat 1's %d)", revealed, mine, weak, strong)
	}
	if len(markers) != 2 || markers[0] != 11 || markers[2] != 10 {
		t.Fatalf("clash markers %v, want seat 0 won and seat 2 lost", markers)
	}
}
