package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Officious Interrogation's "costs {W}{U} more for each target beyond the
// first" lowers to a card-own StatIncreaseCost with StRaisePerTarget, the
// pips per charge packed in Static.B.
func TestRaiseCostPerTargetLowering(t *testing.T) {
	all, bs, _, err := buildPool(lowerNamedPool(t, []string{"Officious Interrogation"}))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for i, c := range all {
		if c.Name != "Officious Interrogation" {
			continue
		}
		if r := lowerRefusals(c); len(r) != 0 {
			t.Fatalf("refused: %v", r)
		}
		for _, a := range bs[i].def.Abilities {
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatIncreaseCost {
				continue
			}
			found++
			st := a.Static
			want := int16(1) | int16(1)<<kop.ReduceColorBits
			if st.StFlags&kop.StRaisePerTarget == 0 || st.A != 0 || st.B != want || st.Zone != kop.ZoneCommand {
				t.Errorf("static flags %#x A %d B %#x Zone %v, want StRaisePerTarget, 0, %#x, ZoneCommand", st.StFlags, st.A, st.B, st.Zone, want)
			}
		}
	}
	if found != 1 {
		t.Fatalf("%d StatIncreaseCost abilities, want 1", found)
	}
}
