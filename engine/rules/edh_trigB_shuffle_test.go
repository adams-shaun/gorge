package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHTriggerModesBWidespreadOpponentShuffle plays Widespread Panic's
// "causes ITS CONTROLLER to shuffle" clause (ShuffleBySelfControlled$ True)
// against the cross-controller case: seat 0 sacrifices Boggart Forager to make
// the OPPONENT shuffle their library. The effect is seat 0's, the library
// seat 1's, so the printed text does not fire Panic; the shuffle itself must
// still have happened (the precondition the absence of a trigger depends on).
func TestEDHTriggerModesBWidespreadOpponentShuffle(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	panicCard := searchCorpusCard(t, reg, "Widespread Panic")
	if unsupported := reg.Unsupported(panicCard, effects.Supported()); len(unsupported) != 0 {
		t.Fatalf("precondition: Widespread Panic unsupported: %v", unsupported)
	}
	e, _, battle, _ := effectGrantEngine(t, 9406,
		[]string{"Widespread Panic", "Boggart Forager"}, nil)
	forager := battle[1]
	oppHand := len(e.G.Zone(state.ZHand, 1))
	shufflesBefore := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Shuffle && ev.Text == events.ShuffleFromEffectMarker {
			shufflesBefore++
		}
	}

	addMana(t, e, 0, "R")
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision pending, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if (o.Kind == "activate" || o.Kind == "ability") && o.Obj == forager {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("the forager's ability is not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	// Answer the cost choice and the target ask: the opponent is the target.
	for i := 0; i < 6; i++ {
		d = e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		pick := d.Options[0].Index
		for _, o := range d.Options {
			if o.Player == 1 && o.Obj == 0 {
				pick = o.Index
			}
		}
		submitChoices(t, e, pick)
	}
	var asked *decision.Decision
	trigBDrive(t, e, 60, func(d *decision.Decision) bool {
		if d.Kind == decision.KChoose || d.Kind == decision.KTarget {
			asked = d
			return true
		}
		return len(e.G.Stack) == 0 && d.Kind == decision.KPriority
	}, nil)
	shuffles := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Shuffle && ev.Text == events.ShuffleFromEffectMarker && ev.Player == 1 {
			shuffles++
		}
	}
	if shuffles <= shufflesBefore {
		t.Fatal("precondition: the forager did not shuffle the opponent's library")
	}
	if asked != nil {
		t.Fatalf("Widespread Panic fired on an opponent-targeted shuffle: %+v", asked)
	}
	if got := len(e.G.Zone(state.ZHand, 1)); got != oppHand {
		t.Fatalf("opponent's hand changed %d -> %d", oppHand, got)
	}
}
