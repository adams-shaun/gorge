package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPaidGapGainMirrorsManaFeeNetsNoMana pins the X-bound sweep's paid-gap
// bookkeeping to the proof gorge runs (rules/potential_nogain.go
// manaFeeNetsNoMana): only a plain mana fee that produces no more than it
// spends (and brings no tap extra) leaves gapGain false. A cost with any
// non-mana part (a filter's {1}, {T}) is a gaining gap; an each-colour
// production counts every colour; a Combo/Any identity choice counts its one
// mana; a dynamic amount has no static price.
func TestPaidGapGainMirrorsManaFeeNetsNoMana(t *testing.T) {
	cases := []struct {
		name     string
		ab       kop.Ability
		tapExtra bool
		want     bool // gapGain
	}{
		{
			name: "plain mana fee that makes no more than it spends",
			ab: kop.Ability{
				Cost:       kop.Cost{Mana: kop.ManaCost{Generic: 2}},
				ManaColors: kop.ColAny, ManaN: 1, AFlags: kop.AfManaChoice,
			},
			want: false,
		},
		{
			name: "plain mana fee whose production outruns it",
			ab: kop.Ability{
				Cost:       kop.Cost{Mana: kop.ManaCost{Generic: 2}},
				ManaColors: kop.ColAny, ManaN: 3, AFlags: kop.AfManaChoice,
			},
			want: true,
		},
		{
			name: "fee with a tap part is not a plain mana fee",
			ab: kop.Ability{
				Cost:       kop.Cost{Flags: kop.CostTap, Mana: kop.ManaCost{Generic: 1}},
				ManaColors: kop.ColW, ManaN: 1,
			},
			want: true,
		},
		{
			name: "each-colour production counts every colour",
			ab: kop.Ability{
				Cost:       kop.Cost{Mana: kop.ManaCost{Generic: 2}},
				ManaColors: kop.ColW | kop.ColU, ManaN: 1, ManaEach: true, AFlags: kop.AfManaEachColor,
			},
			want: false,
		},
		{
			name: "each-colour production outruns a one-mana fee",
			ab: kop.Ability{
				Cost:       kop.Cost{Mana: kop.ManaCost{Generic: 1}},
				ManaColors: kop.ColW | kop.ColU, ManaN: 1, ManaEach: true, AFlags: kop.AfManaEachColor,
			},
			want: true,
		},
		{
			name: "identity-potential production reads one mana",
			ab: kop.Ability{
				Cost:  kop.Cost{Mana: kop.ManaCost{Generic: 2}},
				ManaN: 1, ManaIdentityPotential: true,
			},
			want: false,
		},
		{
			name: "dynamic production is not statically priced",
			ab: kop.Ability{
				Cost:       kop.Cost{Mana: kop.ManaCost{Generic: 2}},
				ManaColors: kop.ColAny, ManaN: 1, AFlags: kop.AfManaDynamic,
			},
			want: true,
		},
		{
			name: "a tap extra makes even a net mana fee gaining",
			ab: kop.Ability{
				Cost:       kop.Cost{Mana: kop.ManaCost{Generic: 2}},
				ManaColors: kop.ColAny, ManaN: 1, AFlags: kop.AfManaChoice,
			},
			tapExtra: true,
			want:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := payDesc{}
			paidGapBooked(&d, 0, 0, &tc.ab, tc.tapExtra)
			if d.gapGain != tc.want {
				t.Fatalf("gapGain = %v, want %v (cost %+v, ManaN %d, each %v, flags %#x)",
					d.gapGain, tc.want, tc.ab.Cost, tc.ab.ManaN, tc.ab.ManaEach, tc.ab.AFlags)
			}
			if d.gapN != 1 {
				t.Fatalf("gapN = %d, want 1 (the gap must be named before its gain is read)", d.gapN)
			}
		})
	}
}

// TestXBoundCountProofIsTotalManaOnly: the count arm compares totals, never a
// pip mask. Six covered units fund a {4}{U} announcement by count even when
// none of them makes blue (a repeatable converter may change colours at a
// mana loss), while four covered units do not.
func TestXBoundCountProofIsTotalManaOnly(t *testing.T) {
	c := cost(4, kop.U)
	var covered avail
	covered.total = 6
	if xBoundCountProof(&covered, &c) {
		t.Fatal("a six-mana covered census was called short of {4}{U} (5) on the pip mask")
	}
	covered.total = 4
	if !xBoundCountProof(&covered, &c) {
		t.Fatal("a four-mana covered census was not called short of {4}{U} (5)")
	}
}

// TestXBoundCountCensusNeedsAClosedRecord: an open production the bounded walk
// could not price leaves no proven upper bound, so the count arm must not run
// even when the walk recorded some units; a closed, complete record is the
// only census the proof may use.
func TestXBoundCountCensusNeedsAClosedRecord(t *testing.T) {
	var units [maxPotUnits]potUnit
	units[0] = potUnit{mask: uint8(kop.ColAny), n: 1}
	var countEx avail
	if xBoundCountCensus(true, &countEx, &units, 20) {
		t.Fatal("an open production still produced a count census")
	}
	if !xBoundCountCensus(false, &countEx, &units, 20) {
		t.Fatal("a closed, complete record did not produce a count census")
	}
	units[0] = potUnit{mask: uint8(kop.ColAny), n: 0} // the overflow marker
	if xBoundCountCensus(false, &countEx, &units, 20) {
		t.Fatal("an incomplete record still produced a count census")
	}
}
