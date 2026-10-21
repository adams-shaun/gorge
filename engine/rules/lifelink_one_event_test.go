package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// This file pins CR 702.15e with CR 119.9: damage one lifelink SOURCE deals
// to several recipients at the same time is ONE life-gain event of the total,
// so "whenever you gain life" (Ajani's Pridemate) triggers once. Several
// lifelink sources dealing damage at the same time are separate events, one
// per source (the guard below). Forge corroborates (GameAction.java's
// per-source lifelink sum, which cites 702.15e).

// lifelinkGains returns the positive LifeChange events for player p logged
// at or after index from, in log order.
func lifelinkGains(e *Engine, p state.PlayerID, from int) []int32 {
	var out []int32
	for _, ev := range e.L.Events[from:] {
		if ev.Kind == events.LifeChange && ev.Player == p && ev.Amount > 0 {
			out = append(out, ev.Amount)
		}
	}
	return out
}

// drainLifeGainTriggers answers every trigger_order ask in the offered order
// and passes priority while the stack is non-empty or the game is still in
// the combat damage step, so the queued "whenever you gain life" triggers
// resolve whatever their count.
func drainLifeGainTriggers(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 64; i++ {
		d := e.Pending()
		switch {
		case d == nil:
			return
		case d.Kind == decision.KTriggerOrder:
			choices := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				choices = append(choices, o.Index)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
				t.Fatalf("submit trigger order: %v", err)
			}
		case d.Kind == decision.KPriority && (len(e.G.Stack) > 0 || e.G.Step == state.StepCombatDamage):
			submitPass(t, e)
		default:
			return
		}
	}
	t.Fatal("life-gain triggers did not settle within the pass budget")
}

// lifelinkSeatBoard is linkBoard for any number of seats: every named corpus
// card is seeded into that seat's deck and moved onto the battlefield with a
// logged MoveZone, creatures are not summoning sick, and the clock is parked
// at turn 2 seat 0 Main1.
func lifelinkSeatBoard(t *testing.T, reg *cards.Registry, seats ...[]string) (*Engine, Config) {
	t.Helper()
	cfg := Config{Seed: 47, Tokens: reg.Tokens}
	named := make([][]*cards.Card, len(seats))
	for p, ns := range seats {
		for _, n := range ns {
			named[p] = append(named[p], mustCorpusCard(t, reg, n))
		}
		cfg.Names = append(cfg.Names, string(rune('a'+p)))
		cfg.Decks = append(cfg.Decks, append(append([]*cards.Card{}, named[p]...), mountainDeck(t, 40-len(named[p]))...))
	}
	cfg = seatZeroStart(cfg)
	e := New(cfg)
	if e.choosing == chooseOpening {
		e.choosing = chooseNone
	}
	for p := range seats {
		for i := range e.G.Objs {
			o := &e.G.Objs[i]
			if o.Owner != state.PlayerID(p) {
				continue
			}
			for _, c := range named[p] {
				if o.Card == c {
					e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: o.Zone, To: state.ZBattlefield})
					break
				}
			}
		}
	}
	// Only seat 0's creatures need to be attack/ability-ready; leaving every
	// other seat's summoning sickness alone keeps the board reproducible from
	// the logged MoveZone events (the same rule combatTriggerBoard follows).
	for i := range e.G.Objs {
		if o := &e.G.Objs[i]; o.Zone == state.ZBattlefield && o.Owner == 0 &&
			o.Face() != nil && o.Face().IsCreature() {
			o.SummonSick = false
		}
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	return e, cfg
}

// TestLifelinkTrampleOverIsOneLifeGainEvent is the audit repro (S08a item 1):
// Fleetfoot Dancer (4/4 trample, lifelink) is blocked by Grizzly Bears and
// assigns 2 to the Bears and 2 tramples over to the player. That is ONE
// source dealing damage to two recipients at once, so its controller gains 4
// life as ONE event and Ajani's Pridemate triggers once.
func TestLifelinkTrampleOverIsOneLifeGainEvent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg, []string{"Fleetfoot Dancer", "Ajani's Pridemate"}, nil,
		[]string{"Grizzly Bears"}, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	dancer := findBattlefield(t, e, 0, "Fleetfoot Dancer", 0)
	pm := findBattlefield(t, e, 0, "Ajani's Pridemate", 0)
	bears := findBattlefield(t, e, 1, "Grizzly Bears", 0)

	e.askAttackers()
	submitAttackers(t, e, dancer)
	start := len(e.L.Events)
	// One blocker: the lethal 2 goes to the Bears and the rest tramples over
	// without a division ask; the damage pass runs inside this submission.
	submitBlockers(t, e, bears)
	if got := lifelinkGains(e, 0, start); len(got) != 1 || got[0] != 4 {
		t.Fatalf("life-gain events for seat 0 = %v, want [4] (CR 702.15e: one source, one event)", got)
	}
	drainLifeGainTriggers(t, e)

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

// TestTwoLifelinkSourcesAreTwoLifeGainEvents is the guard: two different
// lifelink creatures dealing combat damage at the same time are SEPARATE
// life-gain events (CR 702.15e), so Pridemate triggers twice. Fleetfoot
// Dancer (4) and Moldering Karok (3) both attack unblocked.
func TestTwoLifelinkSourcesAreTwoLifeGainEvents(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg,
		[]string{"Fleetfoot Dancer", "Moldering Karok", "Ajani's Pridemate"}, nil, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	dancer := findBattlefield(t, e, 0, "Fleetfoot Dancer", 0)
	karok := findBattlefield(t, e, 0, "Moldering Karok", 0)
	pm := findBattlefield(t, e, 0, "Ajani's Pridemate", 0)

	e.askAttackers()
	start := len(e.L.Events)
	submitAttackers(t, e, dancer, karok)
	drainCombatPriority(t, e)
	drainLifeGainTriggers(t, e)

	if got := e.G.Players[0].Life; got != 27 {
		t.Fatalf("attacker's life = %d, want 27 (20 + 4 + 3)", got)
	}
	got := lifelinkGains(e, 0, start)
	if len(got) != 2 || got[0]+got[1] != 7 || (got[0] != 4 && got[0] != 3) {
		t.Fatalf("life-gain events for seat 0 = %v, want one +4 and one +3 (one per source)", got)
	}
	if o := e.G.Obj(pm); o.Counter("P1P1") != 2 {
		t.Fatalf("Ajani's Pridemate has %d +1/+1 counters, want 2 (two sources, two events)", o.Counter("P1P1"))
	}
	replayCheck(t, e, cfg)
}

// TestLifelinkDamageToEachOpponentIsOneLifeGainEvent is the non-combat,
// multiplayer arm: Merciless Enforcers ({3}{B}: deals 1 damage to each
// opponent; lifelink) in a three-player game deals 1 to each of two
// opponents at once. Its controller gains 2 life as ONE event and Ajani's
// Pridemate triggers once.
func TestLifelinkDamageToEachOpponentIsOneLifeGainEvent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := lifelinkSeatBoard(t, reg, []string{"Merciless Enforcers", "Ajani's Pridemate"}, nil, nil)
	enf := findBattlefield(t, e, 0, "Merciless Enforcers", 0)
	pm := findBattlefield(t, e, 0, "Ajani's Pridemate", 0)
	addMana(t, e, 0, "BBBB")
	start := len(e.L.Events)
	opt := abilityOption(t, e, enf, 0)
	submitChoices(t, e, opt.Index)
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
	if o := e.G.Obj(pm); o.Counter("P1P1") != 1 {
		t.Fatalf("Ajani's Pridemate has %d +1/+1 counters, want 1 (one life-gain event)", o.Counter("P1P1"))
	}
	replayCheck(t, e, cfg)
}

// TestLifelinkTrampleOverAsksOneReplacementOrder pins the merged event
// through CR 616.1's order ask: with Angel of Vitality ("that much plus 1")
// and Boon Reflection ("twice that much") on its controller's battlefield,
// Fleetfoot Dancer trampling over Grizzly Bears gains life as ONE event, so
// the controller orders the two replacements ONCE, over the whole 4:
// plus 1 first gives (4+1)*2 = 10. Per recipient it was two asks over 2
// each, (2+1)*2 twice = 12.
func TestLifelinkTrampleOverAsksOneReplacementOrder(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg,
		[]string{"Fleetfoot Dancer", "Angel of Vitality", "Boon Reflection"}, nil,
		[]string{"Grizzly Bears"}, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	dancer := findBattlefield(t, e, 0, "Fleetfoot Dancer", 0)
	angel := findBattlefield(t, e, 0, "Angel of Vitality", 0)
	bears := findBattlefield(t, e, 1, "Grizzly Bears", 0)

	e.askAttackers()
	submitAttackers(t, e, dancer)
	submitBlockersOnly(t, e, bears)
	asks := 0
	for i := 0; i < 64; i++ {
		d := e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KReplacement {
			asks++
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{optionFor(t, d, angel)}}); err != nil {
				t.Fatalf("submit replacement order: %v", err)
			}
			continue
		}
		if d.Kind != decision.KPriority || (e.G.Step != state.StepDeclareBlockers && e.G.Step != state.StepCombatDamage) {
			break
		}
		submitPass(t, e)
	}
	if asks != 1 {
		t.Fatalf("replacement-order asks = %d, want 1 (one life-gain event for one source)", asks)
	}
	if got := e.G.Players[0].Life; got != 30 {
		t.Fatalf("Dancer's controller life = %d, want 30 (20 + (4+1)*2)", got)
	}
	replayCheck(t, e, cfg)
}
