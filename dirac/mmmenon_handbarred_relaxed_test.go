package dirac

import "testing"

// mmmenonHandBarredBoard seats the board of std-eoe pair 2041: Mm'menon, the
// Right Hand grants every artifact "{T}: Add {U}" for a spell cast from
// elsewhere than the hand; Gene Pollinator taps an untapped permanent for one
// mana of any colour; Nanoform Sentinel, tapped for that cost, untaps a land;
// every land is tapped. Atomic Microsizer is the equipment (Equip {2}).
func mmmenonHandBarredBoard(t *testing.T) (sc *scen, equip Action) {
	t.Helper()
	sc, id := synthScen(t, "std-eoe", "Atomic Microsizer", "Gene Pollinator", "Mm'menon, the Right Hand", "Nanoform Sentinel", "Edge Rover", "Island")
	for i := 0; i < 2; i++ {
		sc.s.perms[sc.slot(sc.perm(id["Island"], 0))].flags |= pfTapped
	}
	sc.perm(id["Mm'menon, the Right Hand"], 0)
	sc.perm(id["Gene Pollinator"], 0)
	sc.perm(id["Nanoform Sentinel"], 0)
	eq := sc.perm(id["Atomic Microsizer"], 0)
	return sc, MakeAction(AkActivate, TCode(eq), 0, 0)
}

// TestRelaxedProofCountsTapAnotherUnitAndRefusesHandBarredGrant: Equip {2}
// beside Mm'menon's hand-barred granted blue with every land tapped. The
// tap-another search alone finds a line (Gene Pollinator taps Nanoform
// Sentinel, whose trigger untaps a land), but gorge's relaxed proof counts one
// free unit for the Pollinator and none for the grant an activation may not
// spend (CR 106.6, 605.3b), so the equip is proven insufficient and never
// searched: it is not offered (std-eoe pair 2041, decision 981).
func TestRelaxedProofCountsTapAnotherUnitAndRefusesHandBarredGrant(t *testing.T) {
	sc, equip := mmmenonHandBarredBoard(t)
	if !sc.s.tapXScript(sc.k, equip, 0, false) {
		t.Fatal("precondition: the tap-another search finds no line for the equip")
	}
	d := sc.s.Step()
	if hasOption(&d, equip) {
		t.Fatalf("Equip {2} offered with every land tapped: %v", d.Options[:d.N])
	}
	// the relaxed proof's two sides: the grant is no unit of an activation,
	// the Pollinator is one (with a land untapped the two pay {2})
	cost, ex, _, ok := sc.s.grantPlayCost(sc.k, 0, equip, true)
	if !ok {
		t.Fatal("precondition: the equip has no mana cost")
	}
	if sc.s.grantRelaxedAffords(sc.k, 0, equip, &cost, ex, sc.s.grantPayDesc(sc.k, equip), availWalk) {
		t.Fatal("the relaxed proof pays Equip {2} from one tap-another unit alone")
	}
}
