package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckActivationHeldOut is the kernel-side half of
// TestActivationHeldOutBesideNoWaterbend (dirac_test): on a table with
// neither a waterbend card nor an X-bound ability, an activation applied and
// reversed with no progress twice in a row holds the card's every activation
// out of the window (CR 733.2, gorge's castSuppressed consult at
// legal_walk_battlefield.go). The kernel read castHeldOut beside those two
// families only, so it offered an activation gorge withheld (tail-53 pair
// 568, decision 831: kernel [pass activate(#68{Lithoform Engine},-,2)],
// gorge [pass]).
func CheckActivationHeldOut(t *testing.T, tbl *kop.Table, le, den, mountain kop.CardID) {
	t.Helper()
	sc := newScenTableLib(t, tbl, mountain, Config{})
	engine := sc.perm(le, 0)
	for i := 0; i < 6; i++ {
		sc.perm(mountain, 0)
	}
	s, k := sc.s, sc.k

	// A permanent spell on the stack gives the engine's third ability
	// ({4}, {T}: copy target permanent spell) a target, so the option list
	// exposes exactly the hold-out and nothing else. The cast runs to
	// completion under elision; the activation asks below read elided
	// decisions back (NoElide).
	spell := sc.hand(den, 0)
	sc.do(t, AkCast, TCode(spell), scAny)
	for i := 0; i < 50 && s.ctl != ctlPriority; i++ {
		s.Step()
	}
	if s.ctl != ctlPriority {
		t.Fatal("the cast never came back to priority")
	}
	s.cfg |= cfgNoElide

	// Two applied-and-reversed activations of the engine's third ability, no
	// progress between: the push's own touches are no progress (cast.go's
	// arm) and reverseCast is the reversal every failing ask runs (gorge
	// abortCast, CR 733.2). The target ask is left unanswered, the way a
	// payment that can no longer be made reverses a pushed activation.
	for i := 0; i < 2; i++ {
		s.ctl = ctlBuild
		s.beginActivate(k, engine, 2)
		d := s.Step()
		if d.Kind != DkTarget {
			t.Fatalf("reversal %d: want the target ask, got kind %d opts %v", i+1, d.Kind, d.Options[:d.N])
		}
		s.reverseCast(k)
		if s.ctl != ctlPriority {
			t.Fatalf("reversal %d: the build survived (ctl %d)", i+1, s.ctl)
		}
	}
	if !s.castHeldOut(engine) {
		t.Fatal("two no-progress reversals did not hold the engine's activations out")
	}
	d := s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkActivate && a.Source() == TCode(engine) {
			t.Fatalf("the held-out activation is still offered: %v", d.Options[:d.N])
		}
	}
}
