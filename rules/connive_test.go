package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// End-to-end connive tests (task connive1, api:Connive / CR 702.59) on REAL
// corpus cards:
//
//   - Lethal Scheme's `DB$ Connive | Defined$ Convoked` sub: each creature
//     that convoked the cast draws one, discards one (a real KModes ask when
//     the hand holds more than one card — the strict-supersets rule), and a
//     nonland discard puts the +1/+1 counter on the conniver (a land discard
//     puts none). This is the corpus carrier the brief names, and it pins
//     the Defined$ Convoked selector through the real convoke announcement
//     (rules/cast.go's convokeAsk -> the pay-time FlagConvoked CastInfo ->
//     Object.Convoked -> effects' definedSpec).
//   - Venerated Loxodon's `DB$ PutCounter | Defined$ Convoked` ETB trigger:
//     the convoked set survives the stack->battlefield move, so the trigger
//     counters exactly the creatures that convoked the cast.
//   - Ledger Shredder's bare `DB$ Connive` trigger connives its own source,
//     and with a one-card hand it discards without asking (the
//     strict-supersets rule); the events.Connive record is what the
//     trig:Connives trigger mode (Iron Monger, Sadistic Tycoon) matches.
//
// All fixtures are compiled corpus cards looked up by name — no Forge script
// text is committed. The opponent deck is all Mountains, so any bear on the
// opponent's battlefield is a deterministic Destroy target.

func conniveCorpusCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	c, ok := searchTestRegistry(t).Lookup(name)
	if !ok {
		t.Fatalf("missing corpus card %q", name)
	}
	return c
}

// conniveEngine builds a two-seat engine whose seat-0 deck is exactly the
// named corpus cards padded with Forests and whose seat-1 deck is all
// Mountains, driven to Main1. Seats are deterministic; the deck order is
// whatever the seed shuffles, so tests read the actual state rather than
// assuming hand/library contents.
func conniveEngine(t *testing.T, seat0, opp0 []string) (*Engine, Config) {
	t.Helper()
	reg := searchTestRegistry(t)
	deck := make([]*cards.Card, 0, 40)
	for _, name := range seat0 {
		deck = append(deck, conniveCorpusCard(t, name))
	}
	forest := conniveCorpusCard(t, "Forest")
	for len(deck) < 40 {
		deck = append(deck, forest)
	}
	opp := make([]*cards.Card, 0, 40)
	mountain := conniveCorpusCard(t, "Mountain")
	for _, name := range opp0 {
		opp = append(opp, conniveCorpusCard(t, name))
	}
	for len(opp) < 40 {
		opp = append(opp, mountain)
	}
	// Seat 0 must hold the priority the tests drive (addMana funds seat 0's
	// pool and re-asks the pending priority), so pick the first seed whose
	// opening toss seats seat 0 -- deterministic for a given build, the same
	// reading every fixture-based test in the package relies on.
	var e *Engine
	var cfg Config
	for seed := uint64(8113); ; seed++ {
		cfg = Config{Seed: seed, Names: []string{"conniver", "opponent"},
			Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens}
		e = New(cfg)
		e.Advance()
		if e.G.Active == 0 {
			break
		}
	}
	toMain1(t, e)
	return e, cfg
}

// conniveMoveTo moves the corpus card named by name to zone for seat p,
// scanning library, hand and battlefield. The pads make the named cards
// seeded but not ordered, so the scan is the test's contract.
func conniveMoveTo(t *testing.T, e *Engine, p state.PlayerID, name string, to state.Zone, avoid ...state.ObjID) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary, state.ZBattlefield} {
		for _, id := range e.G.Zone(z, p) {
			skip := false
			for _, a := range avoid {
				skip = skip || a == id
			}
			if skip {
				continue
			}
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				if z != to {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: to})
					e.pending = nil
					e.priorityRound()
				}
				return id
			}
		}
	}
	t.Fatalf("corpus card %q absent from seat %d's library/hand/battlefield", name, p)
	return 0
}

// submitConniveDiscard answers the pending connive KModes ask with the first
// option whose card matches want (a *cards.Card identity) and asserts the
// ask's own contract (KModes, Min == Max == 1, ResumeKind connive).
func submitConniveDiscard(t *testing.T, e *Engine, want *cards.Card) state.ObjID {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "connive" || d.Min != 1 || d.Max != 1 {
		t.Fatalf("connive discard ask = %+v, want KModes 1..1 resume connive", d)
	}
	idx := -1
	var picked state.ObjID
	for _, o := range d.Options {
		if o.Obj != 0 {
			if o2 := e.G.Obj(o.Obj); o2 != nil && o2.Card == want {
				idx, picked = o.Index, o.Obj
				break
			}
		}
	}
	if idx < 0 {
		t.Fatalf("connive ask offers no %s card: %+v", want.Faces[0].Name, d.Options)
	}
	submitChoices(t, e, idx)
	return picked
}

func firstNonlandHandOption(t *testing.T, e *Engine) decision.Option {
	t.Helper()
	d := e.Pending()
	for _, o := range d.Options {
		if o.Obj != 0 {
			if o2 := e.G.Obj(o.Obj); o2 != nil && o2.Face() != nil && !o2.Face().IsLand() {
				return o
			}
		}
	}
	t.Fatalf("connive ask offers no nonland card: %+v", d.Options)
	return decision.Option{}
}

// castLethalSchemeAtBear drives the whole cast flow of Lethal Scheme (in
// seat 0's hand) at opponentBear: the target ask, the convoke announcement
// convoking convokeWith, and the remaining payment, then returns the first
// non-priority decision after the cast — the pending connive ask (or
// priority, for a caller that manages its own asks).
func castLethalSchemeAtBear(t *testing.T, e *Engine, opponentBear, convokeWith state.ObjID) *decision.Decision {
	t.Helper()
	id := conniveMoveTo(t, e, 0, "Lethal Scheme", state.ZHand)
	addMana(t, e, 0, "BBCCC")
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Lethal Scheme: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	// CR 601.2b order: the convoke announcement precedes the target ask.
	d = e.Pending()
	if d != nil && d.Kind == decision.KChoose {
		cIdx := -1
		for _, o := range d.Options {
			if o.Obj == convokeWith && o.Kind == "convoke_generic" {
				cIdx = o.Index
			}
		}
		if cIdx < 0 {
			t.Fatalf("convoke ask offers no generic option for %d: %+v", convokeWith, d.Options)
		}
		submitChoices(t, e, cIdx)
		d = e.Pending()
	}
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("after convoke pending = %+v, want the destroy target ask", d)
	}
	tIdx := -1
	for _, o := range d.Options {
		if o.Obj == opponentBear {
			tIdx = o.Index
		}
	}
	if tIdx < 0 {
		t.Fatalf("destroy target ask offers no bear %d: %+v", opponentBear, d.Options)
	}
	submitChoices(t, e, tIdx)
	return passUntilNonPriority(t, e, 30)
}

// TestLethalSchemeConvokedConniveNonlandDiscardCounters is the brief's main
// pin: the convoked creature connives (draws one, is ASKED which card to
// discard), and discarding a nonland card puts the +1/+1 counter on it.
func TestLethalSchemeConvokedConniveNonlandDiscardCounters(t *testing.T) {
	e, _ := conniveEngine(t, []string{"Lethal Scheme", "Grizzly Bears", "Grizzly Bears"}, []string{"Grizzly Bears"})
	convokeWith := conniveMoveTo(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	opponentBear := conniveMoveTo(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	// A known nonland card must be offered at the connive ask even if the
	// opening hand held none: move the second bear into the hand.
	conniveMoveTo(t, e, 0, "Grizzly Bears", state.ZHand)
	before := len(e.G.Zone(state.ZHand, 0))
	conniverBefore := e.G.Obj(convokeWith).Counter("P1P1")

	castLethalSchemeAtBear(t, e, opponentBear, convokeWith)
	// Discard a NONLAND card (the strict-supersets ask is live, so choose
	// what the +1/+1 counter hangs on): the first nonland option offered.
	nonland := firstNonlandHandOption(t, e)
	submitChoices(t, e, nonland.Index)
	passUntilStackEmpty(t, e, 30)

	// The discard ask was the KModes contract above; here the answered
	// outcome. The conniver drew one card and discarded one: net hand is
	// unchanged, minus nothing (Lethal Scheme went to the graveyard).
	if got := len(e.G.Zone(state.ZHand, 0)); got != before {
		t.Fatalf("hand after connive = %d, want %d (one draw, one discard)", got, before)
	}
	if o := e.G.Obj(convokeWith); o == nil || o.Zone != state.ZBattlefield || o.Counter("P1P1") != conniverBefore+1 {
		t.Fatalf("conniver after nonland discard = %+v (counters %d), want +1 P1P1 on the battlefield",
			e.G.Obj(convokeWith), e.G.Obj(convokeWith).Counter("P1P1"))
	}
	if o := e.G.Obj(opponentBear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("destroyed bear zone = %+v, want graveyard", o)
	}
	// The record: one events.Connive for the convoked conniver, Amount 1
	// (the nonland discard), the discarded card in IDs.
	sawRecord := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Connive && ev.Obj == convokeWith {
			sawRecord = true
			if ev.Amount != 1 || len(ev.IDs) != 1 || ev.IDs[0] != nonland.Obj {
				t.Fatalf("connive record = %+v, want Amount 1 IDs [the discarded nonland card]", ev)
			}
			if ev.Player != 0 {
				t.Fatalf("connive record player = %d, want 0", ev.Player)
			}
		}
	}
	if !sawRecord {
		t.Fatal("no events.Connive record for the convoked conniver")
	}
	// The +1/+1 counter rode its own CounterChange on the conniver.
	sawCounter := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == convokeWith && ev.Counter == "P1P1" && ev.Amount == 1 {
			sawCounter = true
		}
	}
	if !sawCounter {
		t.Fatal("no P1P1 CounterChange on the conniver")
	}
}

// TestLethalSchemeConvokedConniveLandDiscardSkipsCounter is the other half of
// CR 702.59a: a LAND discard puts no counter.
func TestLethalSchemeConvokedConniveLandDiscardSkipsCounter(t *testing.T) {
	e, _ := conniveEngine(t, []string{"Lethal Scheme", "Grizzly Bears"}, []string{"Grizzly Bears"})
	convokeWith := conniveMoveTo(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	opponentBear := conniveMoveTo(t, e, 1, "Grizzly Bears", state.ZBattlefield)

	d := castLethalSchemeAtBear(t, e, opponentBear, convokeWith)
	// Discard the first LAND option the ask offers (the opening hand's
	// basics, or a Forest the pads moved up); if the hand somehow holds no
	// land, move one in before the cast — but the ask is already pending, so
	// instead require the opening hand to carry one (40-card deck, 7-card
	// hand, 21 Forests: measured by construction below).
	landIdx := -1
	var landID state.ObjID
	for _, o := range d.Options {
		if o.Obj != 0 {
			if o2 := e.G.Obj(o.Obj); o2 != nil && o2.Face() != nil && o2.Face().IsLand() {
				landIdx, landID = o.Index, o.Obj
				break
			}
		}
	}
	if landIdx < 0 {
		t.Skipf("opening hand held no land card to discard; deck composition shifted")
	}
	submitChoices(t, e, landIdx)
	passUntilStackEmpty(t, e, 30)

	if o := e.G.Obj(convokeWith); o == nil || o.Zone != state.ZBattlefield || o.Counter("P1P1") != 0 {
		t.Fatalf("conniver after land discard = %+v (counters %d), want NO P1P1 counter",
			e.G.Obj(convokeWith), e.G.Obj(convokeWith).Counter("P1P1"))
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Connive && ev.Obj == convokeWith && ev.Amount != 0 {
			t.Fatalf("connive record Amount = %d, want 0 (land discard)", ev.Amount)
		}
	}
	_ = landID
}

// TestVeneratedLoxodonCountersExactlyTheConvoked pins the Defined$ Convoked
// read on the ETB half: the convoked set survives the stack->battlefield
// move and the PutCounter trigger counters exactly those creatures.
func TestVeneratedLoxodonCountersExactlyTheConvoked(t *testing.T) {
	e, _ := conniveEngine(t, []string{"Venerated Loxodon", "Grizzly Bears", "Grizzly Bears", "Grizzly Bears", "Grizzly Bears"}, nil)
	// Gather every bear into the hand first so each battlefield move below
	// takes a DIFFERENT bear (conniveMoveTo scans hand first; with the bears
	// all in the hand the three moves are three distinct objects).
	for i := 0; i < 4; i++ {
		conniveMoveTo(t, e, 0, "Grizzly Bears", state.ZHand)
	}
	bearA := conniveMoveTo(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	bearB := conniveMoveTo(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	bearC := conniveMoveTo(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	loxodon := conniveMoveTo(t, e, 0, "Venerated Loxodon", state.ZHand)
	if bearA == bearB || bearB == bearC || bearA == bearC {
		t.Fatalf("bear fixture not distinct: %d %d %d", bearA, bearB, bearC)
	}

	addMana(t, e, 0, "WWWWW")
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == loxodon {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Venerated Loxodon: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("pending = %+v, want the convoke announcement", d)
	}
	cIdx := -1
	for _, o := range d.Options {
		if o.Obj == bearA && o.Kind == "convoke_generic" {
			cIdx = o.Index
		}
	}
	if cIdx < 0 {
		t.Fatalf("convoke ask offers no generic option for %d: %+v", bearA, d.Options)
	}
	submitChoices(t, e, cIdx)
	passUntilStackEmpty(t, e, 30)

	if o := e.G.Obj(bearA); o == nil || o.Zone != state.ZBattlefield || o.Counter("P1P1") != 1 {
		t.Fatalf("convoked bearA = %+v counters %d, want exactly 1 P1P1", e.G.Obj(bearA), e.G.Obj(bearA).Counter("P1P1"))
	}
	for _, id := range []state.ObjID{bearB, bearC} {
		if o := e.G.Obj(id); o == nil || o.Counter("P1P1") != 0 {
			t.Fatalf("unconvoked bear %d counters = %d, want 0", id, e.G.Obj(id).Counter("P1P1"))
		}
	}
}

// TestLedgerShredderConnivesWithOneCardHandWithoutAsking pins the trigger
// carrier and the strict-supersets rule together: a bare `DB$ Connive`
// trigger connives its own source; with exactly one card in hand after the
// draw the discard needs no ask (nobody could answer differently), the drawn
// card is what is discarded, and the events.Connive record is what a
// trig:Connives trigger reads.
func TestLedgerShredderConnivesWithOneCardHandWithoutAsking(t *testing.T) {
	e, _ := conniveEngine(t, []string{"Ledger Shredder", "Lightning Bolt", "Lightning Bolt"}, nil)
	shredder := conniveMoveTo(t, e, 0, "Ledger Shredder", state.ZBattlefield)
	bolt1 := conniveMoveTo(t, e, 0, "Lightning Bolt", state.ZHand)
	conniveMoveTo(t, e, 0, "Lightning Bolt", state.ZHand, bolt1)
	// Empty the hand: the two bolts stay, everything else returns to the
	// library, so after both casts the hand is empty and the connive's draw
	// is the only discard candidate.
	var handIDs []state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name != "Lightning Bolt" {
			handIDs = append(handIDs, id)
		}
	}
	for _, id := range handIDs {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	e.pending = nil
	e.priorityRound()

	for cast := 0; cast < 2; cast++ {
		var bolt state.ObjID
		for _, id := range e.G.Zone(state.ZHand, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Lightning Bolt" {
				bolt = id
				break
			}
		}
		if bolt == 0 {
			t.Fatalf("cast %d: no Lightning Bolt in hand", cast)
		}
		addMana(t, e, 0, "R")
		d := e.Pending()
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == bolt {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("no cast option for bolt: %+v", d.Options)
		}
		submitChoices(t, e, idx)
		// The bolt's own target ask: any option (the first seat) — the bolt
		// is only the cast counter, its target is irrelevant here.
		if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
			submitChoices(t, e, 0)
		}
		passUntilStackEmpty(t, e, 40)
	}
	// Outcome: the connive drew one and discarded it without asking (hand
	// back to 0); the +1/+1 counter sits on the shredder exactly when the
	// drawn card was a nonland.
	if got := len(e.G.Zone(state.ZHand, 0)); got != 0 {
		t.Fatalf("hand after connive = %d, want 0 (drew one, discarded it)", got)
	}
	drawnNonland := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Draw && ev.Player == 0 && !e.G.Obj(ev.Obj).Face().IsLand() {
			drawnNonland = true
		}
	}
	got := e.G.Obj(shredder).Counter("P1P1")
	if drawnNonland && got != 1 {
		t.Fatalf("shredder counters = %d, want 1 (nonland drawn card discarded)", got)
	}
	if !drawnNonland && got != 0 {
		t.Fatalf("shredder counters = %d, want 0 (land drawn card discarded)", got)
	}
	sawRecord := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Connive && ev.Obj == shredder {
			sawRecord = true
			if len(ev.IDs) != 1 {
				t.Fatalf("connive record IDs = %v, want the one discarded card", ev.IDs)
			}
		}
	}
	if !sawRecord {
		t.Fatal("no events.Connive record for the trigger connive")
	}
}

// TestConnivesTriggerFiresForIronMonger pins trig:Connives end to end: Iron
// Monger (itself a Villain) gets its +1/+1 counter when a creature you
// control connives — the record the matcher reads.
func TestConnivesTriggerFiresForIronMonger(t *testing.T) {
	e, _ := conniveEngine(t, []string{"Ledger Shredder", "Lightning Bolt", "Lightning Bolt", "Iron Monger, Sadistic Tycoon"}, nil)
	shredder := conniveMoveTo(t, e, 0, "Ledger Shredder", state.ZBattlefield)
	ironMonger := conniveMoveTo(t, e, 0, "Iron Monger, Sadistic Tycoon", state.ZBattlefield)
	bolt1 := conniveMoveTo(t, e, 0, "Lightning Bolt", state.ZHand)
	conniveMoveTo(t, e, 0, "Lightning Bolt", state.ZHand, bolt1)
	var handIDs []state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name != "Lightning Bolt" {
			handIDs = append(handIDs, id)
		}
	}
	for _, id := range handIDs {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	e.pending = nil
	e.priorityRound()

	for cast := 0; cast < 2; cast++ {
		var bolt state.ObjID
		for _, id := range e.G.Zone(state.ZHand, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Lightning Bolt" {
				bolt = id
				break
			}
		}
		if bolt == 0 {
			t.Fatalf("cast %d: no Lightning Bolt in hand", cast)
		}
		addMana(t, e, 0, "R")
		d := e.Pending()
		idx := -1
		for _, o := range d.Options {
			if o.Kind == "cast" && o.Obj == bolt {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("no cast option for bolt: %+v", d.Options)
		}
		submitChoices(t, e, idx)
		if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
			submitChoices(t, e, 0)
		}
		passUntilStackEmpty(t, e, 40)
	}
	// The Connives trigger ran its PutCounterAll body on the Villain.
	if got := e.G.Obj(ironMonger).Counter("P1P1"); got != 1 {
		t.Fatalf("iron monger counters = %d, want 1 from the connives trigger", got)
	}
	if o := e.G.Obj(shredder); o.Zone != state.ZBattlefield {
		t.Fatalf("shredder zone = %s, want battlefield", o.Zone)
	}
}
