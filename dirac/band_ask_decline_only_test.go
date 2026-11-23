package dirac

import "testing"

// TestBandAskDeclineOnly: the CR 702.22c band ask's answers (dirac/combat_band.go).
// The kernel poses gorge's ask so the granted banding (kop.Override.Banding) is
// observable: the decline is applied without a stall and hands the attackers
// step on, and an answer naming a candidate (an accepted band) joins the asked
// leader's band without one — TestBandFormationLockstep plays it out.
func TestBandAskDeclineOnly(t *testing.T) {
	t.Run("an accepted candidate joins the band without a stall", func(t *testing.T) {
		f, rams := bandAskScen(t, 1)
		d := f.expect(t, DkChoose, ChBand)
		cand := bandOpt(t, d, 1)
		if cand.Source() == TCode(NoObj) {
			t.Fatal("precondition: the ask offers no candidate beside the decline")
		}
		f.s.Apply(cand)
		id := uint8(f.slot(rams[0])) + 1
		if f.s.bandOf(f.slot(rams[0])) != id || f.s.bandOf(f.slot(rams[1])) != id {
			t.Fatalf("bands %d and %d, want both %d", f.s.bandOf(f.slot(rams[0])), f.s.bandOf(f.slot(rams[1])), id)
		}
		if f.s.g.stalled != 0 {
			t.Fatalf("an accepted band stalled %d (%s), want none", f.s.g.stalled, StallName(f.s.g.stalled))
		}
		f.expect(t, DkPriority, 0) // the leader has no candidate left, so it is not re-asked
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
