package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestArcaneDenialFlowDrawsForCounteredController plays Arcane Denial through
// the real cast: seat 1 counters seat 0's spell, and at the next upkeep (seat
// 1's) seat 0 -- the countered spell's controller, not Denial's -- is asked
// "up to two" and seat 1 draws its one card. A Counter whose RememberTargets$
// never reaches the delayed trigger leaves seat 0 unasked.
func TestArcaneDenialFlowDrawsForCounteredController(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	denial := searchCorpusCard(t, reg, "Arcane Denial")
	bears := searchCorpusCard(t, reg, "Grizzly Bears")
	deck0 := append([]*cards.Card{bears}, mountainDeck(t, 39)...)
	deck1 := append([]*cards.Card{denial}, mountainDeck(t, 39)...)
	cfg := seatZeroStart(Config{Seed: 4211, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{deck0, deck1}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)

	bearsID := searchMoveByNameSeat(t, e, 0, "Grizzly Bears", state.ZHand)
	denialID := searchMoveByNameSeat(t, e, 1, "Arcane Denial", state.ZHand)
	addMana(t, e, 0, "1G")
	addMana(t, e, 1, "1U")
	castNoDrain(t, e, bearsID)
	if len(e.G.Stack) != 1 {
		t.Fatalf("stack = %d, want Grizzly Bears alone", len(e.G.Stack))
	}
	passOnce(t, e) // seat 0 passes; seat 1 now holds priority
	d := e.Pending()
	if d == nil || d.Player != 1 {
		t.Fatalf("seat 1 should hold priority: %+v", d)
	}
	castNoDrain(t, e, denialID)
	for i := 0; i < 6 && len(e.G.Stack) > 0; i++ {
		passOnce(t, e)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack not empty: %d", len(e.G.Stack))
	}
	if o := e.G.Obj(bearsID); o.Zone != state.ZGraveyard {
		t.Fatalf("Grizzly Bears in zone %v, want graveyard (countered)", o.Zone)
	}

	// Next turn is seat 1's: its upkeep fires the slowtrip draw for seat 1
	// and the up-to-two ask for seat 0.
	driveToUpkeepAsk(t, e)
	before0, before1 := countDrawFor(e, 0), countDrawFor(e, 1)
	d = drainTriggersThenAsk(t, e, 30)
	if d.Kind != decision.KChoose || d.Player != 0 || d.Min != 0 || d.Max != 2 {
		t.Fatalf("upkeep ask = %+v, want KChoose Min 0 Max 2 for seat 0 (the countered spell's controller)", d)
	}
	submitChoices(t, e, d.Options[0].Index, d.Options[1].Index)
	passUntilStackEmpty(t, e, 30)
	if got := countDrawFor(e, 0) - before0; got != 2 {
		t.Fatalf("seat 0 drew %d at the upkeep, want 2", got)
	}
	if got := countDrawFor(e, 1) - before1; got != 1 {
		t.Fatalf("seat 1 drew %d at the upkeep, want 1", got)
	}
	replayCheck(t, e, cfg)
}

// driveToUpkeepAsk passes priorities until the turn-2 upkeep of seat 1.
func driveToUpkeepAsk(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 400; i++ {
		if e.G.Turn >= 2 && e.G.Active == 1 && e.G.Step == state.StepUpkeep {
			return
		}
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch d.Kind {
		case decision.KPriority:
			passOnce(t, e)
		case decision.KAttackers, decision.KBlockers:
			submitChoices(t, e)
		case decision.KChoose:
			n := d.Max
			if n > len(d.Options) {
				n = len(d.Options)
			}
			ch := make([]int, 0, n)
			for j := 0; j < n; j++ {
				ch = append(ch, d.Options[j].Index)
			}
			submitChoices(t, e, ch...)
		default:
			t.Fatalf("unexpected %v while driving: %+v", d.Kind, d)
		}
	}
	t.Fatal("did not reach seat 1's upkeep")
}
