package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// This file pins the opponent ask of a `Chooser$ Opponent` hidden pick
// (Tasigur, the Golden Fang's "a nonland card of an opponent's choice from
// your graveyard"; CR 608.2d, the CR 801.5b example): with two or more live
// opponents the controller chooses which one answers, and the pick is posed
// to that seat. With one live opponent nothing is asked.

// hiddenOppTapeHost answers the opponent ask with option oppIdx and the pick
// with its first option, recording each served ask in order.
type hiddenOppTapeHost struct {
	*fakeHost
	oppIdx int
	served []*decision.Decision
}

func (h *hiddenOppTapeHost) AskCount() uint64 { return uint64(h.askCount) }

func (h *hiddenOppTapeHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	if len(d.Options) == 0 {
		return decision.Intent{}, false
	}
	choices := []int{0}
	if d.Options[0].Kind == "player" {
		choices = []int{h.oppIdx}
	}
	h.served = append(h.served, d)
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}, true
}

// hiddenOppResolve resolves Tasigur's return over seat 0's three-card
// graveyard at the given seat count.
func hiddenOppResolve(t *testing.T, seats, oppIdx int) (*hiddenOppTapeHost, []state.ObjID) {
	t.Helper()
	h := &hiddenOppTapeHost{fakeHost: newHost(t, seats), oppIdx: oppIdx}
	var ids []state.ObjID
	for _, n := range []string{"Alpha", "Beta", "Gamma", "Source"} {
		c := mkCard(t, "Name:"+n+"\nTypes:Creature\nPT:1/1\nOracle:x\n")
		ids = append(ids, h.g.AddObject(c, 0).ID)
	}
	gy := append([]state.ObjID(nil), ids[:3]...)
	h.g.SetZone(state.ZGraveyard, 0, gy)
	for _, id := range gy {
		h.g.Obj(id).Zone = state.ZGraveyard
	}
	if len(h.g.Players) != seats {
		t.Fatalf("precondition: %d players, want %d", len(h.g.Players), seats)
	}
	Resolve(h, NewCtxPtr(ids[3], 0, CtxInit{}), sa(t, "DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | DefinedPlayer$ You | Chooser$ Opponent | Hidden$ True | Mandatory$ True | ChangeType$ Card.nonLand+YouOwn"))
	return h, ids
}

func TestHiddenPickChooserOpponentAskedAtThreeSeats(t *testing.T) {
	h, ids := hiddenOppResolve(t, 3, 1)
	if len(h.served) != 2 {
		t.Fatalf("served %d asks, want the opponent ask and the pick", len(h.served))
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
	if pick := h.served[1]; pick.Player != 2 {
		t.Fatalf("pick asked of seat %d, want the chosen opponent 2", pick.Player)
	}
	if z := h.g.Obj(ids[0]).Zone; z != state.ZHand {
		t.Fatalf("picked card zone = %d, want hand", z)
	}
}

func TestHiddenPickChooserOpponentNotAskedAtTwoSeats(t *testing.T) {
	h, _ := hiddenOppResolve(t, 2, 0)
	if len(h.served) != 1 || h.served[0].Options[0].Kind == "player" {
		t.Fatalf("served %d asks, want only the pick", len(h.served))
	}
	if p := h.served[0].Player; p != 1 {
		t.Fatalf("pick asked of seat %d, want the one opponent 1", p)
	}
}
