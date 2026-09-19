package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The cast-provenance gate family (task castprov2), pinned on real corpus
// cards in both directions:
//
//   - Amped Raptor's `ConditionDefined$ TriggeredCard | ConditionPresent$
//     Card.wasCastFromYourHandByYou` gate (effects/conditions.go): the
//     exile-until DigUntil chain runs ONLY when the entering permanent was
//     cast from its controller's hand — before the gate's defined group was
//     enumerable the sub ran unconditionally, exiling on every entry.
//   - Emperor Apatzec Intli IV's `ConditionDefined$ TriggeredCard |
//     ConditionPresent$ Creature.powerGE4` gate, one of the 33 raw corpus
//     lines the same change makes real: a creature entering at power 4 or
//     more gains its perpetual haste, a smaller one does not.
//
// The un-cast entries are raw hand→battlefield MoveZone emits — the fixture
// shape rules/etb_counter_gate_test.go's placeFromHand uses — so the gate
// must deny (or grant) on its own provenance, never on zone shape.

// castProvEngine is handEngine's shape with the Config returned, so the
// replay-identity check can run over the same game. The genesis is NOT
// driven: like handEngine, the engine sits at turn 1 main1 with seat 0
// priority and an exact hand.
func castProvEngine(t *testing.T, reg *cards.Registry, hand ...*cards.Card) (*Engine, Config) {
	t.Helper()
	cfg := seatZeroStart(Config{Seed: 11, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)},
		Tokens: reg.Tokens})
	e := New(cfg)
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
	}
	var ids []state.ObjID
	for _, c := range hand {
		o := e.G.AddObject(c, 0)
		o.Zone = state.ZHand
		ids = append(ids, o.ID)
	}
	e.G.SetZone(state.ZHand, 0, ids)
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	return e, cfg
}

// putTopOfLibrary injects an object at the very top of p's library (the
// fixture decks are all Mountains, so the injected card is the scan's first
// non-match or match by position).
func putTopOfLibrary(t *testing.T, e *Engine, c *cards.Card, p state.PlayerID) state.ObjID {
	t.Helper()
	o := e.G.AddObject(c, p)
	o.Zone = state.ZLibrary
	e.G.SetZone(state.ZLibrary, p, append([]state.ObjID{o.ID}, e.G.Zone(state.ZLibrary, p)...))
	return o.ID
}

// drainProvTriggers drains queued triggers and the stack, stopping at the
// first non-priority decision (the caller asserts its shape and answers it).
func drainProvTriggers(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 50; i++ {
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
			return
		}
		if len(e.G.Stack) > 0 {
			e.resolveTop()
			continue
		}
		return
	}
	t.Fatalf("trigger drain did not settle: %d pending, %d on the stack", len(e.pendingTriggers), len(e.G.Stack))
}

func TestAmpedRaptorCastFromHandRunsTheExileUntil(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	raptor := corpusAlternativeCard(t, "Amped Raptor")
	bear := corpusAlternativeCard(t, "Grizzly Bears")
	e, _ := castProvEngine(t, reg, raptor)
	// A known nonland at the top of seat 0's library: the DigUntil turns the
	// Mountain underneath it over first, so the found card is not the scan's
	// first card.
	bearID := putTopOfLibrary(t, e, bear, 0)
	raptorID := e.G.Zone(state.ZHand, 0)[0]
	// Raw ManaAdd emits (no priority re-ask): beginCast pays from the pool
	// directly, so the committed cast needs no live priority decision.
	for _, r := range "CR" {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: string(r), Amount: 1})
	}
	castMode(t, e, raptorID, "")
	resolveOffStack(t, e, raptorID)
	drainProvTriggers(t, e)
	// THE pin: the gate held, so the exile-until ran.
	if n := e.G.Players[0].Counter("ENERGY"); n != 2 {
		t.Fatalf("cast-from-hand Raptor energy = %d, want 2", n)
	}
	exile := e.G.Zone(state.ZExile, 0)
	found := false
	for _, id := range exile {
		if id == bearID {
			found = true
		}
	}
	if !found {
		t.Fatalf("the exile-until did not move the found nonland to exile: %v", exile)
	}
	// The follow-up energy-cast offer (DB$ Play, Optional$) is the ask the
	// resolution suspended on — and it offers ONLY the found card: the
	// triggering permanent itself (the trigger's captured Remembered entry)
	// is excluded from a remembered Play population.
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || d.ResumeKind != "play" {
		t.Fatalf("expected the play ask after the exile-until, got %+v", d)
	}
	if len(d.Options) != 1 || d.Options[0].Obj != bearID {
		t.Fatalf("the play ask offers %+v, want exactly the exiled found card", d.Options)
	}
	// Optional$ Play: the empty answer is the decline. The replay-identity
	// check does not apply to this fixture: castProvEngine's hand overwrite
	// is deliberately event-less (the handEngine shape), so a log-only replay
	// of cfg diverges on the opening hand by construction.
	submitChoices(t, e) // Optional$ Play: the empty answer is the decline
	passUntilStackEmpty(t, e, 60)
}

func TestAmpedRaptorCheatedEntryGrantsEnergyWithoutTheExile(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	raptor := corpusAlternativeCard(t, "Amped Raptor")
	bear := corpusAlternativeCard(t, "Grizzly Bears")
	e, _ := castProvEngine(t, reg, raptor)
	bearID := putTopOfLibrary(t, e, bear, 0)
	raptorID := e.G.Zone(state.ZHand, 0)[0]
	// THE un-cast entry: a raw hand→battlefield MoveZone, no PutOnStack.
	e.emit(events.Event{Kind: events.MoveZone, Obj: raptorID, From: state.ZHand, To: state.ZBattlefield})
	drainProvTriggers(t, e)
	// The {E}{E} grant is ungated; the DigUntil chain is what the gate stops.
	if n := e.G.Players[0].Counter("ENERGY"); n != 2 {
		t.Fatalf("cheated-entry Raptor energy = %d, want 2", n)
	}
	// The exile zone legitimately holds the resolved trigger's own wrapper
	// (CR 608.2m parking); the gate's failure is the LIBRARY scan having
	// moved nothing — the injected nonland must still be on top.
	lib := e.G.Zone(state.ZLibrary, 0)
	if len(lib) == 0 || lib[0] != bearID {
		t.Fatalf("cheated entry moved the library (the gate did not hold): top=%v", lib)
	}
	if d := e.Pending(); d != nil && d.ResumeKind == "play" {
		t.Fatalf("cheated entry reached the play ask: %+v", d)
	}
	_ = bearID
}

func TestEmperorApatzecTriggeredCardGateGrantsHasteOnlyAtFourPower(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	emperor := corpusAlternativeCard(t, "Emperor Apatzec Intli IV")
	dreadmaw := corpusAlternativeCard(t, "Colossal Dreadmaw")
	bears := corpusAlternativeCard(t, "Grizzly Bears")
	e, _ := castProvEngine(t, reg)
	emp := e.G.AddObject(emperor, 0)
	emp.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{emp.ID})

	// A 4+-power creature entering: the powerGE4 gate holds — perpetual haste.
	big := e.G.AddObject(dreadmaw, 0)
	big.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{big.ID})
	e.emit(events.Event{Kind: events.MoveZone, Obj: big.ID, From: state.ZHand, To: state.ZBattlefield})
	drainProvTriggers(t, e)
	if !e.HasKeyword(big.ID, "Haste") {
		t.Fatalf("the 6/6 entering creature did not gain the gated perpetual haste")
	}

	// A 2-power creature entering: the gate denies — no haste.
	small := e.G.AddObject(bears, 0)
	small.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, []state.ObjID{small.ID})
	e.emit(events.Event{Kind: events.MoveZone, Obj: small.ID, From: state.ZHand, To: state.ZBattlefield})
	drainProvTriggers(t, e)
	if e.HasKeyword(small.ID, "Haste") {
		t.Fatalf("the 2/2 entering creature gained haste although the powerGE4 gate denies it")
	}
}
