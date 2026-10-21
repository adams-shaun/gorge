package rules

// CR 601.2b: "If the spell is modal, the player announces the mode choice
// ... If the spell has a variable cost ... the player announces the value of
// that variable." The modes come first, then the additional-cost
// announcements, then X; every cost choice comes later still (601.2h).
// Real corpus cards read as script text; the board is built eventlessly and
// funded through the pool.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR601_2bModesAreAnnouncedBeforeX: Kozilek's Command ({X}{C}{C},
// "choose two") with five {C} floating and the opponent's Grizzly Bears (mana
// value 2) on the battlefield. The first decision of the cast is the mode
// announcement, not X; "exile target creature with mana value X or less" is
// offered although X is not announced yet (it becomes legal for X >= 2); X is
// asked after the modes, then the targets.
func TestCR601_2bModesAreAnnouncedBeforeX(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, corpusCardText(t, "k/kozileks_command.txt")))
	cmd := e.G.Zone(state.ZHand, 0)[0]
	bear := battlePerm(t, e, 1, corpusCardText(t, "g/grizzly_bears.txt"))
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "C", Amount: 5})
	e.askPriority(0)
	cr601Choose(t, e, "cast Kozilek's Command", func(o decision.Option) bool { return o.Kind == "cast" && o.Obj == cmd })
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("CR 601.2b: the first cast decision is %+v, want the mode announcement (before X)", d)
	}
	exile, token := -1, -1
	for _, o := range d.Options {
		switch {
		case o.Label == "Exile target creature with mana value X or less.":
			exile = o.Index
		case o.Label == "Target player creates X 0/1 colorless Eldrazi Spawn creature tokens with \"Sacrifice this creature: Add {C}.\"":
			token = o.Index
		}
	}
	if exile < 0 || token < 0 {
		t.Fatalf("CR 601.2b: the X-dependent modes must be offered before X is announced: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{token, exile}}); err != nil {
		t.Fatal(err)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("after the modes, want the X announcement, got %+v", d)
	}
	cr601Choose(t, e, "X = 3", func(o decision.Option) bool { return o.Kind == "x" && o.Amount == 3 })
	for i := 0; i < 4; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		cr601Choose(t, e, "targets", func(o decision.Option) bool {
			return o.Obj == bear || (o.Kind == "player" && o.Label == "a")
		})
	}
	if z := e.G.Obj(cmd).Zone; z != state.ZStack {
		t.Fatalf("Kozilek's Command is in %s after casting, want the stack", z)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Errorf("pool %d after paying {3}{C}{C}, want 0", got)
	}
}
