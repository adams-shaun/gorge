package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Dino DNA writes its Imprinted list with its first ability and reads it from
// a second one; Journey to the Lost City and Sivriss, Nightmare Speaker write
// without a same-chain unconditional clear. gorge keeps the list on the
// source (Object.Imprinted); the kernel keeps it in the per-source memory
// (CardDef.ImpPersist, dirac/zone_imprint.go) and seeds each resolution of
// the source from it. Naturalize makes a source leave; the list survives the
// departure and is cleared at the source's next battlefield entry.
// The fixture pool is dirac/cardset/pools/fixtures/imppersist.

var impPersistDecks = []fixtureDecks{{
	{
		{Name: "Dino DNA", Count: 4}, {Name: "Journey to the Lost City", Count: 3},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 6},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Forest", Count: 9}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Sivriss, Nightmare Speaker", Count: 4}, {Name: "Dino DNA", Count: 2},
		{Name: "Murder", Count: 4}, {Name: "Naturalize", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8},
	},
}}

var impPersistWatch = map[string]bool{
	"Dino DNA": true, "Journey to the Lost City": true, "Sivriss, Nightmare Speaker": true, "Naturalize": true,
}

func TestImpPersistLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var st dirac.SrcMemStats
	dirac.DebugSrcMemStats = &st
	defer func() { dirac.DebugSrcMemStats = nil }()
	run := runFixtureLockstep(t, "imppersist", impPersistDecks, impPersistWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v, imprints recorded %d (seeded %d, matched %d, cleared %d)",
		run.games, run.divs, run.stalls, run.casts, st.Recorded, st.Seeded, st.Matched, st.Cleared)
	if run.divs != 0 || run.stalls != 0 {
		t.Errorf("%d divergences, %d stalls; want none", run.divs, run.stalls)
	}
	if run.casts["Dino DNA"] < 40 {
		t.Errorf("only %d Dino DNA casts", run.casts["Dino DNA"])
	}
	if st.Recorded < 20 {
		t.Errorf("only %d imprints recorded: the writers rarely resolved", st.Recorded)
	}
}

// TestImpPersistMutation: without the kept list the second Dino DNA ability
// and the next resolution of a writer read an empty set, so the kernel must
// diverge from gorge.
func TestImpPersistMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSrcMemNoImp = true
	defer func() { dirac.DebugSrcMemNoImp = false }()
	run := runFixtureLockstep(t, "imppersist", impPersistDecks, impPersistWatch, 120, false, nil)
	run.requireDivergence(t, "the Imprinted list is not kept on the source")
}
