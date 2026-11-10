package dirac

import (
	"math/rand"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// oneColourFitsExhaustive is the search before it was made symmetric: each
// source tries each of the five colours in turn.
func (a avail) oneColourFitsExhaustive(cost *kop.ManaCost, pipMask uint8, extra int, ns []uint8) bool {
	if len(ns) == 0 {
		return a.hallUnits(cost, pipMask, extra)
	}
	for c := 0; c < 5; c++ {
		b := a
		b.addUnits(1<<c, ns[0])
		if b.oneColourFitsExhaustive(cost, pipMask, extra, ns[1:]) {
			return true
		}
	}
	return false
}

// eachAmountMultiset calls f with every non-decreasing list of up to max
// amounts in 1..4.
func eachAmountMultiset(max int, f func(ns []uint8)) {
	var rec func(ns []uint8)
	rec = func(ns []uint8) {
		f(ns)
		if len(ns) == max {
			return
		}
		lo := uint8(1)
		if len(ns) > 0 {
			lo = ns[len(ns)-1]
		}
		for n := lo; n <= 4; n++ {
			rec(append(ns, n))
		}
	}
	rec(nil)
}

// TestOneColourSymmetricMatchesExhaustive: grouping equal-amount one-colour
// sources and trying each colour multiset answers as the per-source search
// does, on every multiset of up to six amounts and on eight equal ones.
func TestOneColourSymmetricMatchesExhaustive(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	var fits, misses, lists int
	check := func(ns []uint8) {
		lists++
		for i := 0; i < 150; i++ {
			var a avail
			for c := 0; c < 5; c++ {
				if rng.Intn(2) == 0 {
					a.addUnits(1<<c, uint8(rng.Intn(4)))
				}
			}
			if rng.Intn(3) == 0 {
				a.addUnits(uint8(1<<rng.Intn(5)|1<<rng.Intn(5)), uint8(1+rng.Intn(3)))
			}
			var cost kop.ManaCost
			cost.Generic = uint8(rng.Intn(5))
			var pipMask uint8
			for p := 0; p < 1+rng.Intn(4); p++ {
				c := rng.Intn(5)
				cost.Pips[c] += uint8(1 + rng.Intn(3))
				pipMask |= 1 << c
			}
			extra := rng.Intn(2)
			for _, n := range ns {
				a.addUnits(uint8(kop.ColAny), n)
				a.one[a.nOne] = n
				a.nOne++
			}
			b := a
			for _, n := range ns {
				b.remove(uint8(kop.ColAny), n)
			}
			if b.nOne != 0 {
				t.Fatalf("ns %v: %d one-colour sources left after removing them all", ns, b.nOne)
			}
			want := b.oneColourFitsExhaustive(&cost, pipMask, extra, ns)
			if got := b.oneColourFits(&cost, pipMask, extra, ns); got != want {
				t.Fatalf("ns %v cost %+v extra %d: symmetric %v, exhaustive %v", ns, cost, extra, got, want)
			}
			if got := a.oneColourPays(&cost, pipMask, extra); got != want {
				t.Fatalf("ns %v cost %+v extra %d: oneColourPays %v, exhaustive %v", ns, cost, extra, got, want)
			}
			if want {
				fits++
			} else {
				misses++
			}
		}
	}
	eachAmountMultiset(6, check)
	for n := uint8(1); n <= 4; n++ {
		check([]uint8{n, n, n, n, n, n, n, n})
	}
	if fits < lists || misses < lists {
		t.Fatalf("%d fits, %d misses over %d lists: the costs do not exercise both answers", fits, misses, lists)
	}
}
