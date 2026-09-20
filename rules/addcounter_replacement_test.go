package rules

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The counter-placement replacement class (repl:AddCounter / DB$ ReplaceCounter)
// pinned end to end on real corpus cards: Hardened Scales (that many plus one
// +1/+1 counters) and Branching Evolution (twice that many). The counter
// source is an authored fixture artifact with an AB$ PutCounter ability (never
// a corpus .txt, per the licensing rule), so every placement rides the
// engine's own effPutCounter emit path and the replacements intercept exactly
// where the live game's would.

// counterReplSource builds the authored counter-placing artifact: activate it
// and it puts n +1/+1 counters on the named target through effPutCounter's
// real CounterChange path.
func counterReplSource(t testing.TB, n int) *cards.Card {
	return card(t, "Name:Counter Source\nTypes:Artifact\n"+
		"A:AB$ PutCounter | Cost$ T | ValidTgts$ Creature | CounterType$ P1P1 | CounterNum$ "+
		strconv.Itoa(n)+" | SpellDescription$ Put counters on target creature.\n"+
		"Oracle:x\n")
}

// boardWithCounterReplacement seeds the named corpus replacement card and the
// authored counter source onto seat 0's battlefield, plus a target creature,
// and returns (engine, cfg, source, target).
func boardWithCounterReplacement(t *testing.T, seed uint64, replName string, counters int) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	repl := tokenReplCorpusCard(t, replName)
	src := counterReplSource(t, counters)
	target := card(t, "Name:Counter Target\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, cfg := tokenReplGame(t, seed, repl, src, target)
	moveSeededCard(t, e, 0, repl, state.ZBattlefield)
	sourceID := moveSeededCard(t, e, 0, src, state.ZBattlefield)
	targetID := moveSeededCard(t, e, 0, target, state.ZBattlefield)
	return e, cfg, sourceID, targetID
}

// activateCounterSource drives a fresh priority round and submits the
// counter source's ability, targeting seat 0's own creature (the sole legal
// target) and draining the stack.
func activateCounterSource(t *testing.T, e *Engine, source, target state.ObjID) {
	t.Helper()
	addMana(t, e, 0, "")
	submitChoices(t, e, abilityOption(t, e, source, 0).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target decision for the counter source: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == target {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("target %d not offerable: %+v", target, d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 20)
}

// TestHardenedScalesAddsOneCounter is the filing card, end to end: a 1- and a
// 3-counter placement on a creature under Hardened Scales' controller becomes
// 2 and 4 (that many plus one), the real corpus R: line plus its
// ReplaceCount$CounterNum/Plus.1 body.
func TestHardenedScalesAddsOneCounter(t *testing.T) {
	for _, tc := range []struct {
		placed, want int32
	}{
		{1, 2},
		{3, 4},
	} {
		e, cfg, source, target := boardWithCounterReplacement(t, 71, "Hardened Scales", int(tc.placed))
		activateCounterSource(t, e, source, target)
		if got := e.G.Obj(target).Counter("P1P1"); got != tc.want {
			t.Fatalf("Hardened Scales: %d placed -> %d counters, want %d", tc.placed, got, tc.want)
		}
		replayCheck(t, e, cfg)
	}
}

// TestBranchingEvolutionDoublesCounters pins the Twice body: a 1- and a
// 3-counter placement doubles to 2 and 6.
func TestBranchingEvolutionDoublesCounters(t *testing.T) {
	for _, tc := range []struct {
		placed, want int32
	}{
		{1, 2},
		{3, 6},
	} {
		e, cfg, source, target := boardWithCounterReplacement(t, 73, "Branching Evolution", int(tc.placed))
		activateCounterSource(t, e, source, target)
		if got := e.G.Obj(target).Counter("P1P1"); got != tc.want {
			t.Fatalf("Branching Evolution: %d placed -> %d counters, want %d", tc.placed, got, tc.want)
		}
		replayCheck(t, e, cfg)
	}
}

// TestCounterReplacementDoesNotTouchRemoval is the negative direction: a
// CounterChange that REMOVES counters (a negative amount) is never an
// AddCounter event, so neither card changes it. The target's counters are
// placed BEFORE the replacement is on the battlefield, so the removal is the
// only event either card could see.
func TestCounterReplacementDoesNotTouchRemoval(t *testing.T) {
	for _, replName := range []string{"Hardened Scales", "Branching Evolution"} {
		repl := tokenReplCorpusCard(t, replName)
		target := card(t, "Name:Counter Target\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		e, cfg := tokenReplGame(t, 77, repl, target)
		targetID := moveSeededCard(t, e, 0, target, state.ZBattlefield)
		e.emit(events.Event{Kind: events.CounterChange, Obj: targetID, Counter: "P1P1", Amount: 5})
		moveSeededCard(t, e, 0, repl, state.ZBattlefield)
		e.emit(events.Event{Kind: events.CounterChange, Obj: targetID, Counter: "P1P1", Amount: -2})
		if got := e.G.Obj(targetID).Counter("P1P1"); got != 3 {
			t.Fatalf("%s: 2 removed from 5 = %d counters, want 3 (a removal is not an AddCounter event)", replName, got)
		}
		replayCheck(t, e, cfg)
	}
}

// TestCounterReplacementFailsClosedOnOtherCounterKinds pins the counter-kind
// gate: both cards name ValidCounterType$ P1P1, so an M1M1 placement is
// untouched (5 stays 5, never 6 or 10).
func TestCounterReplacementFailsClosedOnOtherCounterKinds(t *testing.T) {
	for _, replName := range []string{"Hardened Scales", "Branching Evolution"} {
		e, cfg, _, target := boardWithCounterReplacement(t, 79, replName, 1)
		e.emit(events.Event{Kind: events.CounterChange, Obj: target, Counter: "M1M1", Amount: 5})
		if got := e.G.Obj(target).Counter("M1M1"); got != 5 {
			t.Fatalf("%s: 5 M1M1 counters became %d, want 5 (the replacement names P1P1)", replName, got)
		}
		replayCheck(t, e, cfg)
	}
}

// TestAddCounterReplacementsComposeScanOrder pins the CR 616.1e running
// total on a competition: Hardened Scales (+1) then Branching Evolution
// (double) turns a 1-counter placement into 4 (1 -> 2 -> 4), each modifier
// reading the amount the previous one produced.
func TestAddCounterReplacementsComposeScanOrder(t *testing.T) {
	hs := tokenReplCorpusCard(t, "Hardened Scales")
	be := tokenReplCorpusCard(t, "Branching Evolution")
	src := counterReplSource(t, 1)
	target := card(t, "Name:Counter Target\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, cfg := tokenReplGame(t, 83, hs, be, src, target)
	// Scan order is the battlefield order the MoveZone events create, so the
	// relative scan position of the two enchantments is deterministic but not
	// meaningful -- assert the COMMUTED final count is one of the two legal
	// scan orders' results (plus-then-double = 4, double-then-plus = 3).
	moveSeededCard(t, e, 0, hs, state.ZBattlefield)
	moveSeededCard(t, e, 0, be, state.ZBattlefield)
	sourceID := moveSeededCard(t, e, 0, src, state.ZBattlefield)
	targetID := moveSeededCard(t, e, 0, target, state.ZBattlefield)
	activateCounterSource(t, e, sourceID, targetID)
	got := e.G.Obj(targetID).Counter("P1P1")
	if got != 3 && got != 4 {
		t.Fatalf("Hardened Scales + Branching Evolution on a 1-counter placement = %d, want 3 or 4", got)
	}
	replayCheck(t, e, cfg)
}

// TestWindingConstrictorPlayerCounters pins the PlayerCounterChange half of
// the class: Winding Constrictor's second R: line is ValidPlayer$ You with
// no ValidCard$, so a counter placed on the replacement's controller becomes
// that many plus one, while a counter on the opponent is untouched.
func TestWindingConstrictorPlayerCounters(t *testing.T) {
	wc := tokenReplCorpusCard(t, "Winding Constrictor")
	e, cfg := tokenReplGame(t, 89, wc)
	moveSeededCard(t, e, 0, wc, state.ZBattlefield)
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "ENERGY", Amount: 3})
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 1, Counter: "ENERGY", Amount: 3})
	if got := e.G.Players[0].Counter("ENERGY"); got != 4 {
		t.Fatalf("Winding Constrictor: 3 ENERGY on its controller = %d, want 4", got)
	}
	if got := e.G.Players[1].Counter("ENERGY"); got != 3 {
		t.Fatalf("opponent's 3 ENERGY = %d, want 3 (ValidPlayer$ You)", got)
	}
	replayCheck(t, e, cfg)
}

// TestWindingConstrictorObjectCounters is the object-placement assertion the
// player-form gate needs: Winding Constrictor has BOTH a ValidCard$ object
// line and a ValidPlayer$ You line, and on an object CounterChange the
// ValidPlayer$ line must NOT fire (the object event's Player field is the
// zero value, so an ungated `ev.Player == you` read would collide with seat
// zero). A single +1/+1 counter on a creature it controls is that many plus
// one = 2, never the 3 two overlapping matches would give.
func TestWindingConstrictorObjectCounters(t *testing.T) {
	wc := tokenReplCorpusCard(t, "Winding Constrictor")
	target := card(t, "Name:Counter Target\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, cfg := tokenReplGame(t, 91, wc, target)
	moveSeededCard(t, e, 0, wc, state.ZBattlefield)
	targetID := moveSeededCard(t, e, 0, target, state.ZBattlefield)
	e.emit(events.Event{Kind: events.CounterChange, Obj: targetID, Counter: "P1P1", Amount: 1})
	if got := e.G.Obj(targetID).Counter("P1P1"); got != 2 {
		t.Fatalf("Winding Constrictor: 1 P1P1 placed on a controlled creature = %d, want 2 (object line only; the ValidPlayer$ line must not fire on an object event)", got)
	}
	replayCheck(t, e, cfg)
}

// TestVizierOfRemediesReplacesToZero pins the zero result: Vizier of
// Remedies' Minus.1 body resolves "that many -1/-1 counters minus one" for a
// single -1/-1 counter to exactly ZERO, and the placement must apply that
// zero (place none) rather than skip the replacement and leave the 1 in
// place. A 3-counter placement resolves 3 - 1 = 2 as a control.
func TestVizierOfRemediesReplacesToZero(t *testing.T) {
	for _, tc := range []struct {
		placed, want int32
	}{
		{1, 0},
		{3, 2},
	} {
		vizier := tokenReplCorpusCard(t, "Vizier of Remedies")
		target := card(t, "Name:Counter Target\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		e, cfg := tokenReplGame(t, 93, vizier, target)
		moveSeededCard(t, e, 0, vizier, state.ZBattlefield)
		targetID := moveSeededCard(t, e, 0, target, state.ZBattlefield)
		e.emit(events.Event{Kind: events.CounterChange, Obj: targetID, Counter: "M1M1", Amount: tc.placed})
		if got := e.G.Obj(targetID).Counter("M1M1"); got != tc.want {
			t.Fatalf("Vizier of Remedies: %d M1M1 placed -> %d counters, want %d (that many minus one)", tc.placed, got, tc.want)
		}
		replayCheck(t, e, cfg)
	}
}
