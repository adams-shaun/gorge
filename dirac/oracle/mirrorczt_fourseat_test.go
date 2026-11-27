package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// mirrorCztDeck is the four-seat Mirror-Style Master deck: the copies need an
// attacking modified creature, so Bears with an Equipment or an Aura.
func mirrorCztDeck(t *testing.T) []string {
	t.Helper()
	d := pod4Deck(deckOf(3, "Mirror-Style Master"), deckOf(8, "Grizzly Bears"),
		deckOf(3, "Bonesplitter"), deckOf(2, "Rancor"), deckOf(24, "Mountain"))
	if len(d) != 40 {
		t.Fatalf("deck has %d cards, want 40", len(d))
	}
	return d
}

// TestFourSeatMirrorCztLockstep: the batched RepeatEach body copies each
// attacking modified creature, and each copy's TokenAttacking$ True poses
// gorge's CR 508.4 defender ask before it enters. Four seats so the copy has
// two or more defenders to choose from; the loop parks on the ask, so the
// zone batch and entry hold stay open across it.
func TestFourSeatMirrorCztLockstep(t *testing.T) {
	d := mirrorCztDeck(t)
	st := &dirac.TokenAttackingStats{}
	dirac.DebugTokenAttackingStats = st
	defer func() { dirac.DebugTokenAttackingStats = nil }()
	if n := pod4FixtureLockstep(t, repeateachcztPool, [4][]string{d, d, d, d}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
	if st.Asks == 0 || st.Copies == 0 {
		t.Fatalf("copy stats %+v: want a CR 508.4 ask and a copy minted", *st)
	}
}

// TestFourSeatMirrorCztMutation: the CR 508.4 copy ask fires in the batched
// loop, posed before the copy enters with the loop's zone batch still open
// (BatchedAsks == BatchedHeld), and disabling the ask diverges: the kernel
// then attacks the fallback while gorge asks which defender.
func TestFourSeatMirrorCztMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	d := mirrorCztDeck(t)
	st := &dirac.TokenAttackingStats{}
	dirac.DebugTokenAttackingStats = st
	if n := pod4FixtureLockstep(t, repeateachcztPool, [4][]string{d, d, d, d}, 20, false, nil); n != 0 {
		dirac.DebugTokenAttackingStats = nil
		t.Fatalf("baseline: %d diverged", n)
	}
	dirac.DebugTokenAttackingStats = nil
	if st.BatchedAsks == 0 {
		t.Fatalf("copy stats %+v: no batched copy ask posed", *st)
	}
	if st.BatchedHeld != st.BatchedAsks {
		t.Fatalf("copy stats %+v: %d batched asks but only %d with the zone batch open", *st, st.BatchedAsks, st.BatchedHeld)
	}
	if st.Copies == 0 {
		t.Fatalf("copy stats %+v: no copy minted", *st)
	}
	dirac.DebugNoCopyDefenderAsk = true
	defer func() { dirac.DebugNoCopyDefenderAsk = false }()
	if n := pod4FixtureLockstep(t, repeateachcztPool, [4][]string{d, d, d, d}, 20, false, nil); n == 0 {
		t.Fatal("DebugNoCopyDefenderAsk: no divergence in 20 games")
	}
}
