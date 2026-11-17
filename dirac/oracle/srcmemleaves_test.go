package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Dance of Many's enters trigger copies a nontoken creature and remembers the
// token; its leaves trigger exiles the remembered token and a second trigger
// sacrifices Dance when the token (ValidCard$ Card.IsRemembered) leaves. gorge
// keeps the list on the source (Object.Remembered); the kernel keeps it in the
// per-source memory (CardDef.SrcMem, dirac/zone_srcmemrem.go): the leaves
// trigger starts with the records of the incarnation it fired from, and the
// watch trigger's filter reads them beside the resolution's set. Boomerang
// returns Dance (or the token's creature) and Naturalize destroys it, so the
// source leaves with the list still kept and gets a new one on re-entry.
// The fixture pool is dirac/cardset/pools/fixtures/srcmemleaves.

var srcMemLeavesDecks = []fixtureDecks{{
	{
		{Name: "Dance of Many", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Hill Giant", Count: 6}, {Name: "Boomerang", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Naturalize", Count: 4},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
	},
}}

var srcMemLeavesWatch = map[string]bool{"Dance of Many": true, "Boomerang": true, "Naturalize": true}

func TestSrcMemLeavesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var st dirac.SrcMemStats
	dirac.DebugSrcMemStats = &st
	defer func() { dirac.DebugSrcMemStats = nil }()
	run := runFixtureLockstep(t, "srcmemleaves", srcMemLeavesDecks, srcMemLeavesWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v, records %d (seeded %d, reads %d, matched %d, cleared %d)",
		run.games, run.divs, run.stalls, run.casts, st.Recorded, st.Seeded, st.Reads, st.Matched, st.Cleared)
	if run.divs != 0 || run.stalls != 0 {
		t.Errorf("%d divergences, %d stalls; want none", run.divs, run.stalls)
	}
	if run.casts["Dance of Many"] < 60 {
		t.Errorf("only %d Dance of Many casts", run.casts["Dance of Many"])
	}
	if st.Recorded < 30 {
		t.Errorf("only %d records written: the enters trigger rarely resolved", st.Recorded)
	}
	if st.Matched < 5 {
		t.Errorf("only %d watch-trigger reads found a record: the token rarely left first", st.Matched)
	}
	if st.Seeded < 10 {
		t.Errorf("only %d seeded resolutions: the leaves trigger rarely read the kept set", st.Seeded)
	}
}

// TestSrcMemLeavesMutation: without the records the leaves trigger starts with
// an empty set and the watch trigger never matches the token, so the kernel
// must diverge from gorge.
func TestSrcMemLeavesMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSrcMemNoRem = true
	defer func() { dirac.DebugSrcMemNoRem = false }()
	run := runFixtureLockstep(t, "srcmemleaves", srcMemLeavesDecks, srcMemLeavesWatch, 120, false, nil)
	run.requireDivergence(t, "the Remembered list is not kept on the source")
}
