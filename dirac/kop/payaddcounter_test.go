package kop

import "testing"

// TestPayAddCounterBRoundTrips: the packed B.N of an unless AddCounter keeps
// every counter kind and the three recipient classes distinct, never reads as
// a Waterbend, and refuses a recipient it cannot carry.
func TestPayAddCounterBRoundTrips(t *testing.T) {
	for _, r := range []Ref{RefSelf, RefTarget0, RefTarget1, RefTarget2} {
		for k := 0; k < 256; k++ {
			n, ok := PayAddCounterB(CounterKind(k), r)
			if !ok || n < 0 || n > 32767 {
				t.Fatalf("kind %d ref %d: packed %d ok %v", k, r, n, ok)
			}
			if !IsPayAddCounterB(int16(n)) || int16(n) == PayBSubWaterbend {
				t.Fatalf("kind %d ref %d: %d is not recognised as an AddCounter encoding", k, r, n)
			}
			gk, gr := PayAddCounterFrom(int16(n))
			if gk != CounterKind(k) || gr != r {
				t.Fatalf("kind %d ref %d: unpacked kind %d ref %d", k, r, gk, gr)
			}
		}
	}
	for _, r := range []Ref{RefNone, RefController, RefTriggerObj, RefEach} {
		if _, ok := PayAddCounterB(CtrStun, r); ok {
			t.Errorf("ref %d packed; only the source and targets 0..2 are carried", r)
		}
	}
	for _, n := range []int16{0, PayBSubWaterbend, PayBSubBlight, PayBBehold} {
		if IsPayAddCounterB(n) {
			t.Errorf("B.N %d reads as an AddCounter encoding", n)
		}
	}
}
