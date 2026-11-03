package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A Witness-Protected Gnarlid Colony loses "Each creature you control with a
// +1/+1 counter on it has trample". Both effects apply in layer 6, but the
// grant's existence depends on the ability removal (CR 613.8a), so the
// removal applies first whatever the timestamps and the grant never applies:
// neither the Colony nor another countered creature has trample (census
// pairs 4109, 6841).
func TestWitnessProtectedColonyGrantsNoTrample(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Gnarlid Colony"), lookup(t, reg, "Grizzly Bears"),
		lookup(t, reg, "Witness Protection")}, nil)
	colony := moveCorpusCard(t, e, "Gnarlid Colony", 0, state.ZBattlefield)
	bears := moveCorpusCard(t, e, "Grizzly Bears", 0, state.ZBattlefield)
	e.emit(events.Event{Kind: events.CounterChange, Obj: colony, Counter: "P1P1", Amount: 2})
	e.emit(events.Event{Kind: events.CounterChange, Obj: bears, Counter: "P1P1", Amount: 1})
	lossNextTurn(e)
	if !e.HasKeyword(colony, "Trample") || !e.HasKeyword(bears, "Trample") {
		t.Fatalf("precondition: Colony trample=%v Bears trample=%v, want both", e.HasKeyword(colony, "Trample"), e.HasKeyword(bears, "Trample"))
	}
	lossAttach(t, e, "Witness Protection", 0, colony)
	if e.HasKeyword(colony, "Trample") {
		t.Errorf("the Witness-Protected Colony still has trample: %v", e.Keywords(colony))
	}
	if e.HasKeyword(bears, "Trample") {
		t.Errorf("Grizzly Bears still has trample from a Colony with no abilities: %v", e.Keywords(bears))
	}
	replayCheck(t, e, cfg)
}

// A permanent that lost its abilities has no replacement effect (CR 613.1f): Angel
// of Vitality under Witness Protection adds nothing to a life gain, on the same scan
// that adds its 1 without the Aura.
func TestLifeReplacementLostWithTheAbilities(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Angel of Vitality"), lookup(t, reg, "Witness Protection")}, nil)
	angel := moveCorpusCard(t, e, "Angel of Vitality", 0, state.ZBattlefield)
	gain := func() int32 {
		before := e.G.Players[0].Life
		e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 2})
		return e.G.Players[0].Life - before
	}
	if got := gain(); got != 3 {
		t.Fatalf("with the Angel's ability a gain of 2 came to %d, want 3", got)
	}
	lossAttach(t, e, "Witness Protection", 0, angel)
	if got := gain(); got != 2 {
		t.Fatalf("with the Angel's abilities gone a gain of 2 came to %d, want 2", got)
	}
	replayCheck(t, e, cfg)
}
