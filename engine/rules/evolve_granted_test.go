package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A static-granted Evolve (CR 702.100a, 613.1f): Tyranid Prime's "Other
// creatures you control have evolve" and Propagator Drone's "Creature tokens
// you control have evolve". The printed expansion (kw_evolve.go) only runs at
// face parse, so a layer-6 grant used to land in the derived keywords with no
// trigger behind it.

// hasDerivedEvolve reports whether the object's derived keywords carry Evolve.
func hasDerivedEvolve(e *Engine, id state.ObjID) bool {
	return slices.ContainsFunc(e.Derived(id).Keywords, func(k string) bool {
		return strings.EqualFold(cards.KeywordHead(k), "Evolve")
	})
}

func TestTyranidPrimeGrantsEvolveThatFires(t *testing.T) {
	t.Parallel()
	const prime, bear, big = "Tyranid Prime", "Grizzly Bears", "Craw Wurm"
	e, cfg, _ := evolvedGame(t, 77, prime, bear, big)
	enterFromLibrary(t, e, 0, prime)
	br := enterFromLibrary(t, e, 0, bear)
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if !hasDerivedEvolve(e, br) {
		t.Fatalf("precondition: the grant did not land; derived=%v", e.Derived(br).Keywords)
	}
	if got := e.G.Obj(br).Counter("P1P1"); got != 0 {
		t.Fatalf("precondition: Bears start with %d counters, want 0", got)
	}
	bg := enterFromLibrary(t, e, 0, big)
	if e.Power(bg) <= e.Power(br) && e.Toughness(bg) <= e.Toughness(br) {
		t.Fatalf("precondition: %s must be strictly bigger", big)
	}
	e.priorityRound()
	answerTriggerOrders(t, e)
	passUntilStackEmpty(t, e, 40)
	if got := e.G.Obj(br).Counter("P1P1"); got != 1 {
		t.Fatalf("granted-evolve Bears has %d +1/+1 counters after a bigger creature entered, want 1", got)
	}
	if n := evolvedMarkerCount(e, br); n != 1 {
		t.Fatalf("events.Evolved = %d, want 1", n)
	}
	replayCheck(t, e, cfg)
}

func TestGrantedEvolveRechecksItsConditionOnResolution(t *testing.T) {
	t.Parallel()
	const prime, elves, bears = "Tyranid Prime", "Llanowar Elves", "Grizzly Bears"
	e, cfg, _ := evolvedGame(t, 78, prime, elves, bears, bears)
	enterFromLibrary(t, e, 0, prime)
	el := enterFromLibrary(t, e, 0, elves)
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if !hasDerivedEvolve(e, el) {
		t.Fatalf("precondition: the grant did not land; derived=%v", e.Derived(el).Keywords)
	}
	if p, tt := e.Power(el), e.Toughness(el); p != 1 || tt != 1 {
		t.Fatalf("precondition: Elves are %d/%d, want 1/1", p, tt)
	}
	b1 := enterFromLibrary(t, e, 0, bears)
	b2 := enterFromLibrary(t, e, 0, bears)
	for _, b := range []state.ObjID{b1, b2} {
		if p, tt := e.Power(b), e.Toughness(b); p != 2 || tt != 2 {
			t.Fatalf("precondition: entrant is %d/%d, want 2/2", p, tt)
		}
	}
	if got := len(e.pendingTriggers); got < 2 {
		t.Fatalf("precondition: %d pending triggers, want at least 2 evolve triggers", got)
	}
	e.priorityRound()
	answerTriggerOrders(t, e)
	passUntilStackEmpty(t, e, 40)
	if got := e.G.Obj(el).Counter("P1P1"); got != 1 {
		t.Fatalf("Elves +1/+1 counters = %d after two 2/2s entered together, want 1 (the second evolve's recheck fails)", got)
	}
	if n := evolvedMarkerCount(e, el); n != 1 {
		t.Fatalf("events.Evolved = %d, want 1", n)
	}
	replayCheck(t, e, cfg)
}

func TestPrintedAndGrantedEvolveEachTrigger(t *testing.T) {
	t.Parallel()
	const prime, exp, big = "Tyranid Prime", "Experiment One", "Craw Wurm"
	e, cfg, _ := evolvedGame(t, 79, prime, exp, big, big)
	enterFromLibrary(t, e, 0, prime)
	src := enterFromLibrary(t, e, 0, exp)
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	if !hasDerivedEvolve(e, src) {
		t.Fatalf("precondition: the grant did not land; derived=%v", e.Derived(src).Keywords)
	}
	if got := e.G.Obj(src).Counter("P1P1"); got != 0 {
		t.Fatalf("precondition: Experiment One starts with %d counters, want 0", got)
	}
	e.priorityRound()
	enterFromLibrary(t, e, 0, big)
	enterFromLibrary(t, e, 0, big)
	e.priorityRound()
	answerTriggerOrders(t, e)
	passUntilStackEmpty(t, e, 40)
	if got := e.G.Obj(src).Counter("P1P1"); got != 4 {
		t.Fatalf("Experiment One +1/+1 counters = %d, want 4 (printed + granted evolve per entrant, CR 113.2c)", got)
	}
	if n := evolvedMarkerCount(e, src); n != 4 {
		t.Fatalf("events.Evolved = %d, want 4", n)
	}
	replayCheck(t, e, cfg)
}

func TestPropagatorDroneGrantsEvolveToTokens(t *testing.T) {
	t.Parallel()
	const drone, big = "Propagator Drone", "Craw Wurm"
	e, cfg, _ := conditionEvolveGame(t, 80, drone, big)
	enterFromLibrary(t, e, 0, drone)
	e.priorityRound()
	passUntilStackEmpty(t, e, 40)
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "c_0_1_eldrazi_spawn_sac"})
	var tok state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken {
			tok = id
		}
	}
	if tok == 0 {
		t.Fatal("precondition: no Eldrazi Spawn token was created")
	}
	if !hasDerivedEvolve(e, tok) {
		t.Fatalf("precondition: the token has no granted evolve; derived=%v", e.Derived(tok).Keywords)
	}
	if got := e.G.Obj(tok).Counter("P1P1"); got != 0 {
		t.Fatalf("precondition: token starts with %d counters, want 0", got)
	}
	bg := enterFromLibrary(t, e, 0, big)
	if e.Power(bg) <= e.Power(tok) && e.Toughness(bg) <= e.Toughness(tok) {
		t.Fatalf("precondition: %s must be strictly bigger than the token", big)
	}
	e.priorityRound()
	answerTriggerOrders(t, e)
	passUntilStackEmpty(t, e, 40)
	if got := e.G.Obj(tok).Counter("P1P1"); got != 1 {
		t.Fatalf("token +1/+1 counters = %d after a bigger creature entered, want 1", got)
	}
	if n := evolvedMarkerCount(e, tok); n != 1 {
		t.Fatalf("token events.Evolved = %d, want 1", n)
	}
	replayCheck(t, e, cfg)
}
