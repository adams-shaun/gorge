package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func TestPrintedStrengthBuckets(t *testing.T) {
	cr := func(mc string, p, tt int32) CardSpec {
		return CardSpec{Types: "Creature Bear", ManaCost: mc, Power: p, Toughness: tt}
	}
	tr := kop.KwTrample
	for _, c := range []struct {
		name string
		sp   CardSpec
		kw   kop.KwMask
		mv   int32
		want uint8
	}{
		{"1/1 for 1", cr("{G}", 1, 1), 0, 1, 0},
		{"2/2 for 2", cr("{1}{G}", 2, 2), 0, 2, 1},
		{"4/4 for 4", cr("{3}{G}", 4, 4), 0, 4, 2},
		{"6/6 trample for 6", cr("{5}{G}", 6, 6), tr, 6, 3},
		{"6-MV sorcery", CardSpec{Types: "Sorcery", ManaCost: "{5}{R}"}, 0, 6, 2},
		{"basic land", CardSpec{Types: "Basic Land Forest"}, 0, 0, 0},
	} {
		if got := StrengthBucket(printedStrength(c.sp, c.kw, c.mv)); got != c.want {
			t.Errorf("%s: bucket %d, want %d", c.name, got, c.want)
		}
	}
	for s, want := range map[int32]uint8{3: 0, 4: 1, 7: 1, 8: 2, 12: 2, 13: 3} {
		if got := StrengthBucket(s); got != want {
			t.Errorf("StrengthBucket(%d)=%d want %d", s, got, want)
		}
	}
}
