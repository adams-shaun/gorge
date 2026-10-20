package botpolicy

import (
	"reflect"
	"testing"
)

func TestBlockCombatDoubleStrike(t *testing.T) {
	c := func(p, tough int32, kw ...string) Creature { return Creature{Power: p, Toughness: tough, Keywords: kw} }
	for _, tc := range []struct {
		name     string
		a        Creature
		block    []Creature
		wantOff  []bool // dead, ds=false
		aOff     bool
		wantOn   []bool
		aOn      bool
		offKnown bool
	}{
		{"2/2 DS into 2/2", c(2, 2, "Double Strike"), []Creature{c(2, 2)}, []bool{true}, true, []bool{true}, false, true},
		{"3/3 DS into 4/4", c(3, 3, "Double Strike"), []Creature{c(4, 4)}, []bool{false}, true, []bool{true}, true, true},
		{"3/3 into 1/1 DS and 1/4", c(3, 3), []Creature{c(1, 1, "Double Strike"), c(1, 4)}, nil, false, []bool{true, false}, true, false},
		{"2/2 FS into 2/2", c(2, 2, "First Strike"), []Creature{c(2, 2)}, []bool{true}, false, []bool{true}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.offKnown {
				aDead, dead := blockCombatDS(tc.a, tc.block, false)
				if aDead != tc.aOff || !reflect.DeepEqual(dead, tc.wantOff) {
					t.Errorf("ds=false = (%v,%v), want (%v,%v)", aDead, dead, tc.aOff, tc.wantOff)
				}
				if a2, d2 := blockCombat(tc.a, tc.block); a2 != aDead || !reflect.DeepEqual(d2, dead) {
					t.Errorf("blockCombat = (%v,%v), want the ds=false answer (%v,%v)", a2, d2, aDead, dead)
				}
			}
			aDead, dead := blockCombatDS(tc.a, tc.block, true)
			if aDead != tc.aOn || !reflect.DeepEqual(dead, tc.wantOn) {
				t.Errorf("ds=true = (%v,%v), want (%v,%v)", aDead, dead, tc.aOn, tc.wantOn)
			}
		})
	}
}
