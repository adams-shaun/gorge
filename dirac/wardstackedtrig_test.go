package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestStackedWardTriggeredDoublesLeaveSameStack covers the same-source pending
// pair the review measured: Twinflame Travelers doubles Rimekin Recluse's
// enters-the-battlefield trigger, and both copies target Bristlebane Battler
// (Ward {2}). Both copies are on the stack before either ward trigger is
// placed, because the ward triggers belong to the non-active player (APNAP).
// Each ward trigger must answer the copy that targeted the Battler: a stack
// re-scan at placement stamps both with the second copy's identity, the first
// copy's ward asks nothing and leaves, and that copy resolves, returning the
// Battler to its owner's hand. With the identity captured when the trigger is
// queued, both ward triggers ask and both copies are countered, as gorge does.
func TestStackedWardTriggeredDoublesLeaveSameStack(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Twinflame Travelers", Count: 1},
		{Name: "Rimekin Recluse", Count: 1},
		{Name: "Bristlebane Battler", Count: 1},
		{Name: "Island", Count: 4},
		{Name: "Mountain", Count: 4},
		{Name: "Forest", Count: 4},
	}
	sc, id := xScen(t, "std-ecl", deck)
	s := sc.s
	sc.perm(id["Twinflame Travelers"], 0)
	battler := sc.perm(id["Bristlebane Battler"], 1)
	for i := 0; i < 4; i++ {
		sc.perm(id["Island"], 0)
		sc.perm(id["Mountain"], 0)
		sc.perm(id["Forest"], 0)
	}
	recluse := sc.hand(id["Rimekin Recluse"], 0)
	s.active, s.prio, s.passes, s.ctl = 0, 0, 0, ctlPriority
	sc.do(t, AkCast, TCode(recluse), scAny)
	var asks, sizes []int
	targets := 0
	cast := false
	for i := 0; i < 200 && !(cast && targets == 2 && len(asks) == 2 && s.sp == 0 && s.npend == 0); i++ {
		d := s.Step()
		switch {
		case d.Kind == DkNone:
			i = 200 // no more decisions: fall through to the assertions
		case d.Kind == DkTarget:
			chosen := false
			for j := 0; j < int(d.N); j++ {
				if a := d.Options[j]; a.Target() == TCode(uint8(battler)) {
					s.Apply(a)
					targets++
					chosen = true
					break
				}
			}
			if !chosen {
				t.Fatalf("target %d not offered for the doubled enters trigger: %v", battler, d.Options[:d.N])
			}
		case d.Kind == DkChoose && d.Sub == ChWardPay:
			asks = append(asks, int(s.sp))
			sizes = append(sizes, int(s.sp))
			s.Apply(d.Options[0]) // decline: the copy this trigger answers is countered
		case d.Kind == DkPriority:
			if !cast {
				cast = true
			}
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		default:
			s.Apply(d.Options[0])
		}
	}
	if !cast {
		t.Fatal("Rimekin Recluse was never cast")
	}
	if targets != 2 {
		t.Fatalf("the enters trigger was doubled to %d targets, want 2 (Twinflame Travelers)", targets)
	}
	if len(asks) != 2 {
		t.Fatalf("ward payment asked %v (stack %v), want two asks: each ward trigger answers its own copy", asks, sizes)
	}
	if !sc.onBF(battler) {
		t.Fatalf("Bristlebane Battler left the battlefield (%d): one copy's ward found nothing and that copy resolved", s.objs[battler].zone)
	}
}
