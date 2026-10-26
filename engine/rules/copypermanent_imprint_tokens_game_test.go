package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// shredderEngine seeds a 3-seat game with Shredder, Shadow Master on seat 0's
// battlefield and returns the engine, its replay config and Shredder's id.
func shredderEngine(t *testing.T) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	shredder := lookup(t, reg, "Shredder, Shadow Master")
	deck := append([]*cards.Card{shredder}, mountainDeck(t, 39)...)
	cfg := seatZeroStart(Config{Seed: 42, Names: []string{"a", "b", "c"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{deck, mountainDeck(t, 40), mountainDeck(t, 40)}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	id := moveByName(t, e, 0, "Shredder, Shadow Master", state.ZBattlefield)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Shredder on the battlefield under seat 0, got %+v", o)
	}
	return e, cfg, id
}

// TestShredderShadowMasterCopiesSacrificedAtEndOfCombat drives the real corpus
// Shredder, Shadow Master end to end: it attacks seat 1, its attack trigger
// creates one tapped-and-attacking copy aimed at the OTHER opponent (seat 2),
// ImprintTokens$ True records that copy on Shredder, and the end-of-combat
// DelayedTrigger reads ImprintedLKI back and sacrifices the copy while
// Shredder itself stays.
func TestShredderShadowMasterCopiesSacrificedAtEndOfCombat(t *testing.T) {
	t.Parallel()
	e, cfg, shredder := shredderEngine(t)
	shredderCard := e.G.Obj(shredder).Card

	// Shredder entered turn 1 and is summoning sick; attack on seat 0's next
	// turn (turn 4 in a 3-seat game).
	ateotDriveToStep(t, e, 4, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("no attackers decision, got %+v", d)
	}
	attack := -1
	for _, o := range d.Options {
		if o.Obj == shredder && o.Player == 1 {
			attack = o.Index
		}
	}
	if attack < 0 {
		t.Fatalf("Shredder was not offered an attack against seat 1: %+v", d.Options)
	}
	submitChoices(t, e, attack)

	// The attack trigger resolves: one copy for the other opponent (seat 2).
	answerQuiet(t, e, 200)
	var copyID state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o != nil && o.IsToken && o.IsCopy && o.Card == shredderCard {
			if copyID != 0 {
				t.Fatalf("more than one Shredder copy: %d and %d", copyID, id)
			}
			copyID = id
		}
	}
	if copyID == 0 {
		t.Fatal("the attack trigger created no Shredder token copy")
	}
	o := e.G.Obj(copyID)
	if !o.Tapped || !o.IsAttacking || o.Attacking != 2 || o.Controller != 0 {
		t.Fatalf("Shredder copy: tapped=%v attacking=%v defender=%d controller=%d, want tapped attacking seat 2 under seat 0",
			o.Tapped, o.IsAttacking, o.Attacking, o.Controller)
	}
	if slices.Contains(e.Derived(copyID).Types, "Legendary") {
		t.Fatalf("Shredder copy types = %v, want no Legendary (NonLegendary$)", e.Derived(copyID).Types)
	}
	// ImprintTokens$ is the read-back the end-of-combat DelayedTrigger needs.
	// The carrier's own DBCleanup clears the source's ImprintTokens as part of
	// this same resolution, so pin the two logged facts that make the
	// sacrifice work: the Imprint event naming the copy, and the DelayedTrigger
	// registration whose RememberObjects$ ImprintedLKI read it back.
	imprinted := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Imprint && ev.Obj == shredder && slices.Contains(ev.IDs, copyID) {
			imprinted = true
		}
	}
	if !imprinted {
		t.Fatalf("no Imprint event recorded the copy %d on Shredder", copyID)
	}
	registered := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.DelayedRegister && slices.Contains(ev.IDs, copyID) {
			registered = true
		}
	}
	if !registered {
		t.Fatalf("the end-of-combat DelayedTrigger did not register the imprinted copy %d", copyID)
	}

	// Drive through the rest of combat and resolve the end-of-combat trigger.
	shredderDriveToStep(t, e, 4, 0, state.StepEndCombat)
	answerQuiet(t, e, 200)

	if got := e.G.Obj(copyID).Zone; got == state.ZBattlefield || got == state.ZStack {
		t.Fatalf("the Shredder copy is still in the game at end of combat: %s", got)
	}
	sacrificed := false
	for _, ev := range e.L.Events {
		if events.IsSacrifice(ev) && ev.Obj == copyID {
			sacrificed = true
		}
	}
	if !sacrificed {
		t.Fatalf("the end-of-combat DelayedTrigger never sacrificed the copy %d", copyID)
	}
	if got := e.G.Obj(shredder).Zone; got != state.ZBattlefield {
		t.Fatalf("Shredder itself moved at end of combat: %s", got)
	}
	replayCheck(t, e, cfg)
}

// shredderDriveToStep drives to a step, answering combat declarations empty and
// any other non-priority decision with its first Min options (the end-of-combat
// window can pose a multi-pick this fixture only has to get past).
func shredderDriveToStep(t *testing.T, e *Engine, turn int32, active state.PlayerID, step state.Step) {
	t.Helper()
	for i := 0; i < 6000; i++ {
		if e.G.Turn == turn && e.G.Active == active && e.G.Step == step {
			return
		}
		if e.G.Over {
			t.Fatalf("game ended before reaching turn %d seat %d step %s", turn, active, step)
		}
		d := e.Pending()
		if d == nil {
			continue
		}
		switch d.Kind {
		case decision.KAttackers, decision.KBlockers:
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player}); err != nil {
				t.Fatalf("submit empty combat declaration: %v", err)
			}
		case decision.KPriority:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
				t.Fatalf("submit pass: %v", err)
			}
		case decision.KTriggerOrder:
			answerTriggerOrders(t, e)
		default:
			t.Logf("driving: answering %s resume=%q Min/Max=%d/%d options=%d at turn %d step %s",
				d.Kind, d.ResumeKind, d.Min, d.Max, len(d.Options), e.G.Turn, e.G.Step)
			n := d.Min
			if n == 0 {
				n = 1
			}
			if n > len(d.Options) {
				n = len(d.Options)
			}
			choices := make([]int, 0, n)
			for k := 0; k < n; k++ {
				choices = append(choices, d.Options[k].Index)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
				t.Fatalf("submit %v: %v", choices, err)
			}
		}
	}
	t.Fatalf("did not reach turn %d seat %d step %s", turn, active, step)
}
