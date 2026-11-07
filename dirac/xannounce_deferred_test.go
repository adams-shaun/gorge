package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestXAnnouncementCountsADeferredSource: Pyromancer's Goggles' {T}: Add {R}
// carries TriggersWhenSpent$, so the payment planner defers it and its card is
// no census unit; CR 601.2g's window can still tap it, and gorge's xAsk reads
// that window. Steel Hellkite's {X} is therefore offered for 1 with the Goggles
// as the only untapped source, and answering 1 taps it and puts the ability on
// the stack (mana_deferred.go).
func TestXAnnouncementCountsADeferredSource(t *testing.T) {
	sc, id := xScen(t, "std-fdn", []cardset.Entry{{Name: "Steel Hellkite", Count: 1}, {Name: "Pyromancer's Goggles", Count: 1}})
	hk := sc.perm(id["Steel Hellkite"], 0)
	g := sc.perm(id["Pyromancer's Goggles"], 0)
	// The precondition the assertion depends on: the Goggles is the only
	// source there is, and it is untapped where the window looks.
	if sc.tapped(g) {
		t.Fatal("precondition: Pyromancer's Goggles is tapped")
	}
	// Without a source the same ability offers only X = 0, so the 1 comes
	// from the Goggles and not from the ability's cost.
	bare, bid := xScen(t, "std-fdn", []cardset.Entry{{Name: "Steel Hellkite", Count: 1}})
	if xs := fmt.Sprint(bare.xChoices(t, bare.perm(bid["Steel Hellkite"], 0), 1)); xs != "[0]" {
		t.Fatalf("precondition: X options with no source %s, want [0]", xs)
	}
	if xs := fmt.Sprint(sc.xChoices(t, hk, 1)); xs != "[0 1]" {
		t.Fatalf("X options %s, want [0 1]", xs)
	}
	// X = 1 is payable: the deferred source taps for its {R} and the
	// activation reaches the stack instead of reversing for want of mana.
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 1))
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("after X = 1 decision kind %d, want priority with the ability on the stack", d.Kind)
	}
	if !sc.tapped(g) {
		t.Fatal("X = 1 paid without tapping Pyromancer's Goggles")
	}
}
