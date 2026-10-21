package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The K:Tribute expansion (CR 702.104, task kw-tribute-expansion), pinned on
// real corpus carriers:
//
//   - Snake of the Golden Grove (Tribute 3): the ETB replacement poses the
//     two asks -- the entering creature's controller picks the opponent,
//     then that opponent elects pay/decline -- an acceptance enters the
//     snake with three +1/+1 counters and the "if tribute wasn't paid"
//     trigger silent, a decline enters it plain and the trigger fires (4
//     life).
//   - Ornitharch (Tribute 2): a decline mints the two Bird tokens, the same
//     Execute$ path every carrier shares.

// enterTributee emits the named hand card's battlefield entry (the ETB
// replacement fires on the Move) and returns the pending decision -- the
// opponent-pick ask the replacement's body poses. The entry runs as a
// resolution-kernel probe (kr5EnterDevourer's shape): an ask a replacement
// body poses is posable only inside a kernel run, which poses the decision
// and unwinds to its checkpoint instead of applying the R-9 stand-in.
func enterTributee(t *testing.T, e *Engine, name string) *decision.Decision {
	t.Helper()
	id := searchMoveByName(t, e, name, state.ZHand)
	e.pending = nil
	e.probe(func() {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	})
	return e.Pending()
}

// drainTriggerAndStack drives the queued trigger onto the stack and lets it
// resolve: pass every priority ask, answer anything else with its first
// option, until the queue and the stack are both empty (a live priority
// decision is the game's ordinary state, not something to drain).
func drainTriggerAndStack(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 40; i++ {
		if len(e.pendingTriggers) == 0 && len(e.G.Stack) == 0 {
			return
		}
		d := e.Pending()
		if d == nil {
			e.priorityRound()
			continue
		}
		idx := d.Options[0].Index
		if d.Kind == decision.KPriority {
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
		}
		submitChoices(t, e, idx)
	}
	t.Fatal("the unpaid trigger's resolution did not drain")
}

func TestTributePaidEntersWithCounters(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Snake of the Golden Grove"))

	d := enterTributee(t, e, "Snake of the Golden Grove")
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "tribute_payer" ||
		d.Player != 0 || len(d.Options) != 1 || d.Options[0].Player != 1 {
		t.Fatalf("payer ask: %+v, want seat 0 asked to pick its one opponent", d)
	}
	submitChoices(t, e, d.Options[0].Index)

	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "tribute" ||
		d.Player != 1 || len(d.Options) != 2 {
		t.Fatalf("tribute ask: %+v, want seat 1 asked pay/decline", d)
	}
	submitChoices(t, e, d.Options[0].Index) // yes

	o := e.G.Obj(idOf(t, e, "Snake of the Golden Grove"))
	if got := o.Counter("P1P1"); got != 3 {
		t.Fatalf("accepted snake entered with %d P1P1, want 3 (Tribute 3)", got)
	}
	if !o.Tributed {
		t.Fatal("accepted tribute did not fold the Tributed designation")
	}
	if n := len(e.pendingTriggers); n != 0 {
		t.Fatalf("%d trigger(s) queued on an accepted tribute, want the unpaid trigger silent", n)
	}
}

func TestTributeDeclinedFiresUnpaidTrigger(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Snake of the Golden Grove"))

	d := enterTributee(t, e, "Snake of the Golden Grove")
	if d == nil || d.ResumeKind != "tribute_payer" {
		t.Fatalf("payer ask: %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	d = e.Pending()
	if d == nil || d.ResumeKind != "tribute" || d.Player != 1 {
		t.Fatalf("tribute ask: %+v", d)
	}
	submitChoices(t, e, d.Options[1].Index) // no
	drainTriggerAndStack(t, e)

	o := e.G.Obj(idOf(t, e, "Snake of the Golden Grove"))
	if got := o.Counter("P1P1"); got != 0 {
		t.Fatalf("declined snake entered with %d P1P1, want 0", got)
	}
	if o.Tributed {
		t.Fatal("a declined tribute folded the Tributed designation")
	}
	if life := e.G.Players[0].Life; life != 24 {
		t.Fatalf("controller life %d, want 24 (the unpaid trigger gained 4)", life)
	}
}

func TestTributeOrnitharchDeclineMintsBirds(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Ornitharch"))

	d := enterTributee(t, e, "Ornitharch")
	if d == nil || d.ResumeKind != "tribute_payer" {
		t.Fatalf("payer ask: %+v", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	submitChoices(t, e, e.Pending().Options[1].Index) // decline
	drainTriggerAndStack(t, e)

	if o := e.G.Obj(idOf(t, e, "Ornitharch")); o.Counter("P1P1") != 0 || o.Tributed {
		t.Fatalf("declined Ornitharch: counters %d tributed %v, want 0/false",
			o.Counter("P1P1"), o.Tributed)
	}
	birds := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if b := e.G.Obj(id); b != nil && b.Face() != nil && b.Face().Name == "Bird Token" {
			birds++
		}
	}
	if birds != 2 {
		t.Fatalf("%d Bird tokens after a declined tribute, want 2", birds)
	}
}

// tributeFourSeats is the 4-seat variant of handEngineTokens: seat 1 holds
// the hand, seat 0 is the active player, so both tribute asks are answered
// by players other than the active one.
func tributeFourSeats(t *testing.T, hand ...*cards.Card) *Engine {
	t.Helper()
	decks := make([][]*cards.Card, 4)
	for i := range decks {
		decks[i] = mountainDeck(t, 40)
	}
	e := New(Config{Seed: 1, Names: []string{"a", "b", "c", "d"},
		Decks: decks, Tokens: testutil.CorpusRegistry(t).Tokens})
	for p := state.PlayerID(0); p < 4; p++ {
		e.G.SetZone(state.ZHand, p, nil)
	}
	var ids []state.ObjID
	for _, c := range hand {
		o := e.G.AddObject(c, 1)
		o.Zone = state.ZHand
		ids = append(ids, o.ID)
	}
	e.G.SetZone(state.ZHand, 1, ids)
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	return e
}

// enterTributeeSeat is enterTributee for a player other than seat 0.
func enterTributeeSeat(t *testing.T, e *Engine, p state.PlayerID, name string) *decision.Decision {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, p) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil || o.Face().Name != name {
			continue
		}
		e.pending = nil
		e.probe(func() {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
		})
		return e.Pending()
	}
	t.Fatalf("%q absent from seat %d's hand", name, p)
	return nil
}

// TestTributeFourSeatsOpponentNotActiveChooses pins the ask ownership at
// four seats: the entering creature's controller (seat 1, not the active
// player) picks ONE opponent from the three offered, and the picked
// opponent (seat 2, still not the active player) is the one asked
// pay/decline. An acceptance places the Tribute 3 counters and leaves the
// unpaid trigger silent.
func TestTributeFourSeatsOpponentNotActiveChooses(t *testing.T) {
	t.Parallel()
	e := tributeFourSeats(t, corpusAlternativeCard(t, "Snake of the Golden Grove"))

	d := enterTributeeSeat(t, e, 1, "Snake of the Golden Grove")
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "tribute_payer" || d.Player != 1 {
		t.Fatalf("payer ask: %+v, want seat 1 (the controller, not the active player) asked to pick", d)
	}
	if len(d.Options) != 3 {
		t.Fatalf("payer ask offered %d options, want 3 opponents", len(d.Options))
	}
	pick := -1
	for _, o := range d.Options {
		if o.Player == 2 {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("payer ask did not offer seat 2: %+v", d.Options)
	}
	submitChoices(t, e, pick)

	d = e.Pending()
	if d == nil || d.ResumeKind != "tribute" || d.Player != 2 {
		t.Fatalf("tribute ask: %+v, want seat 2 asked pay/decline", d)
	}
	if d.Player == e.G.Active {
		t.Fatalf("chooser seat %d is the active player %d, want an opponent", d.Player, e.G.Active)
	}
	submitChoices(t, e, d.Options[0].Index) // pay

	var oid state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 1) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Snake of the Golden Grove" {
			oid = id
		}
	}
	o := e.G.Obj(oid)
	if o == nil {
		t.Fatal("no battlefield snake under seat 1")
	}
	if got := o.Counter("P1P1"); got != 3 || !o.Tributed {
		t.Fatalf("paid snake: %d P1P1 tributed=%v, want 3/true", got, o.Tributed)
	}
	if n := len(e.pendingTriggers); n != 0 {
		t.Fatalf("%d trigger(s) queued on an accepted tribute, want the unpaid trigger silent", n)
	}
}

// TestTributeRedirectedEntryClearsDesignation pins the one-stint rule the
// review's break attempt named: an accepted tribute folds the designation
// before the Move folds, so an entry a later replacement redirects off the
// battlefield must clear it. Otherwise a stale true would suppress the
// unpaid trigger when the card really enters later.
func TestTributeRedirectedEntryClearsDesignation(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "Snake of the Golden Grove"))
	var id state.ObjID
	for _, h := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(h); o != nil && o.Face() != nil && o.Face().Name == "Snake of the Golden Grove" {
			id = h
		}
	}
	if id == 0 {
		t.Fatal("Snake of the Golden Grove absent from the hand")
	}

	e.emit(events.Event{Kind: events.AlterAttribute, Obj: id,
		Text: "Tributed", Amount: 1})
	if !e.G.Obj(id).Tributed {
		t.Fatal("precondition: the acceptance did not fold Tributed")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id,
		From: state.ZHand, To: state.ZGraveyard})
	if e.G.Obj(id).Tributed {
		t.Fatal("a redirected entry left the Tributed designation on the card")
	}
}
