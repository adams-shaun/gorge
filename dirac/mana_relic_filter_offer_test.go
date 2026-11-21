package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// Relic of Legends' no-{T} "tap an untapped legendary creature: add one mana of
// any color" beside a first-only filter land (Overflowing Basin) is a mix board:
// the filter search cannot fund through the Relic (filtFundable), so the tapX
// search is the offer's fallback, as scriptPrefix's mix arm already plays it.
// Death Cloud's {X}{B}{B}{B} at X=0 is paid by the Relic's own {T} and two
// legendary creatures' taps; gorge offers it, and the kernel once did not
// (tail-49 pair 14 decision 506, death_cloud_x_spell_offer).
func TestRelicBesideFilterPaysBlackPips(t *testing.T) {
	scen := func(board ...string) (sc *scen, cloud Obj) {
		t.Helper()
		deck := []cardset.Entry{{Name: "Death Cloud", Count: 1}, {Name: "Island", Count: 10}}
		seen := map[string]bool{}
		for _, n := range board {
			if !seen[n] {
				seen[n] = true
				deck = append(deck, cardset.Entry{Name: n, Count: 1})
			}
		}
		tbl, da, _, err := cardsettest.Set(t, "relicfilter").Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		sc = newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
		for _, n := range board {
			x := sc.slot(sc.perm(id[n], 0))
			sc.s.perms[x].flags &^= pfTapped | pfSick
		}
		return sc, sc.hand(id["Death Cloud"], 0)
	}
	sc, cloud := scen("Relic of Legends", "Shalai, Voice of Plenty", "Radha, Heart of Keld", "Overflowing Basin")
	if !sc.s.mixBoard(sc.k, 0) || !sc.s.tapXTwoLiveSource(sc.k, 0) {
		t.Fatalf("precondition: mixBoard %v tapXTwoLiveSource %v, want a filter beside a live no-{T} tap-another source",
			sc.s.mixBoard(sc.k, 0), sc.s.tapXTwoLiveSource(sc.k, 0))
	}
	d := sc.s.Step()
	if !castOffered(&d, cloud) {
		t.Fatal("Death Cloud not offered: the Relic's {T} and two legendary taps make {B}{B}{B}")
	}
	// one legendary creature fewer: only {B}{B} through the Relic, no offer
	sc, cloud = scen("Relic of Legends", "Radha, Heart of Keld", "Overflowing Basin")
	d = sc.s.Step()
	if castOffered(&d, cloud) {
		t.Fatal("Death Cloud offered with only two black mana sources")
	}
}
