package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestGrantedExaltedFiresPerInstance: Sublime Archangel's static gives the
// other creatures its controller has exalted (CR 702.83a-b, 113.2c). With
// the Archangel (printed exalted), a Squire (printed, plus the granted one)
// and two Bears on the battlefield, a lone Bear attacking triggers five
// instances: the Archangel's printed one, the Squire's printed and granted
// ones, and each Bear's granted one (a holder need not attack); two
// attackers trigger none.
func TestGrantedExaltedFiresPerInstance(t *testing.T) {
	sc, id := newSetScenTweak(t, "exaltedgrant", []cardset.Entry{
		{Name: "Sublime Archangel", Count: 2}, {Name: "Akrasan Squire", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Plains", Count: 8},
	}, nil)
	sc.perm(id["Sublime Archangel"], 0)
	sc.perm(id["Akrasan Squire"], 0)
	bear, bear2 := sc.perm(id["Grizzly Bears"], 0), sc.perm(id["Grizzly Bears"], 0)
	declare := func(objs ...Obj) int {
		for _, o := range []Obj{bear, bear2} {
			sc.s.perms[sc.slot(o)].flags &^= pfAttacking
		}
		for _, o := range objs {
			p := &sc.s.perms[sc.slot(o)]
			p.flags |= pfAttacking
			p.atk = PlayerTarget1
		}
		sc.s.nAttackers = uint8(len(objs))
		sc.s.npend = 0
		sc.s.fireAttacksGen(sc.k)
		return int(sc.s.npend)
	}
	if n := declare(bear); n != 5 {
		t.Fatalf("one attacker alone: %d exalted triggers, want 5", n)
	}
	if n := declare(bear, bear2); n != 0 {
		t.Fatalf("two attackers: %d exalted triggers, want 0", n)
	}
}
