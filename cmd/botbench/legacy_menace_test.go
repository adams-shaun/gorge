package main

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// TestLegacyMenaceLegalDropsALoneBlockOnAMenaceAttacker pins the bench-seat
// guard that keeps the frozen, fact-free legacy policy from submitting an
// intent the engine rejects. LegacyDecide coins one blocker per attacker and
// cannot see Menace, so it can leave a lone block on a Menace attacker; the
// engine's validateBlockers rejects that and aborts the whole run (measured
// at HEAD: `botbench -a bot -b legacy` aborts at seed 22, intent 216). The
// guard withholds exactly that declaration.
func TestLegacyMenaceLegalDropsALoneBlockOnAMenaceAttacker(t *testing.T) {
	const (
		menaceAtk = state.ObjID(9)
		plainAtk  = state.ObjID(20)
		blockerA  = state.ObjID(30)
		blockerB  = state.ObjID(31)
		blockerC  = state.ObjID(32)
	)

	d := &decision.Decision{Kind: decision.KBlockers, Options: []decision.Option{
		{Index: 0, Kind: "block", Obj: blockerA, Attacker: menaceAtk},
		{Index: 1, Kind: "block", Obj: blockerB, Attacker: menaceAtk},
		{Index: 2, Kind: "block", Obj: blockerC, Attacker: plainAtk},
	}}

	// Precondition: the view really marks the attacker Menace, and the
	// plain attacker is NOT Menace -- otherwise the guard would be vacuous.
	v := view.View{Players: []view.PlayerView{{Battlefield: []view.CardView{
		{ID: menaceAtk, Keywords: []string{"Menace"}},
		{ID: plainAtk, Keywords: []string{"Trample"}},
	}}}}
	if !attackerHasMenace(v, menaceAtk) || attackerHasMenace(v, plainAtk) {
		t.Fatalf("precondition failed: view menace facts wrong for %d/%d", menaceAtk, plainAtk)
	}

	// A lone block on the Menace attacker is dropped; the plain attacker's
	// block survives.
	got := legacyMenaceLegal(v, d, []int{0, 2})
	if len(got) != 1 || got[0] != 2 {
		t.Fatalf("lone Menace block = %v, want just the plain attacker's option [2]", got)
	}

	// A TWO-blocker team on the Menace attacker is legal and survives, so
	// the guard is not a blanket suppression of Menace blocks.
	got = legacyMenaceLegal(v, d, []int{0, 1, 2})
	if len(got) != 3 {
		t.Fatalf("legal Menace team = %v, want all three options kept", got)
	}

	// A non-blockers decision is untouched.
	kp := &decision.Decision{Kind: decision.KPriority, Options: []decision.Option{
		{Index: 0, Kind: "activate", Obj: blockerA},
	}}
	if got := legacyMenaceLegal(v, kp, []int{0}); len(got) != 1 || got[0] != 0 {
		t.Fatalf("non-blockers decision mutated: %v", got)
	}
}

// attackerHasMenace mirrors the guard's read so the test's precondition is
// checked on the same facts the guard consumes.
func attackerHasMenace(v view.View, id state.ObjID) bool {
	for _, p := range v.Players {
		for _, cv := range p.Battlefield {
			if cv.ID != id {
				continue
			}
			for _, k := range cv.Keywords {
				if k == "Menace" {
					return true
				}
			}
		}
	}
	return false
}
