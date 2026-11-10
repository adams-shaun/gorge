package dirac

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestXAnnouncementCountsARiderSource: Twitching Doll's "{T}: Add one mana of
// any color. Put a nest counter on this" is a counter-rider mana ability, so
// the payment planner defers it and its card is no census unit; CR 601.2g's
// window can still tap it, and gorge's xAsk reads that window
// (castWindowUnits). Valgavoth's Onslaught ({X}{X}{G}) is therefore offered
// X = 2 from four floating mana plus the Doll, where the same pool alone pays
// only X = 1 (mana_rider.go, mana_tapsdefer.go).
func TestXAnnouncementCountsARiderSource(t *testing.T) {
	deck := []cardset.Entry{{Name: "Valgavoth's Onslaught", Count: 1}, {Name: "Twitching Doll", Count: 1}}
	// four floating mana pay X = 1's {1}{1}{G}; the fifth unit, the Doll's
	// any-colour mana, is what makes X = 2's five payable.
	choices := func(t *testing.T, withDoll bool) []int {
		t.Helper()
		sc, id := xScen(t, "std-dsk", deck)
		if withDoll {
			doll := sc.perm(id["Twitching Doll"], 0)
			// The preconditions the assertion depends on: the Doll is the
			// counter-rider shape, untapped where the window looks, and
			// deferred out of the planner census into the prefix gap.
			if _, _, ok := cardManaRider(&sc.k.t.Cards[id["Twitching Doll"]]); !ok {
				t.Fatal("precondition: Twitching Doll's mana ability is not the counter-rider shape")
			}
			if sc.tapped(doll) {
				t.Fatal("precondition: Twitching Doll is tapped")
			}
			if !sc.s.planDeferred(sc.k, 0, sc.slot(doll)) || !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(doll)) {
				t.Fatal("precondition: the Doll is not a deferred source and prefix gap")
			}
		}
		sc.s.pl[0].pool[kop.W] = 2
		sc.s.pl[0].pool[kop.G] = 1
		sc.s.pl[0].pool[kop.B] = 1
		vo := sc.hand(id["Valgavoth's Onslaught"], 0)
		sc.do(t, AkCast, uint8(vo), 0xff)
		d := sc.s.Step()
		if d.Kind != DkChoose || d.Sub != ChX {
			t.Fatalf("decision kind %d sub %d, want the X ask", d.Kind, d.Sub)
		}
		var xs []int
		for i := 0; i < int(d.N); i++ {
			xs = append(xs, int(d.Options[i].Mode()))
		}
		return xs
	}
	// Without the Doll the pool alone pays only X = 1, so the extra value
	// comes from the Doll and not from the cost.
	if xs := fmt.Sprint(choices(t, false)); xs != "[0 1]" {
		t.Fatalf("precondition: X options with the pool alone %s, want [0 1]", xs)
	}
	if xs := fmt.Sprint(choices(t, true)); xs != "[0 1 2]" {
		t.Fatalf("X options %s, want [0 1 2]", xs)
	}
}
