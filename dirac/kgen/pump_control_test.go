package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPumpControlLowers: a battlefield Pump keyword grant with Duration$
// AsLongAsControl lowers to one host-anchored override record
// (WhileHost + AsLongAsControl) per card, carrying the granted keywords.
func TestPumpControlLowers(t *testing.T) {
	cases := []struct {
		name   string
		kw     kop.KwMask
		cantAt bool
	}{
		{"Aegis Angel", kop.KwIndestructible, false},
		{"Kyodai, Soul of Kamigawa", kop.KwIndestructible, false},
		{"Thancred Waters", kop.KwIndestructible, false},
		{"Tale of Tinúviel", kop.KwIndestructible, false},
		{"There and Back Again", kop.KwCantBlock, false},
		{"Possession Engine", kop.KwCantBlock, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, c := range lowerNamed(t, []string{tc.name}) {
				for _, r := range c.Refusals {
					t.Errorf("%s: refused: %s", c.Name, r.Reason)
				}
			}
			_, _, pt, err := buildPool(lowerNamedPool(t, []string{tc.name}))
			if err != nil {
				t.Fatal(err)
			}
			var got []kop.Override
			for _, o := range pt.overrides {
				if o.AsLongAsControl {
					got = append(got, o)
				}
			}
			if len(got) != 1 {
				t.Fatalf("%d AsLongAsControl overrides (of %d), want exactly 1", len(got), len(pt.overrides))
			}
			o := got[0]
			if !o.WhileHost {
				t.Errorf("record is not WhileHost: %+v", o)
			}
			if o.AddKw&tc.kw != tc.kw {
				t.Errorf("AddKw %#x lacks %#x", uint64(o.AddKw), uint64(tc.kw))
			}
			if o.CantAttack != tc.cantAt {
				t.Errorf("CantAttack = %v, want %v", o.CantAttack, tc.cantAt)
			}
		})
	}
}
