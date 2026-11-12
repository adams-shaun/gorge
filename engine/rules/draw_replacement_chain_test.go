package rules

// The draw-replacement chain (CR 616.1e/614.5). A draw
// replacement's ReplaceWith$ body draws in the replaced event's place, and
// those draws are the modified events of CR 616.1e: the remaining applicable
// draw replacements get their opportunity on them, with every replacement
// already applied to this draw's lineage excluded (CR 614.5). A multi-card
// draw batch is also offered to the R:Event$ DrawCards class as ONE event
// before any card moves (Forge's Player.drawCards(n) DrawCards pass, Alms
// Collector's "would draw two or more cards"). These pins drive the real
// corpus scripts through the engine; the XMage port's DrawEffectsTest
// scenarios cover the same mechanism end to end.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestThoughtReflectionDrawsMeetNotionThief: Thought Reflection replaces the
// pending draw with two cards, and each of those draws must meet the
// opponent's Notion Thief ("instead that player skips that draw and you draw
// a card"). Before the chain the body's draws ran under the applyingReplacement
// guard, so the controller simply drew the replaced count and Notion Thief's
// controller drew nothing.
func TestThoughtReflectionDrawsMeetNotionThief(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := drawReplEngine(t, 750)
	tr := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Thought Reflection"))
	onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Notion Thief"))
	setupDrawLibrary(t, e, 0, mustCorpusCard(t, reg, "Mountain"))
	if len(e.G.Zone(state.ZLibrary, 0)) == 0 || e.G.Obj(tr).Zone != state.ZBattlefield {
		t.Fatal("precondition: seat 0's library and Thought Reflection must be in place")
	}
	e.pending = nil
	hand0, hand1 := len(e.G.Zone(state.ZHand, 0)), len(e.G.Zone(state.ZHand, 1))
	emitDraw(t, e, 0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KReplacement {
		t.Fatalf("precondition: the per-card draw must offer the CR 616.1 order choice, got %+v", d)
	}
	pick := indexOfObjOption(d, tr)
	if pick < 0 {
		t.Fatalf("Thought Reflection missing from the draw competition: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	if got := len(e.G.Zone(state.ZHand, 0)) - hand0; got != 0 {
		t.Fatalf("seat 0 hand delta = %d, want 0 (Notion Thief replaces both resulting draws)", got)
	}
	if got := len(e.G.Zone(state.ZHand, 1)) - hand1; got != 2 {
		t.Fatalf("seat 1 hand delta = %d, want 2 (Notion Thief's controller draws each replaced draw)", got)
	}
}

// TestAlmsCollectorReplacesThoughtReflectionBatch: Thought Reflection turns
// the pending one-card draw into a two-card batch, and Alms Collector ("if an
// opponent would draw two or more cards, instead you and that player each
// draw a card") must replace that batch as ONE event. The one-card draw
// itself never meets Alms Collector: its Number$ GE2 gate rejects a batch of
// one, so Thought Reflection is the only candidate there.
func TestAlmsCollectorReplacesThoughtReflectionBatch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := drawReplEngine(t, 751)
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Thought Reflection"))
	onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Alms Collector"))
	setupDrawLibrary(t, e, 0, mustCorpusCard(t, reg, "Mountain"))
	e.pending = nil
	hand0, hand1 := len(e.G.Zone(state.ZHand, 0)), len(e.G.Zone(state.ZHand, 1))
	emitDraw(t, e, 0)
	if d := e.Pending(); d != nil && d.Kind == decision.KReplacement {
		t.Fatalf("a one-card draw must not offer the Number$ GE2 batch replacement: %+v", d)
	}
	if got := len(e.G.Zone(state.ZHand, 0)) - hand0; got != 1 {
		t.Fatalf("seat 0 hand delta = %d, want 1 (Alms Collector replaced the two-card batch)", got)
	}
	if got := len(e.G.Zone(state.ZHand, 1)) - hand1; got != 1 {
		t.Fatalf("seat 1 hand delta = %d, want 1 (Alms Collector's controller draws too)", got)
	}
}

// TestAlmsCollectorIgnoresASingleCardDraw is the Number$ GE2 gate's own
// leaf: with no other replacement in play a one-card draw is not a
// "two or more cards" event, so Alms Collector must not replace it.
func TestAlmsCollectorIgnoresASingleCardDraw(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := drawReplEngine(t, 752)
	onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Alms Collector"))
	setupDrawLibrary(t, e, 0, mustCorpusCard(t, reg, "Mountain"))
	e.pending = nil
	hand0, hand1 := len(e.G.Zone(state.ZHand, 0)), len(e.G.Zone(state.ZHand, 1))
	emitDraw(t, e, 0)
	if d := e.Pending(); d != nil && d.Kind == decision.KReplacement {
		t.Fatalf("a one-card draw must not offer Alms Collector: %+v", d)
	}
	if got := len(e.G.Zone(state.ZHand, 0)) - hand0; got != 1 {
		t.Fatalf("seat 0 hand delta = %d, want 1 (the draw happens)", got)
	}
	if got := len(e.G.Zone(state.ZHand, 1)) - hand1; got != 0 {
		t.Fatalf("seat 1 hand delta = %d, want 0 (Alms Collector stays out of it)", got)
	}
}

// TestSecondBloodScrivenerReplacesTheFirstDrawsFirstCard is the 2013 ruling
// on two Blood Scriveners: the first replaces "draw a card" with "draw two
// cards and lose 1 life", and the second replaces the drawing of the FIRST of
// those two cards with the same thing, so the controller draws three and
// loses two. The first Scrivener must not apply again to the draws it
// produced (CR 614.5), and the second's Hellbent condition is evaluated when
// the resulting draw happens -- after its own two cards are in hand, the
// second resulting draw no longer qualifies.
func TestSecondBloodScrivenerReplacesTheFirstDrawsFirstCard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := drawReplEngine(t, 753)
	first := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Blood Scrivener"))
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Blood Scrivener"))
	setDrawHand(t, e, 0, 0)
	setupDrawLibrary(t, e, 0, mustCorpusCard(t, reg, "Mountain"))
	if got := len(e.G.Zone(state.ZHand, 0)); got != 0 {
		t.Fatalf("precondition: seat 0 hand = %d, want 0 (Hellbent must hold)", got)
	}
	e.pending = nil
	life := e.G.Players[0].Life
	emitDraw(t, e, 0)
	d := e.Pending()
	if d == nil || d.Kind != decision.KReplacement {
		t.Fatalf("precondition: the two Scriveners must compete for the draw, got %+v", d)
	}
	pick := indexOfObjOption(d, first)
	if pick < 0 {
		t.Fatalf("the first Scrivener missing from the competition: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	if got := len(e.G.Zone(state.ZHand, 0)); got != 3 {
		t.Fatalf("seat 0 hand = %d, want 3 (draw two, then one more)", got)
	}
	if got := e.G.Players[0].Life; got != life-2 {
		t.Fatalf("seat 0 life = %d, want %d (both Scriveners lost 1 life)", got, life-2)
	}
}

// TestCompetingDrawCardsReplacementsAskTheDrawer pins the batch competition's
// own CR 616.1 order choice: two DrawCards replacements (Quantum Riddler and
// Alms Collector) can both modify the same two-card batch, so the DRAWER is
// asked which applies first. Quantum Riddler's chosen body draws "that many
// plus one" (three), and because Alms Collector never applied to the original
// batch it is still applicable to the modified one (CR 616.1f), where it
// replaces the three-card batch with one card each.
func TestCompetingDrawCardsReplacementsAskTheDrawer(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := drawReplEngine(t, 754)
	qr := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Quantum Riddler"))
	onBoardCard(t, e, 1, mustCorpusCard(t, reg, "Alms Collector"))
	setDrawHand(t, e, 0, 1) // Quantum Riddler's LE1-over-hand gate
	e.pending = nil
	hand0, hand1 := len(e.G.Zone(state.ZHand, 0)), len(e.G.Zone(state.ZHand, 1))
	e.CountReplacementProposed(events.Event{Kind: events.DrawBatchProposal,
		Player: 0, Obj: qr, Amount: 2})
	d := e.Pending()
	if d == nil || d.Kind != decision.KReplacement || d.Player != 0 || len(d.Options) != 2 {
		t.Fatalf("batch competition = %+v, want the draw-er ordering two DrawCards candidates", d)
	}
	pick := indexOfObjOption(d, qr)
	if pick < 0 {
		t.Fatalf("Quantum Riddler missing from the batch competition: %+v", d.Options)
	}
	submitChoices(t, e, pick)
	if got := len(e.G.Zone(state.ZHand, 0)) - hand0; got != 1 {
		t.Fatalf("seat 0 hand delta = %d, want 1 (Alms Collector replaced the resulting three-card batch)", got)
	}
	if got := len(e.G.Zone(state.ZHand, 1)) - hand1; got != 1 {
		t.Fatalf("seat 1 hand delta = %d, want 1", got)
	}
}

// driveThreeRiddlers poses one DrawBatchProposal against three Quantum
// Riddlers and answers every replacement ask with pickOption(options). It
// returns the option counts of the asks and the hand/library deltas.
func driveThreeRiddlers(t *testing.T, pickOption func(n int) int) (asks []int, handDelta, libDelta int) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e := drawReplEngine(t, 754)
	var qr state.ObjID
	for i := 0; i < 3; i++ {
		qr = onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Quantum Riddler"))
	}
	setDrawHand(t, e, 0, 1)
	e.pending = nil
	hand0, lib0 := len(e.G.Zone(state.ZHand, 0)), len(e.G.Zone(state.ZLibrary, 0))
	e.CountReplacementProposed(events.Event{Kind: events.DrawBatchProposal,
		Player: 0, Obj: qr, Amount: 1})
	for len(asks) < 12 {
		d := e.Pending()
		if d == nil || d.Kind != decision.KReplacement {
			break
		}
		asks = append(asks, len(d.Options))
		submitChoices(t, e, pickOption(len(d.Options)))
	}
	return asks, len(e.G.Zone(state.ZHand, 0)) - hand0, lib0 - len(e.G.Zone(state.ZLibrary, 0))
}

// TestThreeQuantumRiddlersEachApplyOnceToOneDraw: a parked order ask must
// remember the draw lineage's applied set (CR 614.5), or the answered body's
// nested proposal re-offers a Riddler that already applied and the asks never
// end. Three Riddlers: 3 options, then 2, then the last applies unasked.
func TestThreeQuantumRiddlersEachApplyOnceToOneDraw(t *testing.T) {
	t.Parallel()
	for name, pick := range map[string]func(int) int{
		"first": func(int) int { return 0 },
		"last":  func(n int) int { return n - 1 },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			asks, handDelta, libDelta := driveThreeRiddlers(t, pick)
			if len(asks) == 0 || asks[0] != 3 {
				t.Fatalf("first ask options = %v, want 3 Riddler candidates", asks)
			}
			if len(asks) != 2 || asks[1] != 2 {
				t.Fatalf("asks = %v, want [3 2]", asks)
			}
			if handDelta != 4 || libDelta != 4 {
				t.Fatalf("hand delta %d, library delta %d, want 4 and 4 (1 + one per Riddler)", handDelta, libDelta)
			}
		})
	}
}
