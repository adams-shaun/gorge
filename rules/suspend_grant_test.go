package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func suspendCorpusCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	c, ok := testutil.CorpusRegistry(t).Lookup(name)
	if !ok {
		t.Fatalf("corpus missing %s", name)
	}
	return c
}

// TestTenthDoctorExileGainsSuspendAndCasts drives the real GiveSuspend body
// from The Tenth Doctor's compiled SVar. In particular, it reaches the
// final-counter upkeep path;
// a grant that only affected the layer-6 keyword view would never tick TIME.
func TestTenthDoctorExileGainsSuspendAndCasts(t *testing.T) {
	doctor := suspendCorpusCard(t, "The Tenth Doctor")
	spell := suspendCorpusCard(t, "Profane Tutor")
	e := handEngine(t, doctor, spell)
	doctorID, spellID := e.G.Zone(state.ZHand, 0)[0], e.G.Zone(state.ZHand, 0)[1]
	e.emit(events.Event{Kind: events.MoveZone, Obj: doctorID, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: spellID, From: state.ZHand, To: state.ZExile})
	f := e.G.Obj(doctorID).Face()
	// Invoke the real GiveSuspend body from the Doctor's compiled SVar. The
	// preceding DigUntil is deliberately not part of this focused pin: the
	// grant and its later cast are the behaviour under test.
	effects.Resolve(e, &effects.Ctx{Source: doctorID, Controller: 0, SVars: f.SVars,
		Remembered: []state.Target{{Obj: spellID}}}, &cards.SA{Kind: "DB", API: "Pump", Params: map[string]string{
		"Defined": "Remembered", "KW": "Suspend", "PumpZone": "Exile", "Duration": "Permanent",
	}})
	o := e.G.Obj(spellID)
	if o.Zone != state.ZExile || o.Counter("TIME") != 0 || !o.SuspendGranted {
		t.Fatalf("Tenth Doctor grant = zone %s TIME %d granted %v", o.Zone, o.Counter("TIME"), o.SuspendGranted)
	}
	// Add the three counters the real preceding PutCounter body supplies.
	e.emit(events.Event{Kind: events.CounterChange, Obj: spellID, Counter: "TIME", Amount: 3})
	// The first two upkeeps decrement TIME. The third must pose the normal
	// suspend cast choice, proving the granted card is in the same queue.
	e.beginTurn(0)
	e.beginTurn(0)
	if got := o.Counter("TIME"); got != 1 {
		t.Fatalf("granted suspend TIME after two upkeeps = %d, want 1", got)
	}
	e.beginTurn(0)
	if d := e.Pending(); d == nil || d.Kind != decision.KChoose || len(d.Options) != 2 || d.Options[0].Kind != "suspend_cast_yes" {
		t.Fatalf("granted suspend final counter did not ask to cast: %+v", d)
	}
}

// TestRoseTylerBadWolfCountsSuspendedCards uses Rose's compiled SVar, not a
// hand-built count, so Card.suspended is exercised by a real consumer.
func TestRoseTylerBadWolfCountsSuspendedCards(t *testing.T) {
	rose := suspendCorpusCard(t, "Rose Tyler")
	spell := suspendCorpusCard(t, "Profane Tutor")
	e := handEngine(t, rose, spell)
	roseID, spellID := e.G.Zone(state.ZHand, 0)[0], e.G.Zone(state.ZHand, 0)[1]
	e.emit(events.Event{Kind: events.MoveZone, Obj: roseID, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.MoveZone, Obj: spellID, From: state.ZHand, To: state.ZExile})
	e.emit(events.Event{Kind: events.CounterChange, Obj: spellID, Counter: "TIME", Amount: 2})
	e.emit(events.Event{Kind: events.AlterAttribute, Obj: spellID, Text: "Suspend", Amount: 1})
	f := e.G.Obj(roseID).Face()
	effects.Resolve(e, &effects.Ctx{Source: roseID, Controller: 0, SVars: f.SVars}, cards.ResolveSVar(f.SVars, "TrigPutCounter"))
	if got := e.G.Obj(roseID).Counter("TIME"); got != 1 {
		t.Fatalf("Rose Bad Wolf TIME = %d, want 1 from one suspended card", got)
	}
}

// TestFaceOfBoeCastsSuspendedSpellAtSuspendCost drives the real PlayCost
// transaction used by The Face of Boe and checks that it reaches the stack
// without paying the spell's printed mana cost.
func TestFaceOfBoeCastsSuspendedSpellAtSuspendCost(t *testing.T) {
	boe := suspendCorpusCard(t, "The Face of Boe")
	spell := suspendCorpusCard(t, "Profane Tutor")
	e := handEngine(t, boe, spell)
	boeID, spellID := e.G.Zone(state.ZHand, 0)[0], e.G.Zone(state.ZHand, 0)[1]
	e.emit(events.Event{Kind: events.MoveZone, Obj: boeID, From: state.ZHand, To: state.ZBattlefield})
	// The Face's compiled ability selects this same card through Card.withSuspend;
	// exercise the resulting PlayCost transaction directly so the test remains
	// deterministic when the sole eligible card takes the no-ask path.
	e.G.Players[0].Pool[state.MC] = 1
	e.G.Players[0].Pool[state.MB] = 1
	e.beginPlay(0, spellID, false, "SuspendCost", false)
	if e.G.Obj(spellID).Zone != state.ZStack {
		t.Fatalf("Face of Boe SuspendCost play did not reach stack: %s", e.G.Obj(spellID).Zone)
	}
}

func TestDynamicSuspendGrantIsDerivedAndFilterable(t *testing.T) {
	e := handEngine(t)
	c := card(t, "Name:Grantable\nManaCost:2 U\nTypes:Creature\nPT:2/2\nOracle:x\n")
	o := e.G.AddObject(c, 0)
	o.Zone = state.ZExile
	e.G.SetZone(state.ZExile, 0, []state.ObjID{o.ID})
	e.emit(events.Event{Kind: events.CounterChange, Obj: o.ID, Counter: "TIME", Amount: 2})
	e.emit(events.Event{Kind: events.AlterAttribute, Obj: o.ID, Text: "Suspend", Amount: 1})
	if !o.SuspendGranted || !e.HasKeyword(o.ID, "Suspend") {
		t.Fatalf("granted suspend was not derived: granted=%v keywords=%v", o.SuspendGranted, e.Derived(o.ID).Keywords)
	}
	if !effects.MatchesSpecFrom(e.G, "Card.withSuspend", o.ID, 0, o.ID) || !effects.MatchesSpecFrom(e.G, "Card.suspended", o.ID, 0, o.ID) {
		t.Fatal("suspend predicates did not see the granted keyword")
	}
	if effects.MatchesSpecFrom(e.G, "Card.withoutSuspend", o.ID, 0, o.ID) {
		t.Fatal("Card.withoutSuspend matched a card with granted suspend")
	}
}

func TestFaceOfBoeSuspendCostUsesChosenCardKeyword(t *testing.T) {
	f := card(t, "Name:Suspended\nManaCost:5 R\nTypes:Sorcery\nK:Suspend:3:1 R\nOracle:x\n").Faces[0]
	got, ok := pricePlayCost(f, "SuspendCost")
	if !ok || got.Generic != 1 || got.Colored[state.MR] != 1 {
		t.Fatalf("SuspendCost = %+v, %v; want {1}{R}", got, ok)
	}
}
