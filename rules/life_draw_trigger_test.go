package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func corpusCard(t *testing.T, name string) *cards.Card {
	t.Helper()
	c, ok := testutil.CorpusRegistry(t).Lookup(name)
	if !ok {
		t.Fatalf("corpus fixture %q is missing", name)
	}
	return c
}

func TestSheoldredDrawnTriggerUsesDrawEventPlayer(t *testing.T) {
	e := layerEngine(t)
	sheoldred := corpusCard(t, "Sheoldred, the Apocalypse")
	source := onBoardCard(t, e, 0, sheoldred)
	if got := e.G.Obj(source).Face().Triggers[0].Mode; got != "Drawn" {
		t.Fatalf("Sheoldred trigger mode = %q, want Drawn", got)
	}

	drawn := e.G.Zone(state.ZLibrary, 0)[0]
	e.emit(events.Event{Kind: events.Draw, Player: 0, Obj: drawn, From: state.ZLibrary, To: state.ZHand, Secret: true})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("Sheoldred draw trigger stack = %v, want one trigger", e.G.Stack)
	}
	e.resolveTop()
	if got := e.G.Players[0].Life; got != 22 {
		t.Fatalf("life after Sheoldred's draw trigger = %d, want 22", got)
	}
}

func TestOrcishBowmastersDrawnSkipsFirstDrawStepCard(t *testing.T) {
	e := layerEngine(t)
	bowmasters := corpusCard(t, "Orcish Bowmasters")
	source := onBoardCard(t, e, 0, bowmasters)
	if got := e.G.Obj(source).Face().Triggers[1].Params["FirstCardInDrawStep"]; got != "False" {
		t.Fatalf("Bowmasters FirstCardInDrawStep = %q, want False", got)
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDraw})
	first := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.Draw, Player: 1, Obj: first, From: state.ZLibrary, To: state.ZHand, Secret: true})
	if len(e.pendingTriggers) != 0 {
		t.Fatalf("first draw step card queued %#v, want no Bowmasters trigger", e.pendingTriggers)
	}
	second := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.Draw, Player: 1, Obj: second, From: state.ZLibrary, To: state.ZHand, Secret: true})
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != source {
		t.Fatalf("second draw step card queued %#v, want Bowmasters trigger", e.pendingTriggers)
	}
}

func TestLifeLostAllObNixilisQueuesOnceForSimultaneousOpponents(t *testing.T) {
	// Ob Nixilis's real "one or more opponents each lose exactly 1 life"
	// needs a three-seat batch: two serialized events are one simultaneous
	// event, hence exactly one trigger.
	e := New(seatZeroStart(Config{Seed: 1, Names: []string{"a", "b", "c"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}}))
	ob := corpusCard(t, "Ob Nixilis, Captive Kingpin")
	source := onBoardCard(t, e, 0, ob)
	if got := e.G.Obj(source).Face().Triggers[0].Mode; got != "LifeLostAll" {
		t.Fatalf("Ob Nixilis trigger mode = %q, want LifeLostAll", got)
	}
	e.BeginLifeLossBatch()
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -1})
	e.emit(events.Event{Kind: events.LifeChange, Player: 2, Amount: -1})
	e.EndLifeLossBatch()
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != source {
		t.Fatalf("simultaneous opponent losses queued %#v, want one Ob Nixilis trigger", e.pendingTriggers)
	}
}

func TestValgavothLifeLostFirstTimeGate(t *testing.T) {
	e := layerEngine(t)
	valgavoth := corpusCard(t, "Valgavoth, Harrower of Souls")
	source := onBoardCard(t, e, 0, valgavoth)
	trig := e.G.Obj(source).Face().Triggers[0]
	if trig.Mode != "LifeLost" || trig.Params["FirstTime"] != "True" {
		t.Fatalf("Valgavoth trigger fixture changed: %+v", trig)
	}

	// ValidPlayer$ Opponent.Active is the unmodified corpus condition: the
	// opponent must be the active player, and FirstTime permits only its first
	// life-loss event this turn.
	e.G.Active = 1
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -1})
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != source {
		t.Fatalf("first active-opponent loss queued %#v, want Valgavoth", e.pendingTriggers)
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -1})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("second active-opponent loss queued %#v, want no second Valgavoth", e.pendingTriggers)
	}
}

func TestArchfiendCantGainLifePreventsOpponentOnly(t *testing.T) {
	e := layerEngine(t)
	archfiend := corpusCard(t, "Archfiend of Despair")
	source := onBoardCard(t, e, 0, archfiend)
	if got := e.G.Obj(source).Face().Statics[0].Mode; got != "CantGainLife" {
		t.Fatalf("Archfiend static mode = %q, want CantGainLife", got)
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: 3})
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 3})
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("opponent life = %d, want prevented gain at 20", got)
	}
	if got := e.G.Players[0].Life; got != 23 {
		t.Fatalf("Archfiend controller life = %d, want 23", got)
	}
}

func TestSulfuricVortexGainLifeReplacementPreventsGain(t *testing.T) {
	e := layerEngine(t)
	vortex := corpusCard(t, "Sulfuric Vortex")
	source := onBoardCard(t, e, 0, vortex)
	if got := e.G.Obj(source).Face().Repls[0].Event; got != "GainLife" {
		t.Fatalf("Sulfuric Vortex replacement event = %q, want GainLife", got)
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: 4})
	if got := e.G.Players[1].Life; got != 20 {
		t.Fatalf("life after Sulfuric Vortex replacement = %d, want 20", got)
	}
}

func TestBloodletterLifeReducedDoublesOpponentLossOnYourTurn(t *testing.T) {
	e := layerEngine(t)
	bloodletter := corpusCard(t, "Bloodletter of Aclazotz")
	source := onBoardCard(t, e, 0, bloodletter)
	if got := e.G.Obj(source).Face().Repls[0].Event; got != "LifeReduced" {
		t.Fatalf("Bloodletter replacement event = %q, want LifeReduced", got)
	}
	e.G.Active = 0
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -3})
	if got := e.G.Players[1].Life; got != 14 {
		t.Fatalf("life after Bloodletter loss replacement = %d, want 14", got)
	}
}

func TestAlhammarretsArchiveDoublesGain(t *testing.T) {
	e := layerEngine(t)
	archive := corpusCard(t, "Alhammarret's Archive")
	onBoardCard(t, e, 0, archive)
	e.emit(events.Event{Kind: events.LifeChange, Player: 0, Amount: 3})
	if got := e.G.Players[0].Life; got != 26 {
		t.Fatalf("life after Archive +3 gain = %d, want 26", got)
	}
}

func TestTwoBloodlettersEachApplyOnce(t *testing.T) {
	e := layerEngine(t)
	bloodletter := corpusCard(t, "Bloodletter of Aclazotz")
	onBoardCard(t, e, 0, bloodletter)
	onBoardCard(t, e, 0, bloodletter)
	e.G.Active = 0
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -3})
	if got := e.G.Players[1].Life; got != 8 {
		t.Fatalf("life after two Bloodletters -3 = %d, want 8", got)
	}
}
