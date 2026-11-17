package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// signetScen: p0 has an Azorius Signet ("{1}, {T}: Add {W}{U}", an all-at-once
// filter), built from Abzan Devotee's card with its ability swapped, and pool
// floating.
func signetScen(t *testing.T, pool [kop.NumManaKinds]uint8) (sc *scen, sig Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Abzan Devotee", Count: 1}, {Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}}
	tbl, da, _, err := cardsettest.Set(t, "manaabmanacost").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tb := *tbl
	tb.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	dv := &tb.Cards[id["Abzan Devotee"]]
	dv.Abilities = append([]kop.Ability(nil), dv.Abilities...)
	for i := range dv.Abilities {
		if ab := &dv.Abilities[i]; ab.ManaColors != 0 {
			ab.ManaColors = kop.ColorMask(1<<kop.W | 1<<kop.U)
			ab.ManaN, ab.ManaEach, ab.AFlags = 1, true, 0
			ab.Cost.Flags = kop.CostTap
		}
	}
	sc = newScenTableLib(t, &tb, id["Plains"], Config{NoElide: true})
	sig = sc.perm(id["Abzan Devotee"], 0)
	sc.s.pl[0].pool = pool
	return sc, sig
}

// TestFaceReachFilterFeePaysAUnit: faceReachable pays a filter's fee out of
// mana the window already made (CR 605.3b), so the unit the fee spends pays no
// pip. {B}{B} floating beside a {1} -> {W}{U} filter reaches {B}{B}{W/U}
// only if the fee were charged to the total alone; the real fee takes a black,
// leaving one black for two black pips. A spare {G} takes the fee instead.
func TestFaceReachFilterFeePaysAUnit(t *testing.T) {
	var face kop.ManaCost
	face.Pips[kop.B] = 2
	face.Pips[kop.W] = 1 // {B}{B}{W/U}, resolved to its white face
	for _, c := range []struct {
		pool [kop.NumManaKinds]uint8
		want bool
	}{
		{[kop.NumManaKinds]uint8{kop.B: 2}, false},
		{[kop.NumManaKinds]uint8{kop.B: 2, kop.G: 1}, true},
	} {
		sc, sig := signetScen(t, c.pool)
		s, k := sc.s, sc.k
		sl := sc.slot(sig)
		if !s.scriptFilt(k, 0).has(sl) || s.filtFee(k, sl) != 1 {
			t.Fatalf("pool %v: no {1} -> {W}{U} filter on the board", c.pool)
		}
		var av avail
		s.availFor(k, 0, &av)
		if av.affords(&face, pipMaskOf(&face), 0) {
			t.Fatalf("pool %v: the face is payable without the filter", c.pool)
		}
		if got := s.faceReachable(k, 0, &av, &face, 0); got != c.want {
			t.Errorf("pool %v: faceReachable %v, want %v", c.pool, got, c.want)
		}
	}
}
