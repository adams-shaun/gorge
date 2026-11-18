package rules

// Auntie Ool, Cursewretch's Ward—Blight 2 against a targeting opponent who
// controls no creature: the pay election is offered, but paying it is
// impossible, so beginWardPayment must decline without asking -- its blight
// arm posed a Min 1 / Max 1 KChoose with zero options, the unanswerable shape
// a seat answers only with a rejected empty intent (hunt seed 779, intent
// 1062: "expected 1..1 choices, got 0"). An unpaid ward cost counters the
// warded spell (CR 702.22). The payable control pins the guard to the empty
// case only: one controlled creature still pays, two M1M1 counters on it.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// wardBlightEngine: Auntie Ool, Cursewretch under seat 0, an opposing bolt in
// seat 1's hand, and creatures per seat 1's board argument.
func wardBlightEngine(t *testing.T, seat1Creatures int) (*Engine, state.ObjID) {
	t.Helper()
	e := handEngine(t, corpusAlternativeCard(t, "Auntie Ool, Cursewretch"))
	for _, o := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(o).Face().Name == "Auntie Ool, Cursewretch" {
			e.emit(events.Event{Kind: events.MoveZone, Obj: o, From: state.ZHand, To: state.ZBattlefield})
			break
		}
	}
	ool := e.G.Zone(state.ZBattlefield, 0)[0]
	for i := 0; i < seat1Creatures; i++ {
		// Phyrexian Fleshgorger: 7/5, so the two M1M1 counters the paid
		// ward places on it leave it alive on the battlefield.
		o := e.G.AddObject(corpusAlternativeCard(t, "Phyrexian Fleshgorger"), 1)
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	}
	bolt := e.G.AddObject(card(t, "Name:Bolt\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n"), 1)
	e.G.SetZone(state.ZHand, 1, append(e.G.Zone(state.ZHand, 1), bolt.ID))
	// Any event posts the first priority decision; handEngine's genesis ends
	// with none pending.
	addMana(t, e, 1, "R")
	return e, ool
}

// electWardPay casts seat 1's bolt at target and elects Pay on the ward
// election, reporting whether a Pay option was offered.
func electWardPay(t *testing.T, e *Engine, target state.ObjID) bool {
	t.Helper()
	opponentBoltAt(t, e, target)
	d := e.Pending()
	if !isWardPayAsk(d) {
		t.Fatalf("ward pay election not posed, got %+v", d)
	}
	for _, o := range d.Options {
		if o.Mode == decision.ModeUnlessPay {
			submitChoices(t, e, o.Index)
			return true
		}
	}
	return false
}

func TestWardBlightUnpayableDeclinesWithoutAsking(t *testing.T) {
	t.Parallel()
	e, ool := wardBlightEngine(t, 0)
	if n := len(e.G.Zone(state.ZBattlefield, 1)); n != 0 {
		t.Fatalf("precondition: seat 1 controls %d permanents, want none", n)
	}
	bolt := e.G.Zone(state.ZHand, 1)[0]
	if !electWardPay(t, e, ool) {
		t.Fatal("precondition: no Pay option in the ward election")
	}
	for i := 0; i < 30 && e.Pending() != nil; i++ {
		d := e.Pending()
		if d.Kind == decision.KChoose && d.Prompt == "Choose a creature to blight" {
			t.Fatalf("blight ask posed with %d options for Min %d: %+v", len(d.Options), d.Min, d)
		}
		wardLifePass(t, e)
	}
	if z := e.G.Obj(bolt).Zone; z != state.ZGraveyard {
		t.Fatalf("bolt zone = %s, want Graveyard (countered by the unpaid ward)", z)
	}
	if dmg := e.G.Obj(ool).Damage; dmg != 0 {
		t.Fatalf("Auntie Ool took %d damage from a countered bolt", dmg)
	}
	if got := blightCounters(t, e); len(got) != 0 {
		t.Fatalf("M1M1 counters placed = %v, want none", got)
	}
}

func TestWardBlightPayableStillBlightsAndResolves(t *testing.T) {
	t.Parallel()
	e, ool := wardBlightEngine(t, 1)
	if n := len(e.G.Zone(state.ZBattlefield, 1)); n != 1 {
		t.Fatalf("precondition: seat 1 controls %d permanents, want 1", n)
	}
	bear := e.G.Zone(state.ZBattlefield, 1)[0]
	if !electWardPay(t, e, ool) {
		t.Fatal("precondition: no Pay option in the ward election")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Prompt != "Choose a creature to blight" {
		t.Fatalf("pending %+v, want seat 1's blight ask", d)
	}
	if len(d.Options) < d.Min {
		t.Fatalf("blight ask posed with %d options for Min %d", len(d.Options), d.Min)
	}
	submitChoices(t, e, d.Options[0].Index)
	if got := blightCounters(t, e); got[bear] != 2 || len(got) != 1 {
		t.Fatalf("counters = %v, want 2 M1M1 on the blighted bear %d", got, bear)
	}
	passUntilStackEmpty(t, e, 20)
	if dmg := e.G.Obj(ool).Damage; dmg != 3 {
		t.Fatalf("Auntie Ool damage = %d, want 3 (the ward was paid)", dmg)
	}
	if z := e.G.Obj(bear).Zone; z != state.ZBattlefield {
		t.Fatalf("fleshgorger zone = %s, want Battlefield (7/5 survives 2 M1M1)", z)
	}
}
