package kgen

import "testing"

// TestMayPlaySelfZonePair pins the two-zone self may-play lowering (Squee,
// the Immortal's "You may cast CARDNAME from your graveyard or from exile"):
// AffectedZone$ Exile,Graveyard + EffectZone$ Exile,Graveyard lowers to ONE
// STATIC PER ZONE, each with its own zone (so its own filter zone, the
// graveyard filter matching the card only there) and the zone's own
// FunctionsIn flag (the kernel's self walk reads the graveyard statics
// through sclsGyMay + StFunctionsInGraveyard and the exile statics through
// sclsExMay + StFunctionsInExile; statics_gen.go mayPlayStaticsOf). One
// static carrying both flags compiles its one filter with one zone, so the
// other zone's match fails -- the divergence the split fixes.
func TestMayPlaySelfZonePair(t *testing.T) {
	c := lowerNamedCard(t, "Squee, the Immortal")
	if len(c.Refusals) != 0 {
		t.Fatalf("Squee, the Immortal refusals = %v, want none", c.Refusals)
	}
	var mps []lStatic
	for _, st := range c.Statics {
		if st.Kind == "MayPlayFrom" {
			mps = append(mps, st)
		}
	}
	// the precondition the zone reads hang on: both per-zone statics lowered
	if len(mps) != 2 {
		t.Fatalf("Squee, the Immortal: %d MayPlayFrom statics, want one per zone (2)", len(mps))
	}
	want := map[int][]string{
		zGraveyard: {"FunctionsInGraveyard"},
		zExile:     {"FunctionsInExile"},
	}
	for _, st := range mps {
		flags, ok := want[st.Zone]
		if !ok {
			t.Errorf("Squee MayPlayFrom static zone = %d, want %d or %d", st.Zone, zGraveyard, zExile)
			continue
		}
		delete(want, st.Zone)
		if len(st.Flags) != len(flags) || st.Flags[0] != flags[0] {
			t.Errorf("Squee zone-%d static flags = %v, want %v", st.Zone, st.Flags, flags)
		}
		if st.F == nil || len(st.F.Clauses) != 1 || st.F.Clauses[0].Flags&fSelf == 0 {
			t.Errorf("Squee zone-%d static filter = %+v, want the Card.Self clause", st.Zone, st.F)
		}
	}
	if len(want) != 0 {
		t.Errorf("Squee: no MayPlayFrom static for zone(s) %v", want)
	}
}
