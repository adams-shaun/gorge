// Fix round (findings-sol4 MAJOR): a GainLife→Draw replacement body (Lich's
// "If you would gain life, draw that many cards instead") drives its draws
// through effects.DrawFor, and each draw may suspend on a Dredge ask (CR
// 702.55). The old `case "Draw"` loop did not check h.Suspended() between
// iterations: the first draw's ask was orphaned by the second draw's own ask
// (two DecisionAsk events for one life event), and answering the surviving
// ask produced one draw where the card said two.
//
// The committed probe is exactly the finding's repro, on real card scripts:
// Lich on the battlefield, a Golgari Thug (Dredge 4) in its controller's
// graveyard, one LifeChange{Amount: 2} emit. Post-fix the loop parks the
// remaining count on the ask's resume point, the answered dredge re-drives
// the rest, and a second dredger re-parks for its own sequential ask.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// lichThugEngine deals seat 0 a deck led by the named cards, moves every
// named card out of seat 0's zones (thugs to the graveyard, the Lich onto
// the battlefield), and returns the engine. The tests use NEFARIOUS Lich
// rather than Lich because its GainLife replacement is the same shape
// ("If you would gain life, draw that many cards instead") without Lich's
// entry life drain, which would park seat 0 at 0 life — and the engine does
// not implement the R:Event$ GameLoss CantHappen family that keeps Lich's
// controller alive there (recorded as an issue, not fixed here).
func lichThugEngine(t *testing.T, seed uint64, lead ...string) (*Engine, Config, []state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	deck := []*cards.Card{}
	for _, name := range lead {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s missing from corpus", name)
		}
		if d := c.Link(); len(d) != 0 {
			t.Fatalf("link %s: %v", name, d)
		}
		deck = append(deck, c)
	}
	deck = append(deck, mountainDeck(t, 40-len(lead))...)
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{deck, mountainDeck(t, 40)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	ids := make([]state.ObjID, 0, len(lead))
	for _, name := range lead {
		var tid state.ObjID
		for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
			for _, id := range e.G.Zone(z, 0) {
				if o := e.G.Obj(id); o.Face() != nil && o.Face().Name == name {
					tid = id
				}
			}
		}
		if tid == 0 {
			t.Fatalf("%s was not dealt", name)
		}
		to := state.ZGraveyard
		if name == "Nefarious Lich" {
			to = state.ZBattlefield
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: tid, From: e.G.Obj(tid).Zone, To: to})
		ids = append(ids, tid)
	}
	return e, cfg, ids
}

// countAsks counts the mid-resolution KModes (dredge) DecisionAsk events
// for seat 0 since `since` — not the turn-structure priority asks that the
// surrounding Submit tail legitimately grants once the body completes.
func countAsks(t *testing.T, e *Engine, since int) int {
	t.Helper()
	n := 0
	for _, ev := range e.L.Events[since:] {
		if ev.Kind == events.DecisionAsk && ev.Player == 0 && ev.Text == string(decision.KModes) {
			n++
		}
	}
	return n
}

// TestLifeReplacementDredgeAsksOnceAndDrawsAll is the finding's probe: a
// replaced 2-card gain must never have two asks outstanding at once. Each
// draw poses its OWN dredge ask (CR 702.55 is per draw), sequentially: the
// first is answered before the second is posed, and both draws are delivered
// on declines.
func TestLifeReplacementDredgeAsksOnceAndDrawsAll(t *testing.T) {
	e, cfg, _ := lichThugEngine(t, 244, "Golgari Thug", "Nefarious Lich")
	since := len(e.L.Events)
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 2})
	if n := countAsks(t, e, since); n != 1 {
		t.Fatalf("DecisionAsk events for one replaced 2-card gain = %d, want exactly 1 (a second ask before the first is answered orphans it)", n)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "dredge" {
		t.Fatalf("pending = %+v, want the dredge ask", d)
	}
	// Decline (the trailing "Draw card" option): the ordinary draw for THIS
	// ask, then the parked remainder re-driven — which asks again, since the
	// dredger is still in the graveyard.
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player,
		Choices: []int{len(d.Options) - 1}}); err != nil {
		t.Fatalf("submit first dredge decline: %v", err)
	}
	if n := countAsks(t, e, since); n != 2 {
		t.Fatalf("asks after the first decline = %d, want 2 (the second draw's own ask)", n)
	}
	d2 := e.Pending()
	if d2 == nil || d2.ResumeKind != "dredge" {
		t.Fatalf("pending after the first decline = %+v, want the second draw's ask", d2)
	}
	if err := e.Submit(decision.Intent{Seq: d2.Seq, Player: d2.Player,
		Choices: []int{len(d2.Options) - 1}}); err != nil {
		t.Fatalf("submit second dredge decline: %v", err)
	}
	if n := countAsks(t, e, since); n != 2 {
		t.Fatalf("total DecisionAsk events = %d, want 2", n)
	}
	draws := 0
	for _, ev := range e.L.Events[since:] {
		if ev.Kind == events.Draw && ev.Player == 0 {
			draws++
		}
	}
	if draws != 2 {
		t.Fatalf("draws delivered for a replaced 2-card gain = %d, want 2", draws)
	}
	if e.Pending() == nil {
		t.Fatal("no decision pending after the replacement body completed")
	}
	replayCheck(t, e, cfg)
}

// TestLifeReplacementDredgeAcceptAppliesThenContinues accepts the dredge:
// the mill-and-return applies to the draw that asked, and the parked
// remainder is still drawn afterwards — not lost the way the unfixed loop
// lost it.
func TestLifeReplacementDredgeAcceptAppliesThenContinues(t *testing.T) {
	e, cfg, ids := lichThugEngine(t, 245, "Golgari Thug", "Nefarious Lich")
	since := len(e.L.Events)
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 2})
	d := e.Pending()
	if d == nil || d.ResumeKind != "dredge" {
		t.Fatalf("pending = %+v, want the dredge ask", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
		t.Fatalf("submit dredge accept: %v", err)
	}
	milled := 0
	for _, ev := range e.L.Events[since:] {
		if ev.Kind == events.MoveZone && ev.From == state.ZLibrary && ev.To == state.ZGraveyard {
			milled++
		}
	}
	if milled != 4 {
		t.Fatalf("accepted dredge milled %d cards, want 4", milled)
	}
	if hand := e.G.Zone(state.ZHand, 0); len(hand) == 0 {
		t.Fatal("dredger did not return to hand: hand empty")
	} else {
		found := false
		for _, id := range hand {
			if id == ids[0] {
				found = true
			}
		}
		if !found {
			t.Fatal("dredger did not return to hand")
		}
	}
	if z := e.G.Zone(state.ZGraveyard, 0); len(z) != 4 {
		t.Fatalf("graveyard after the dredge = %d cards, want the 4 milled", len(z))
	}
	draws := 0
	for _, ev := range e.L.Events[since:] {
		if ev.Kind == events.Draw && ev.Player == 0 {
			draws++
		}
	}
	if draws != 1 {
		t.Fatalf("remaining draws after the accepted dredge = %d, want 1", draws)
	}
	replayCheck(t, e, cfg)
}

// TestLifeReplacementDredgeReParksForEachDraw proves the re-drive is itself
// suspension-aware: with a SECOND dredger still in the graveyard, the second
// draw poses its own ask after the first answer landed — two sequential
// asks, each answered, never two outstanding at once.
func TestLifeReplacementDredgeReParksForEachDraw(t *testing.T) {
	e, cfg, _ := lichThugEngine(t, 246, "Golgari Thug", "Golgari Thug", "Nefarious Lich")
	since := len(e.L.Events)
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 2})
	if n := countAsks(t, e, since); n != 1 {
		t.Fatalf("first draw's asks = %d, want 1", n)
	}
	d := e.Pending()
	if d == nil || d.ResumeKind != "dredge" {
		t.Fatalf("pending = %+v, want the first dredge ask", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
		t.Fatalf("submit first dredge accept: %v", err)
	}
	if n := countAsks(t, e, since); n != 2 {
		t.Fatalf("asks after the first answer = %d, want 2 (the second dredger's own sequential ask)", n)
	}
	d2 := e.Pending()
	if d2 == nil || d2.ResumeKind != "dredge" {
		t.Fatalf("pending after the first answer = %+v, want the second dredge ask", d2)
	}
	if err := e.Submit(decision.Intent{Seq: d2.Seq, Player: d2.Player,
		Choices: []int{len(d2.Options) - 1}}); err != nil {
		t.Fatalf("submit second dredge decline: %v", err)
	}
	if n := countAsks(t, e, since); n != 2 {
		t.Fatalf("total asks = %d, want 2", n)
	}
	draws := 0
	for _, ev := range e.L.Events[since:] {
		if ev.Kind == events.Draw && ev.Player == 0 {
			draws++
		}
	}
	if draws != 1 {
		t.Fatalf("declined second draw = %d Draw events, want 1", draws)
	}
	replayCheck(t, e, cfg)
}
