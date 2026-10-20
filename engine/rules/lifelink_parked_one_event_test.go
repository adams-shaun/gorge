package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// This file pins CR 702.15e on the PARKED damage path: when a CR 616.1
// replacement order choice holds a hit (Battletide Alchemist's optional
// prevention, declined here), that hit's lifelink is paid only when the
// answer lands, and it must still merge with every other hit the same source
// dealt in the same simultaneous batch. The pre-fix direct emit paid the
// parked hit separately, so one source's damage produced two life-gain
// events.

// declineParkedDamageReplacements answers every parked damage replacement
// with its decline option, resolves trigger-order asks in the offered order,
// and passes priority. It returns how many damage replacements it declined.
func declineParkedDamageReplacements(t *testing.T, e *Engine) int {
	t.Helper()
	asks := 0
	for i := 0; i < 64; i++ {
		d := e.Pending()
		switch {
		case d == nil:
			return asks
		case d.Kind == decision.KReplacement:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "skip_replacement" {
					idx = o.Index
					break
				}
			}
			if idx < 0 {
				t.Fatalf("replacement ask offers no decline option: %+v", d.Options)
			}
			asks++
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("decline damage replacement: %v", err)
			}
		case d.Kind == decision.KTriggerOrder:
			choices := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				choices = append(choices, o.Index)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
				t.Fatalf("submit trigger order: %v", err)
			}
		case d.Kind == decision.KPriority:
			submitPass(t, e)
		default:
			return asks
		}
	}
	t.Fatal("parked damage replacements did not settle within the pass budget")
	return asks
}

// TestLifelinkTrampleOverWithParkedHitIsOneLifeGainEvent is the combat arm:
// Fleetfoot Dancer's blocker damage lands first and its trample damage to the
// player is parked by Battletide Alchemist, then declined. The two hits are
// still ONE source dealing damage to two recipients at once, so the
// controller gains 4 as ONE event and Ajani's Pridemate triggers once.
func TestLifelinkTrampleOverWithParkedHitIsOneLifeGainEvent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg,
		[]string{"Fleetfoot Dancer", "Battletide Alchemist", "Ajani's Pridemate"}, nil,
		[]string{"Grizzly Bears"}, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	dancer := findBattlefield(t, e, 0, "Fleetfoot Dancer", 0)
	pm := findBattlefield(t, e, 0, "Ajani's Pridemate", 0)
	bears := findBattlefield(t, e, 1, "Grizzly Bears", 0)

	e.askAttackers()
	submitAttackers(t, e, dancer)
	start := len(e.L.Events)
	submitBlockers(t, e, bears)
	if asks := declineParkedDamageReplacements(t, e); asks != 1 {
		t.Fatalf("damage replacement asks = %d, want 1 (the trample hit parked)", asks)
	}
	drainLifeGainTriggers(t, e)

	if got := lifelinkGains(e, 0, start); len(got) != 1 || got[0] != 4 {
		t.Fatalf("life-gain events for seat 0 = %v, want [4] (CR 702.15e: one source, one event)", got)
	}
	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("defender life = %d, want 18 (2 trampled over)", got)
	}
	if got := e.G.Players[0].Life; got != 24 {
		t.Fatalf("Dancer's controller life = %d, want 24 (20 + 4 lifelink)", got)
	}
	if n := countCounterChanges(e, pm, "P1P1", 1); n != 1 {
		t.Fatalf("Ajani's Pridemate got %d +1/+1 counter events, want 1 (one life-gain event)", n)
	}
	replayCheck(t, e, cfg)
}

// TestLifelinkEachOpponentWithParkedHitIsOneLifeGainEvent is the non-combat
// arm: Merciless Enforcers deals 1 to each of two opponents in one ability
// resolution, and each hit is parked by Battletide Alchemist (seat 1, whose
// replacement covers damage to any player) and declined. The controller
// still gains 2 as ONE event and Ajani's Pridemate triggers once.
func TestLifelinkEachOpponentWithParkedHitIsOneLifeGainEvent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := lifelinkSeatBoard(t, reg,
		[]string{"Merciless Enforcers", "Ajani's Pridemate"},
		[]string{"Battletide Alchemist"}, nil)
	enf := findBattlefield(t, e, 0, "Merciless Enforcers", 0)
	pm := findBattlefield(t, e, 0, "Ajani's Pridemate", 0)
	addMana(t, e, 0, "BBBB")
	start := len(e.L.Events)
	opt := abilityOption(t, e, enf, 0)
	submitChoices(t, e, opt.Index)
	if asks := declineParkedDamageReplacements(t, e); asks != 2 {
		t.Fatalf("damage replacement asks = %d, want 2 (one per damaged opponent)", asks)
	}
	drainLifeGainTriggers(t, e)

	if e.G.Players[1].Life != 19 || e.G.Players[2].Life != 19 {
		t.Fatalf("opponent lives = %d/%d, want 19/19", e.G.Players[1].Life, e.G.Players[2].Life)
	}
	if got := e.G.Players[0].Life; got != 22 {
		t.Fatalf("Enforcers' controller life = %d, want 22 (20 + 1 + 1 lifelink)", got)
	}
	if got := lifelinkGains(e, 0, start); len(got) != 1 || got[0] != 2 {
		t.Fatalf("life-gain events for seat 0 = %v, want [2] (CR 702.15e: one source, one event)", got)
	}
	if n := countCounterChanges(e, pm, "P1P1", 1); n != 1 {
		t.Fatalf("Ajani's Pridemate got %d +1/+1 counter events, want 1 (one life-gain event)", n)
	}
	replayCheck(t, e, cfg)
}
