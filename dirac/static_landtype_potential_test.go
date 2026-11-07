package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBasicLandGrantPotential: a trigger's own mana cost is offered by gorge's
// potential over-bound, which counts a colour per mana ability and a source's
// granted basic land types give it one each (CR 305.6). A Plains under Leyline
// of the Guildpact reaches {W}, {U}, {B}, {R} and {G} in the potential, so
// Krovod Haunch's {1}{W} is offered with that one untapped land; without the
// grant the Plains' potential is {W} alone.
func TestBasicLandGrantPotential(t *testing.T) {
	sc, _ := blgScen(t, "Leyline of the Guildpact", "Plains")
	var av avail
	other := payDesc{class: pdOther}
	sc.s.availPotentialFor(sc.k, 0, &av, &other)
	if av.total != 5 {
		t.Fatalf("potential units %d, want 5: a Plains under Leyline counts each granted colour", av.total)
	}
	c := cost(1, kop.W)
	if !av.affords(&c, pipMaskOf(&c), 0) {
		t.Error("the over-bound does not reach {1}{W} with a Leyline-granted Plains")
	}

	sc, _ = blgScen(t, "Plains")
	av = avail{}
	sc.s.availPotentialFor(sc.k, 0, &av, &other)
	if av.total != 1 {
		t.Fatalf("potential units %d, want 1 for a Plains without the grant", av.total)
	}
}
