package dirac

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Throwaway: the real game's board shape. Does the kernel offer the plain
// cast of Bedlam Reveler ({4}{R}{R} after the 2-card graveyard reduction)
// with 3 whites + Mountain + Fabled Path (intrinsic R) + Shadowblood Ridge
// ({1},{T}: Add {B}{R}) + the restricted Untaidake?
func TestZZBedlamFeeGap(t *testing.T) {
	var out strings.Builder
	pr := func(f string, a ...any) { fmt.Fprintf(&out, f, a...) }
	set := cardsettest.Synth(t, "tail-73")
	deck := []cardset.Entry{
		{Name: "Bedlam Reveler", Count: 1},
		{Name: "Wanderer's Intervention", Count: 1},
		{Name: "Devastate", Count: 1},
		{Name: "Plains", Count: 1},
		{Name: "Island", Count: 1},
		{Name: "Forest", Count: 1},
		{Name: "Mountain", Count: 1},
		{Name: "Fabled Path of Searo Point", Count: 1},
		{Name: "Shadowblood Ridge", Count: 1},
		{Name: "Untaidake, the Cloud Keeper", Count: 1},
		{Name: "Plains", Count: 30},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var plains CardID
	for c := CardID(0); c < CardID(len(tbl.Cards)); c++ {
		if tbl.Cards[c].Name == "Plains" {
			plains = c
			break
		}
	}
	sc := newScenTableLib(t, tbl, plains, Config{})
	debugBedlamTrace = true
	defer func() { debugBedlamTrace = false }()
	for _, n := range []string{"Plains", "Island", "Forest", "Mountain", "Fabled Path of Searo Point", "Shadowblood Ridge", "Untaidake, the Cloud Keeper"} {
		sc.perm(id[n], 0)
	}
	for _, n := range []string{"Wanderer's Intervention", "Devastate"} {
		sc.s.place(sc.obj(id[n], 0), kop.ZoneGraveyard, false)
	}
	br := id["Bedlam Reveler"]
	sc.hand(br, 0)
	var ho Obj
	for i := 0; i < int(sc.s.pl[0].handN); i++ {
		if sc.s.objs[sc.s.pl[0].hand[i]].card == br {
			ho = sc.s.pl[0].hand[i]
		}
	}
	var av avail
	sc.s.availFor(sc.k, 0, &av)
	pr("avail total=%d n=%d mask=%v cnt=%v\n", av.total, av.n, av.mask[:6], av.cnt[:6])
	base := sc.k.cards[br].cost
	adj := sc.s.adjustCostGen(sc.k, 0, br, ho, base)
	pr("base=%v adj=%v\n", base, adj)
	fit := sc.s.adjustFit(sc.k, &av, 0, br, ho, base, false)
	pr("fit=%v gate=%v\n", fit, sc.s.castGateOrPoolPays(sc.k, 0, br, ho, &fit, &av))
	var fav avail
	cav := sc.s.castGrantAvail(sc.k, 0, ho, &av, &fav)
	pr("cav=%d gate2=%v\n", cav.total, sc.s.castGateAffordable(sc.k, 0, br, ho, &fit, cav))
	pr("phy=%v restr=%v\n", sc.s.phyCastGateAffords(sc.k, 0, cav, &fit), sc.s.restrFallback(sc.k, 0, br, sc.s.restrOriginOfObj(ho), &fit, cav))
	pr("cfgGorgeAutopay=%v lifeGap=%v uses=%v\n", sc.s.cfg&cfgGorgeAutopay != 0, sc.s.lifeGapMask(sc.k, 0).any(), sc.k.gen != nil && sc.k.gen.uses.has(useLifeGap))
	d := sc.s.Step()
	pr("options (%d):\n", d.N)
	offered := false
	// find the hand obj properly
	for i := range d.Options[:d.N] {
		a := &d.Options[i]
		if a.Kind() == AkCast && a.Source() == TCode(uint8(ho)) {
			offered = true
		}
	}
	pr("reveler cast offered=%v\n", offered)
	for i := range d.Options[:d.N] {
		a := &d.Options[i]
		pr("opt %v kind=%d src=%d tgt=%d mode=%d\n", i, a.Kind(), a.Source(), a.Target(), a.Mode())
	}
	t.Fatal("DEBUG\n" + out.String())
}
