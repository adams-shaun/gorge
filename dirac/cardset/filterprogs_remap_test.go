package cardset

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFilterProgsRoundTripAndRemap: a PoolData's filter programs survive the
// table codec, and a matchup's copy renumbers an FwSubtype literal's pool
// subtype index (a subtype the matchup lacks maps past every subtype), leaving
// the pool's programs and every other literal untouched.
func TestFilterProgsRoundTripAndRemap(t *testing.T) {
	pd := sampleData()
	pd.FilterProgs = []kop.FilterProg{{}, {Lits: []kop.FLit{
		{W: kop.FwPowerCmp, Op: kop.CmpGE, V: kop.Const(2)},
		{W: kop.FwSubtype, Arg: 7},
		{W: kop.FwSubtype, Arg: 9, Neg: true},
		{W: kop.FwKeyword, Arg: 3, Neg: true},
	}}}
	got, _, err := DecodeTable(EncodeTable(pd, "k"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.FilterProgs, pd.FilterProgs) {
		t.Fatalf("round trip: got %+v, want %+v", got.FilterProgs, pd.FilterProgs)
	}
	out := remapFilterProgs(pd.FilterProgs, map[uint8]uint8{7: 2})
	l := out[1].Lits
	if l[1].Arg != 2 || l[2].Arg != NoSubtype-1 || !l[2].Neg {
		t.Errorf("subtype literals %+v, want 7->2 and 9->NoSubtype-1", l[1:3])
	}
	if l[0] != pd.FilterProgs[1].Lits[0] || l[3] != pd.FilterProgs[1].Lits[3] {
		t.Errorf("other literals changed: %+v", l)
	}
	if pd.FilterProgs[1].Lits[1].Arg != 7 {
		t.Error("the pool's program was renumbered in place")
	}
	if remapFilterProgs(nil, nil) != nil {
		t.Error("no programs must stay nil")
	}
}
