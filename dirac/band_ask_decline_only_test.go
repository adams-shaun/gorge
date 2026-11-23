package dirac

import "testing"

// TestBandAskDeclineOnly: the CR 702.22c band ask's decline-only invariant
// (dirac/combat_band.go). The kernel poses gorge's ask so the granted banding
// (kop.Override.Banding) is observable, never so a band forms: the decline is
// the only answer it applies — an answer naming a candidate (an accepted
// band) reports the named banding stall, and the decline hands the attackers
// step on without one.
func TestBandAskDeclineOnly(t *testing.T) {
	t.Run("a candidate reports the named banding stall", func(t *testing.T) {
		f, _ := bandAskScen(t, 1)
		d := f.expect(t, DkChoose, ChBand)
		cand := bandOpt(t, d, 1)
		if cand.Source() == TCode(NoObj) {
			t.Fatal("precondition: the ask offers no candidate beside the decline")
		}
		f.s.Apply(cand)
		if f.s.g.stalled != StallBand {
			t.Fatalf("stalled %d (%s), want StallBand", f.s.g.stalled, StallName(f.s.g.stalled))
		}
	})
	t.Run("the decline is applied without a stall", func(t *testing.T) {
		f, _ := bandAskScen(t, 1)
		d := f.expect(t, DkChoose, ChBand)
		decl := bandOpt(t, d, 0)
		if decl.Source() != TCode(NoObj) {
			t.Fatalf("option 0 names source %d, want the decline", decl.Source())
		}
		f.s.Apply(decl)
		if f.s.g.stalled != 0 {
			t.Fatalf("a declined band ask stalled %d (%s), want none", f.s.g.stalled, StallName(f.s.g.stalled))
		}
		f.expect(t, DkPriority, 0)
	})
}
