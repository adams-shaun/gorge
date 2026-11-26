package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTwoZoneSelfGrantFilterIsZoneFree: Squee, the Immortal's Exile,Graveyard
// self grant builds ONE MayPlayFrom static per zone (mayPlaySelfZones), each
// with exactly its own zone's function flag and a Self filter whose zone is
// that same zone: the kernel's self walk matches the filter at the card's own
// zone, so a filter zone that disagreed with the flag would withhold the cast
// the flag opens, and one static carrying both flags was the bug that left
// Squee uncastable from the graveyard.
func TestTwoZoneSelfGrantFilterIsZoneFree(t *testing.T) {
	lc := lowerNamedCard(t, "Squee, the Immortal")
	if len(lc.Refusals) != 0 {
		t.Fatalf("Squee refused: %v", lc.Refusals)
	}
	pt := newPoolTables(&lowered{cards: []*lCard{lc}}, nil)
	b := buildCard(pt, lc)
	if len(b.refusals) != 0 {
		t.Fatalf("Squee build refused: %v", b.refusals)
	}
	want := map[kop.Zone]uint32{
		kop.ZoneGraveyard: uint32(kop.StFunctionsInGraveyard),
		kop.ZoneExile:     uint32(kop.StFunctionsInExile),
	}
	got := map[kop.Zone]bool{}
	both := uint32(kop.StFunctionsInGraveyard | kop.StFunctionsInExile)
	for _, a := range b.def.Abilities {
		st := a.Static
		if a.Kind != kop.AbStatic || st.Kind != kop.StatMayPlayFrom {
			continue
		}
		if st.Filter == 0 {
			t.Fatal("precondition: the Self filter is interned (non-zero)")
		}
		if uint32(st.StFlags)&both == both {
			t.Fatalf("StFlags %#x carry both function flags; want one static per zone", st.StFlags)
		}
		z := pt.filters[st.Filter].Zone
		f, ok := want[z]
		if !ok {
			t.Fatalf("the Self filter's zone is %v, want graveyard or exile", z)
		}
		if uint32(st.StFlags)&f == 0 {
			t.Errorf("filter zone %v but StFlags %#x lack its function flag", z, st.StFlags)
		}
		got[z] = true
	}
	if len(got) != 2 {
		t.Fatalf("MayPlayFrom statics built for zones %v, want graveyard and exile", got)
	}
}
