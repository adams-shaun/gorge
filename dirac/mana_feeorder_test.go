package dirac

import "testing"

// A fee-carrying filter beside its own free tap is funded before the free tap
// is taken. Baxter Building's {T}: Add {C} is the free ability and its
// {4}, {T}: Add four mana in any combination of colors the filter
// (CR 605.3b, CR 601.2g-h): Earth's Mightiest Heroes' {4}{G}{G} has only one
// green source without it, so the cast is offered only when the search funds
// the filter's fee first. The plain order's gap pass taps the free ability and
// loses the filter; potentialPrefixScript's feeOrder order funds first.
func TestFilterFeeOrderFundsBeforeFreeAbility(t *testing.T) {
	sc, id := stdMshScen(t, "Earth's Mightiest Heroes", "Baxter Building", "Plains", "Forest", "Hidden Lair", "Hercules, Prince of Power")
	sc.perm(id["Forest"], 0)
	for i := 0; i < 4; i++ {
		sc.perm(id["Plains"], 0)
	}
	sc.perm(id["Hidden Lair"], 0)
	bax := sc.perm(id["Baxter Building"], 0)
	sc.perm(id["Hercules, Prince of Power"], 0)
	emh := sc.hand(id["Earth's Mightiest Heroes"], 0)
	k, p := sc.k, uint8(0)
	bsl := sc.s.objs[bax].slot
	// The precondition the offer depends on: the source is a free tap beside
	// a fee-carrying combination filter (mana_filter.go filterTapMana).
	if !sc.s.canTapForMana(k, bsl) || sc.s.filtFee(k, bsl) == 0 || !comboFiltAb(sc.s.filtAb(k, bsl)) {
		t.Fatalf("Baxter Building is not a free tap beside a fee-carrying combination filter")
	}
	// And the cast is not payable from the printed sources, so it is the
	// filter that funds it (the object sits where the search reads it).
	c := id["Earth's Mightiest Heroes"]
	var av avail
	sc.s.availFor(k, p, &av)
	if sc.s.castGateOrPoolPays(k, p, c, emh, &k.t.Cards[c].Cost, &av) {
		t.Fatalf("the cast is affordable from the printed sources; the filter is not the source under test")
	}
	d := sc.s.Step()
	if want := MakeAction(AkCast, TCode(emh), 0, 0); !hasOption(&d, want) {
		t.Fatalf("Earth's Mightiest Heroes not offered through Baxter Building's filter: %v", d.Options[:d.N])
	}
}
