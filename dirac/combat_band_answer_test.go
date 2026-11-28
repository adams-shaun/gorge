package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// bandThreeScen declares three Test Rams attacking, the first n of them
// granted banding (the CR 702.22c leader needs a candidate beside it).
func bandThreeScen(t *testing.T, banding int) (*kwBandingScen, [3]Obj) {
	t.Helper()
	f := newKwBandingScen(t)
	var rams [3]Obj
	for i := range rams {
		rams[i] = f.perm(f.ram, 0)
	}
	for i := 0; i < banding; i++ {
		f.s.applyOverrideFl(f.k, f.slot(rams[i]), f.band, f.ram, f.s.ovDurFl(kop.DurPermanent))
	}
	f.passUntil(t, kop.StepAttackers)
	for _, o := range rams {
		f.do(t, AkAttack, TCode(o), scAny)
	}
	f.do(t, AkDone, scAny, scAny)
	return f, rams
}

// TestBandAskAnswers: the answers to the CR 702.22c band declaration ask
// (dirac/combat_band.go). An accepted candidate forms the band and re-asks
// the same leader, so a band grows one creature per answer; a decline moves
// to the next leader. Once a band holds a creature without banding, only
// banding creatures may join it (gorge bandCandidates).
func TestBandAskAnswers(t *testing.T) {
	t.Run("a candidate re-asks the same leader", func(t *testing.T) {
		f, rams := bandThreeScen(t, 3)
		d := f.expect(t, DkChoose, ChBand)
		if f.s.lastPick != f.slot(rams[0]) {
			t.Fatalf("first leader slot %d, want %d", f.s.lastPick, f.slot(rams[0]))
		}
		f.s.Apply(bandOpt(t, d, 1)) // ram 1 joins
		d = f.expect(t, DkChoose, ChBand)
		if f.s.lastPick != f.slot(rams[0]) {
			t.Fatalf("the re-ask names leader slot %d, want the same leader %d", f.s.lastPick, f.slot(rams[0]))
		}
		if bandOpt(t, d, 1).Source() != TCode(rams[2]) {
			t.Fatalf("the re-ask's candidate is not ram 2: %v", d.Options[:d.N])
		}
		leader := f.slot(rams[0])
		f.s.Apply(bandOpt(t, d, 1)) // ram 2 joins
		if f.s.g.stalled != 0 {
			t.Fatalf("the second accepted candidate stalled %d (%s)", f.s.g.stalled, StallName(f.s.g.stalled))
		}
		for i, o := range rams {
			if got := f.s.bandOf(f.slot(o)); got != leader {
				t.Fatalf("ram %d's band names slot %d, want the leader %d", i, got, leader)
			}
		}
		f.expect(t, DkPriority, 0)
	})
	t.Run("a decline moves to the next leader", func(t *testing.T) {
		f, rams := bandAskScen(t, 2)
		d := f.expect(t, DkChoose, ChBand)
		f.s.Apply(bandOpt(t, d, 0))
		d = f.expect(t, DkChoose, ChBand)
		if f.s.lastPick != f.slot(rams[1]) {
			t.Fatalf("after the decline the leader is slot %d, want %d", f.s.lastPick, f.slot(rams[1]))
		}
		f.s.Apply(bandOpt(t, d, 0))
		f.expect(t, DkPriority, 0)
	})
	t.Run("a plain member closes the band to plain creatures", func(t *testing.T) {
		f, rams := bandThreeScen(t, 1) // only ram 0 has banding
		d := f.expect(t, DkChoose, ChBand)
		f.s.Apply(bandOpt(t, d, 1)) // ram 1, a plain creature, joins
		if f.s.g.stalled != 0 {
			t.Fatalf("an accepted plain member stalled %d (%s)", f.s.g.stalled, StallName(f.s.g.stalled))
		}
		// No re-ask: ram 2 is plain and the band already holds a plain member.
		f.expect(t, DkPriority, 0)
		leader := f.slot(rams[0])
		if got := f.s.bandOf(f.slot(rams[2])); got != NoSlot {
			t.Fatalf("ram 2 joined the band (record %d)", got)
		}
		if f.s.bandOf(leader) != leader {
			t.Fatal("the leader's own band record is missing")
		}
	})
}
