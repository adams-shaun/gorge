package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapxPrefixFloatFloatsCastPipBeforeTarget: the tapX prefix search floats
// the coloured share of a tapX-funded CAST before the cast's target ask, as it
// does for an activation (payNeededColours, mana_resolutionpay.go). Board:
// Jaspera Sentinel's "{T}, tap an untapped creature you control: Add one mana
// of any color", two Swamps and Nirkana Cutthroat (the creature to tap), and
// Hero's Downfall ({1}{B}{B}) in hand. The plain census (two Swamps) cannot
// price the cast, so the prefix search taps the Sentinel for one {B} and stops
// with the third mana in the census. gorge's search floats the second {B}
// from a Swamp before the target ask; the kernel's cast arm called
// payCastPrefixRest without the tapX flag, so the second {B} waited for the
// post-push payment and the target ask saw only one floating black.
func TestTapxPrefixFloatFloatsCastPipBeforeTarget(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Hero's Downfall", Count: 1},
		{Name: "Jaspera Sentinel", Count: 1},
		{Name: "Nirkana Cutthroat", Count: 1},
		{Name: "Swamp", Count: 2},
		{Name: "Plains", Count: 1},
	}
	sc, id := newSetScenTweak(t, "mbc", deck, nil)
	s := sc.s
	jaspera := sc.perm(id["Jaspera Sentinel"], 0)
	cutthroat := sc.perm(id["Nirkana Cutthroat"], 0)
	var swamps []Slot
	for i := 0; i < 2; i++ {
		swamps = append(swamps, sc.slot(sc.perm(id["Swamp"], 0)))
	}
	downfall := sc.hand(id["Hero's Downfall"], 0)
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	d := s.Step()
	cast, ok := castAction(&d, downfall)
	if !ok {
		t.Fatalf("Hero's Downfall not offered by the tapX prefix; opts %v", d.Options[:d.N])
	}
	s.Apply(cast)
	d = s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("after the cast the decision is kind %d, want the destroy spell's target ask", d.Kind)
	}
	// The prefix search spent the Sentinel's any-colour unit on the first {B}
	// (it taps the Sentinel and its candidate), so the pre-ask board has one
	// black floating and both Swamps untapped until the fix floats the second.
	if !sc.tapped(jaspera) || !sc.tapped(cutthroat) {
		t.Fatal("precondition: the tapX prefix did not tap the Sentinel and its candidate")
	}
	if got := s.pl[0].pool[kop.B]; got != 2 {
		t.Fatalf("target ask sees %d black floating, want 2: the cast's needed-colour share is not floated before the ask", got)
	}
	tapped := 0
	for _, sl := range swamps {
		if s.perms[sl].flags&pfTapped != 0 {
			tapped++
		}
	}
	if tapped != 1 {
		t.Fatalf("%d of the two Swamps are tapped at the target ask, want 1 (the floated second black)", tapped)
	}
}

// castAction returns the cast option for o, if the decision offers one.
func castAction(d *Decision, o Obj) (Action, bool) {
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(o) {
			return a, true
		}
	}
	return 0, false
}
