package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestVizierConversionPricesKickedCast pins the offer gate of a kicked cast
// under Vizier of the Menagerie's conversion (CR 609.4b, 702.33): Gatekeeper
// of Malakir's kicked {B}{B}{B} is paid by three Forests, so the kicked cast
// is offered beside the plain one. Without the Vizier it is not, and a
// noncreature kicker spell (Into the Roil's kicked {2}{U}{U} from four
// Forests) gets no conversion.
func TestVizierConversionPricesKickedCast(t *testing.T) {
	const vizier, gatekeeper, roil = "Vizier of the Menagerie", "Gatekeeper of Malakir", "Into the Roil"
	offers := func(t *testing.T, withVizier bool, spell string, forests int) (plain, kicked bool) {
		t.Helper()
		sc, id := synthScen(t, "std-fdn", vizier, spell, "Forest")
		for _, n := range []string{vizier, spell, "Forest"} {
			if _, ok := id[n]; !ok {
				t.Fatalf("%s not in the std-fdn pool (run make dirac-tables)", n)
			}
		}
		if sc.k.cards[id[spell]].kw&kop.KwKicker == 0 {
			t.Fatalf("precondition: %s has no kicker", spell)
		}
		if withVizier {
			if vz := sc.perm(id[vizier], 0); !sc.onBF(vz) {
				t.Fatal("precondition: Vizier of the Menagerie is not on the battlefield")
			}
		}
		for i := 0; i < forests; i++ {
			sc.perm(id["Forest"], 0)
		}
		o := sc.hand(id[spell], 0)
		var av avail
		sc.s.availFor(sc.k, 0, &av)
		if av.total != forests {
			t.Fatalf("precondition: %d mana available, want %d", av.total, forests)
		}
		d := sc.s.Step()
		if d.Kind != DkPriority || d.Player != 0 {
			t.Fatalf("precondition: decision %v for p%d, want p0's priority", d.Kind, d.Player)
		}
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) {
				switch a.Mode() {
				case 0:
					plain = true
				case 1:
					kicked = true
				}
			}
		}
		return plain, kicked
	}
	t.Run("vizier", func(t *testing.T) {
		plain, kicked := offers(t, true, gatekeeper, 3)
		if !plain {
			t.Fatal("precondition: the plain cast of Gatekeeper of Malakir is not offered under Vizier")
		}
		if !kicked {
			t.Fatal("the kicked Gatekeeper of Malakir ({B}{B}{B}) is not offered from three Forests under Vizier's conversion")
		}
	})
	t.Run("no vizier", func(t *testing.T) {
		if plain, kicked := offers(t, false, gatekeeper, 3); plain || kicked {
			t.Fatalf("Gatekeeper of Malakir offered from three Forests without Vizier: plain=%v kicked=%v", plain, kicked)
		}
	})
	t.Run("noncreature", func(t *testing.T) {
		if _, kicked := offers(t, true, roil, 4); kicked {
			t.Fatal("the kicked Into the Roil ({2}{U}{U}) is offered from four Forests: the conversion is for creature spells only")
		}
	})
}
