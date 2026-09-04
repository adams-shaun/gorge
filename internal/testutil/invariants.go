// Package testutil holds fixtures and checks shared by tests that sit
// outside the rules package's own test binary -- Task 26's acceptance
// harness and rules/fuzz_test.go's invariant gate both need this without
// rules importing itself, so this package sits beside state and decision in
// the dependency order (cards -> state -> decision -> events -> effects ->
// rules -> view -> seat -> replay -> cmd/*) and imports only cards, state
// and decision (Ruling P9/P10).
package testutil

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// CheckInvariants is the rules core's acceptance gate: six checks from the
// spec's testing section plus a seventh Task 22 pinned by hand (Ruling P10).
// Every failure names the object or player at fault, and where -- the
// caller's own label for the point in the match this check ran at -- and
// stops at the first violation via t.Fatalf: this is a gate, not a report.
//
// d is the engine's currently pending decision, or nil between decisions
// (e.g. the game just ended). Only invariants 3, 5 and 7 need it --
// *state.Game alone cannot answer "is a decision pending for this player",
// and testutil cannot import rules (the fuzz test is package rules) to ask
// the engine directly (Ruling P10).
func CheckInvariants(t testing.TB, g *state.Game, d *decision.Decision, where string) {
	t.Helper()
	checkZones(t, g, where)       // 1, 6
	checkOneSurvivor(t, g, where) // 2
	checkNoNegatives(t, g, where) // 4
	if d == nil {
		return
	}
	checkDecisionForLiveOpponent(t, g, d, where) // 3, 7
	checkOptionIndices(t, d, where)              // 5
}

// checkZones is invariants 1 and 6: every object is in exactly one zone,
// Object.Zone agrees with the list holding it, ObjID 0 never appears in any
// list, and no object is in both a hidden zone's list and the battlefield's.
// It walks every zone list of every player plus g.Stack, then walks g.Objs
// by ID -- not the maps it built while walking -- so a failure is reported
// in a deterministic order regardless of Go's map iteration order.
func checkZones(t testing.TB, g *state.Game, where string) {
	t.Helper()
	seen := make(map[state.ObjID]int, len(g.Objs))
	hidden := make(map[state.ObjID]bool, len(g.Objs))
	battlefield := make(map[state.ObjID]bool, len(g.Objs))

	walk := func(z state.Zone, p state.PlayerID, ids []state.ObjID) {
		for _, id := range ids {
			if id == 0 {
				t.Fatalf("invariants (%s): ObjID 0 (no object) appears in the %s zone list for player %d",
					where, z, p)
				return
			}
			seen[id]++
			o := g.Obj(id)
			if o == nil {
				t.Fatalf("invariants (%s): the %s zone list for player %d holds object %d, which does not exist",
					where, z, p, id)
				return
			}
			if o.Zone != z {
				t.Fatalf("invariants (%s): object %d is in the %s zone list, but Object.Zone says %s",
					where, id, z, o.Zone)
				return
			}
			if z.Hidden() {
				hidden[id] = true
			}
			if z == state.ZBattlefield {
				battlefield[id] = true
			}
		}
	}

	for i := range g.Players {
		p := state.PlayerID(i)
		walk(state.ZLibrary, p, g.Zone(state.ZLibrary, p))
		walk(state.ZHand, p, g.Zone(state.ZHand, p))
		walk(state.ZBattlefield, p, g.Zone(state.ZBattlefield, p))
		walk(state.ZGraveyard, p, g.Zone(state.ZGraveyard, p))
		walk(state.ZExile, p, g.Zone(state.ZExile, p))
	}
	// The stack is not player-indexed (Game.Zone special-cases ZStack); the
	// PlayerID passed here is only ever used in a failure message.
	walk(state.ZStack, 0, g.Stack)

	for i := range g.Objs {
		id := state.ObjID(i + 1) // Objs is dense: Objs[i] has ID i+1.
		if n := seen[id]; n != 1 {
			t.Fatalf("invariants (%s): object %d appears in %d zone lists, want exactly 1", where, id, n)
			return
		}
		if hidden[id] && battlefield[id] {
			t.Fatalf("invariants (%s): object %d is in both a hidden zone's list and the battlefield's",
				where, id)
			return
		}
	}
}

// checkOneSurvivor is invariant 2: a finished game has at most one
// surviving player. A draw (zero survivors) is fine; two or more players
// both still in the game while g.Over is true is not.
func checkOneSurvivor(t testing.TB, g *state.Game, where string) {
	t.Helper()
	if !g.Over {
		return
	}
	alive := 0
	for _, p := range g.Players {
		if !p.Lost {
			alive++
		}
	}
	if alive > 1 {
		t.Fatalf("invariants (%s): game is over but %d players are still alive, want at most 1", where, alive)
	}
}

// checkNoNegatives is invariant 4: damage is never negative, and no counter
// is ever negative.
func checkNoNegatives(t testing.TB, g *state.Game, where string) {
	t.Helper()
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Damage < 0 {
			t.Fatalf("invariants (%s): object %d has negative damage (%d)", where, o.ID, o.Damage)
			return
		}
		for _, c := range o.Counters {
			if c.N < 0 {
				t.Fatalf("invariants (%s): object %d has a negative %q counter (%d)", where, o.ID, c.Kind, c.N)
				return
			}
		}
	}
}

// checkDecisionForLiveOpponent is invariants 3 and 7: no decision is ever
// pending for an eliminated player (3), nor for a player whose Life <= 0 --
// Task 22's measured pin, encoded here permanently (7).
func checkDecisionForLiveOpponent(t testing.TB, g *state.Game, d *decision.Decision, where string) {
	t.Helper()
	p := d.Player
	if int(p) >= len(g.Players) {
		t.Fatalf("invariants (%s): a decision is pending for player %d, which is not a real seat", where, p)
		return
	}
	pl := g.Players[p]
	if pl.Lost {
		t.Fatalf("invariants (%s): a decision is pending for player %d, who has already lost", where, p)
		return
	}
	if pl.Life <= 0 {
		t.Fatalf("invariants (%s): a decision is pending for player %d, whose life is %d", where, p, pl.Life)
	}
}

// checkOptionIndices is invariant 5: every option index in a pending
// decision is within range and unique.
func checkOptionIndices(t testing.TB, d *decision.Decision, where string) {
	t.Helper()
	seen := make(map[int]bool, len(d.Options))
	for i, o := range d.Options {
		if o.Index < 0 || o.Index >= len(d.Options) {
			t.Fatalf("invariants (%s): option %d of the pending %s decision has index %d, out of range for %d options",
				where, i, d.Kind, o.Index, len(d.Options))
			return
		}
		if seen[o.Index] {
			t.Fatalf("invariants (%s): option index %d appears more than once in the pending %s decision",
				where, o.Index, d.Kind)
			return
		}
		seen[o.Index] = true
	}
}
