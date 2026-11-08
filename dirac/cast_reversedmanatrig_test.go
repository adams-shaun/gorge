package dirac

import "testing"

// TestReversedCastKeepsManaAbilityTriggers pins CR 733.1 for a reversed
// cast: the cast is undone, so what its own proposal queued (the push, its
// targets) is dropped, but a mana ability its payment activated is not undone,
// so the trigger that ability queued (Springleaf Drum tapping Wanderwine
// Distracter: "whenever this becomes tapped") stays pending, as gorge's
// abortCast drops only pendingCast.proposalTriggers.
func TestReversedCastKeepsManaAbilityTriggers(t *testing.T) {
	sc, id := kcastScen(t)
	scour := id("Thought Scour")
	o := sc.hand(scour, 0)
	sc.kcastBuild(o, scour, 0)
	push := sc.s.npend
	if sc.s.bld().pushNP != push {
		t.Fatalf("push mark %d, want %d", sc.s.bld().pushNP, push)
	}
	proposal, mana := Obj(1), Obj(2)
	// a trigger the proposal queued (a target choice), then the mana step
	// begins (manaStep's mark) and a mana ability's tap queues another
	sc.s.pend[sc.s.npend] = pendingTrig{obj: proposal, ab: abCascade}
	sc.s.npend++
	sc.s.bld().manaNP = 1 + sc.s.npend
	sc.s.pend[sc.s.npend] = pendingTrig{obj: mana, ab: abCascade}
	sc.s.npend++
	if sc.s.npend != push+2 {
		t.Fatalf("pending %d before the reversal, want %d", sc.s.npend, push+2)
	}
	sc.s.unpushSpell(sc.k)
	if sc.s.npend != push+1 {
		t.Fatalf("pending %d after the reversal, want %d (the mana ability's trigger kept)", sc.s.npend, push+1)
	}
	if got := sc.s.pend[push].obj; got != mana {
		t.Fatalf("kept trigger of object %d, want the mana ability's %d", got, mana)
	}
}
