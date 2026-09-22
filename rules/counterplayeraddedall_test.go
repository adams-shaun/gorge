// trig:CounterPlayerAddedAll — "Whenever you put one or more counters on
// a(n) <spec>" (Generous Patron, Rikku Resourceful Guardian, Kros Defense
// Contractor, All Will Be One). Pinned end to end on the two corpus cards
// the ticket names: the Patron's support-then-draw chain (one draw per
// counter-PLACING EVENT, never per counter) and Rikku's CantBlockBy window.

package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestGenerousPatronSupportPutsCountersAndDrawsOncePerBatch pins the Patron
// end to end: the ETB support asks "up to two other target creatures" (the
// Patron itself excluded, one counter per creature — not two on one), the
// answered put on the OPPONENT's creature queues exactly one draw trigger,
// and a later TWO-counter batch on the same creature draws exactly one more
// — one trigger per counter-placing event, never per counter.
func TestGenerousPatronSupportPutsCountersAndDrawsOncePerBatch(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	patron, ok := reg.Lookup("Generous Patron")
	if !ok {
		t.Fatal("corpus has no Generous Patron")
	}
	e := layerEngine(t)
	own := onBoard(t, e, 0, "Name:Goblin Skirmisher\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\nOracle:x\n")
	bear := onBoard(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	o := e.G.AddObject(patron, 0)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), o.ID))

	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZHand, To: state.ZBattlefield})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("pendingTriggers after Patron entered = %d, want 1 (the ETB support trigger)", len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	e.resolveTop()
	handAfterEntry := len(e.G.Zone(state.ZHand, 0))

	// The support ask: "up to two other target creatures" — the Patron must
	// NOT be offered (the Other exclusion), Min 0 ("up to").
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "counter_pick" {
		t.Fatalf("after the ETB trigger resolved: %+v, want the support counter_pick ask", d)
	}
	if d.Player != 0 || d.Min != 0 || d.Max != 2 {
		t.Fatalf("support ask player/range = seat %d %d..%d, want seat 0 0..2", d.Player, d.Min, d.Max)
	}
	if len(d.Options) != 2 {
		t.Fatalf("support ask options = %d, want 2 (the goblin and the bear, never the Patron)", len(d.Options))
	}
	if d.Options[0].Obj != own || d.Options[1].Obj != bear {
		t.Fatalf("support options = [%d %d], want [%d %d] in zone order", d.Options[0].Obj, d.Options[1].Obj, own, bear)
	}
	for _, opt := range d.Options {
		if opt.Obj == o.ID {
			t.Fatal("the Patron itself was offered as a support target")
		}
	}
	// Support puts ONE counter per creature: answer with only the opponent's
	// creature (the YouDontCtrl half).
	submitChoices(t, e, d.Options[1].Index)
	passUntilStackEmpty(t, e, 60)

	if got := e.G.Obj(bear).Counter("P1P1"); got != 1 {
		t.Fatalf("bear +1/+1 counters after support 2 on one creature = %d, want 1", got)
	}
	if got := e.G.Obj(own).Counter("P1P1"); got != 0 {
		t.Fatalf("own goblin +1/+1 counters = %d, want 0 (it was not chosen)", got)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handAfterEntry+1 {
		t.Fatalf("hand after the support put = %d, want %d (exactly one draw for the opponent-creature put)", got, handAfterEntry+1)
	}

	// The batch pin: a real two-counter put on the same creature (one
	// CounterChange event, Amount 2) queues ONE trigger and draws ONE card.
	twoCounter := card(t, "Name:Double Count\nManaCost:R\nTypes:Instant\n"+
		"A:SP$ PutCounter | Cost$ R | CounterType$ P1P1 | ValidTgts$ Creature | CounterNum$ 2\nOracle:x\n")
	sp := e.G.AddObject(twoCounter, 0)
	sp.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), sp.ID))
	addMana(t, e, 0, "R")
	d = e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("after addMana: %+v, want seat 0's priority", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == sp.ID {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the two-counter spell: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after casting: %+v, want the spell's target ask", d)
	}
	tIdx := -1
	for _, opt := range d.Options {
		if opt.Obj == bear {
			tIdx = opt.Index
		}
	}
	if tIdx < 0 {
		t.Fatalf("the bear was not offered as a target: %+v", d.Options)
	}
	submitChoices(t, e, tIdx)
	passUntilStackEmpty(t, e, 60)

	if got := e.G.Obj(bear).Counter("P1P1"); got != 3 {
		t.Fatalf("bear +1/+1 counters after the two-counter put = %d, want 3", got)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handAfterEntry+2 {
		t.Fatalf("hand after the two-counter batch = %d, want %d (ONE draw for the batch, not one per counter)", got, handAfterEntry+2)
	}
}

// TestRikkuCounterPutMakesTheCreatureUnblockable pins Rikku, Resourceful
// Guardian end to end: a counter you put on a creature registers the
// CantBlockBy effect (RememberObjects$ TriggeredObjectLKICopy resolves the
// gaining creature), and the combat blocker decision for the opponent no
// longer offers that creature as a blocker — the attack deals its damage
// unblocked. A control game without the put proves the assertions can fail.
func TestRikkuCounterPutMakesTheCreatureUnblockable(t *testing.T) {
	counterSpell := "Name:Count Up\nManaCost:R\nTypes:Instant\n" +
		"A:SP$ PutCounter | Cost$ R | CounterType$ P1P1 | ValidTgts$ Creature | CounterNum$ 1\nOracle:x\n"
	reg := testutil.CorpusRegistry(t)
	rikku, ok := reg.Lookup("Rikku, Resourceful Guardian")
	if !ok {
		t.Fatal("corpus has no Rikku, Resourceful Guardian")
	}
	// The whole board is built through logged events (deck genesis + real
	// battlefield entries), so the game replays from the log alone.
	cfg := seatZeroStart(Config{Seed: 6101, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{card(t, counterSpell), rikku}, mountainDeck(t, 38)...),
			append([]*cards.Card{card(t, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")}, mountainDeck(t, 39)...),
		},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	var spellID, rikkuID, bearID state.ObjID
	findRikku := func(zone state.Zone, p state.PlayerID) {
		for _, id := range e.G.Zone(zone, p) {
			if e.G.Obj(id).Face().Name == "Rikku, Resourceful Guardian" {
				rikkuID = id
			}
		}
	}
	findRikku(state.ZHand, 0)
	if rikkuID == 0 {
		findRikku(state.ZLibrary, 0)
	}
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if e.G.Obj(id).Face().Name == "Count Up" {
			spellID = id
		}
	}
	for _, id := range e.G.Zone(state.ZLibrary, 1) {
		if e.G.Obj(id).Face().Name == "Runeclaw Bear" {
			bearID = id
		}
	}
	if spellID == 0 || rikkuID == 0 || bearID == 0 {
		t.Fatalf("deck bridge failed: spell %d rikku %d bear %d", spellID, rikkuID, bearID)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: rikkuID, From: state.ZLibrary, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: bearID, From: state.ZLibrary, To: state.ZBattlefield})
	if o := e.G.Obj(spellID); o.Zone == state.ZLibrary {
		e.emit(events.Event{Kind: events.MoveZone, Obj: spellID, From: state.ZLibrary, To: state.ZHand})
		e.pending = nil
		e.Advance()
	}

	// Precondition: the target ask's first candidate is Rikku (seat 0's only
	// creature) and the board is where the rule looks.
	addMana(t, e, 0, "R")
	d := e.Pending()
	idx := -1
	for _, opt := range d.Options {
		if opt.Kind == "cast" && opt.Obj == spellID {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for the counter spell: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || len(d.Options) == 0 || d.Options[0].Obj != rikkuID {
		t.Fatalf("after casting: %+v, want the target ask with Rikku as option 0", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Obj(rikkuID).Counter("P1P1"); got != 1 {
		t.Fatalf("Rikku +1/+1 counters after the put = %d, want 1", got)
	}
	var ce *ContinuousEffect
	for i := range e.continuous {
		if e.continuous[i].Restriction == "CantBlockBy" && len(e.continuous[i].Remembered) == 1 && e.continuous[i].Remembered[0] == rikkuID {
			ce = &e.continuous[i]
		}
	}
	if ce == nil {
		t.Fatal("no CantBlockBy effect registered remembering Rikku after the counter put")
	}
	if !e.blockRestricted(bearID, rikkuID) {
		t.Fatal("seat 1's bear can still block the countered Rikku")
	}

	// End to end through the combat decisions: by turn 3 Rikku is past its
	// summoning sick turn; it attacks; the bear is offered no block; the
	// 3/4 Rikku (2/3 + the counter) deals 3.
	driveToStepAll(t, e, 3, 0, state.StepDeclareAttackers)
	e.askAttackers()
	submitAttackers(t, e, rikkuID)
	if got := e.G.Players[1].Life; got != 17 {
		t.Fatalf("defender life after the unblocked attack = %d, want 17", got)
	}
	replayCheck(t, e, cfg)
}

// TestRikkuWindowUncontrolledWithoutThePut is the can-fail control: the same
// board with NO counter put leaves the bear a legal blocker for Rikku and the
// blocker decision posed.
func TestRikkuWindowUncontrolledWithoutThePut(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	rikku, ok := reg.Lookup("Rikku, Resourceful Guardian")
	if !ok {
		t.Fatal("corpus has no Rikku, Resourceful Guardian")
	}
	e := combatEngine(t)
	rikkuID := onBoardCard(t, e, 0, rikku)
	e.G.Obj(rikkuID).SummonSick = false
	bear := onBoard(t, e, 1, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")

	if e.blockRestricted(bear, rikkuID) {
		t.Fatal("control: the bear was restricted without any counter put")
	}
	e.askAttackers()
	submitAttackers(t, e, rikkuID)
	e.G.Step = state.StepDeclareBlockers
	e.askBlockers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KBlockers {
		t.Fatalf("control blocker decision = %+v, want the bear offered as a blocker", d)
	}
	found := false
	for _, opt := range d.Options {
		if opt.Obj == bear {
			found = true
		}
	}
	if !found {
		t.Fatalf("control: the bear had no block option: %+v", d.Options)
	}
}
