package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPowerUpPipFacesPriceReducedCharge: Abomination, Terrifying Titan entered
// this turn owes {2}{R/G}{R/G} for its {5}{R/G}{R/G} Power-up (CR 702.193b).
// The hybrid pip ask prices each face against that reduced charge, as gorge's
// manaAsk reads the activation's reduced cost (CR 601.2b): with {R}{R}{R}{R}
// floating and a Forest both {R} and {G} pay it, though neither face pays the
// printed cost from five mana.
func TestPowerUpPipFacesPriceReducedCharge(t *testing.T) {
	sc, id := powerupScen(t, "Abomination, Terrifying Titan", "Forest")
	a := sc.perm(id["Abomination, Terrifying Titan"], 0)
	sc.perm(id["Forest"], 0)
	sc.s.pl[0].pool[kop.R] = 4 // the pool pays the reduced charge, so the power-up is offered
	if !sc.s.g.zoneTurn.has(a) {
		t.Fatal("precondition: Abomination did not enter this turn, so no CR 702.193b reduction applies")
	}
	d := sc.s.Step()
	if !powerupActivation(&d, a) {
		t.Fatalf("the reduced hybrid power-up was not offered: %v", d.Options[:d.N])
	}
	sc.s.Apply(MakeAction(AkActivate, TCode(a), 0, 0))
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenPip {
		t.Fatalf("decision after activate %v/%d, want the hybrid pip ask", d.Kind, d.Sub)
	}
	var faces []uint8
	for i := 0; i < int(d.N); i++ {
		faces = append(faces, d.Options[i].Mode())
	}
	if len(faces) != 2 || faces[0] != uint8(kop.R) || faces[1] != uint8(kop.G) {
		t.Fatalf("the first {R/G} pip offered faces %v, want [R G] (%d %d)", faces, kop.R, kop.G)
	}
}
