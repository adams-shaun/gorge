package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Throwaway enumeration (never committed): find a state where grantCap's grant
// search exhausts its 160 nodes on Earthen Ally's Earthbend 5 and pre-fix
// scriptOffers' filter search re-prices it.
func TestZZFloatProbe(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Great Divide Guide", Count: 1}, {Name: "Rumble Arena", Count: 1}, {Name: "Earthen Ally", Count: 1},
		{Name: "Bender's Waterskin", Count: 1}, {Name: "Meditation Pools", Count: 1}, {Name: "Jasmine Dragon Tea Shop", Count: 1},
		{Name: "White Lotus Hideout", Count: 1}, {Name: "Badgermole Cub", Count: 1}, {Name: "Island", Count: 20},
	}
	tbl, _, _, err := cardsettest.Synth(t, "std-tla").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	var lands []string
	for _, n := range []string{"Plains", "Mountain", "Swamp", "Forest"} {
		lands = append(lands, n)
	}
	lands = append(lands, "Island")
	type poolSpec struct {
		col int
		n   int
	}
	pools := [][]poolSpec{{{kop.C, 1}}, {{kop.C, 2}}, {{kop.U, 1}}, {{kop.G, 1}}, {{kop.W, 1}},
		{{kop.W, 1}, {kop.U, 1}, {kop.B, 1}, {kop.R, 1}, {kop.G, 1}}, {{kop.C, 3}}}
	nFound, nRef, nOffer := 0, 0, 0
	for _, tea := range []bool{true, false} {
		for _, wlh := range []bool{true, false} {
			for _, wat := range []bool{true, false} {
				for _, cub := range []bool{true, false} {
				for nl := 2; nl <= 6; nl++ {
					for _, tapMode := range []int{0, 1, 2, 3, 4} {
						for _, ps := range pools {
							sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
							perms := []string{"Great Divide Guide", "Rumble Arena", "Earthen Ally"}
							if wat {
								perms = append(perms, "Bender's Waterskin")
							}
							if cub {
								perms = append(perms, "Badgermole Cub")
							}
							if tea {
								perms = append(perms, "Jasmine Dragon Tea Shop")
							}
							if wlh {
								perms = append(perms, "White Lotus Hideout")
							}
							perms = append(perms, "Meditation Pools")
							perms = append(perms, lands[:nl]...)
							var ea Slot
							for _, n := range perms {
								o := sc.perm(id[n], 0)
								if n == "Earthen Ally" {
									ea = sc.slot(o)
								}
							}
							if tapMode > 0 {
								for b := sc.s.ctrlM[0].and(sc.s.occ); b.any(); b = b.rest() {
									sl := b.first()
									ty := sc.k.t.Cards[sc.s.cardAt(sl)].Types
									isLand := ty&kop.TLand != 0
									isCre := ty&kop.TCreature != 0
									if sl == ea || (cub && sc.s.cardAt(sl) == id["Badgermole Cub"]) {
										continue
									}
									if (tapMode == 1) || (tapMode == 2 && isLand) || (tapMode == 3 && isCre) ||
										(tapMode == 4 && sc.s.cardAt(sl) == id["Great Divide Guide"]) {
										sc.s.perms[sl].flags |= pfTapped
									}
								}
							}
							for _, sp := range ps {
								for i := 0; i < sp.n; i++ {
									sc.s.pl[0].pool[sp.col]++
								}
							}
							s, k := sc.s, sc.k
							var ai int = -1
							for i := range k.t.Cards[id["Earthen Ally"]].Abilities {
								ab := &k.t.Cards[id["Earthen Ally"]].Abilities[i]
								if ab.Cost.Mana.Generic > 0 {
									ai = i
									break
								}
							}
							a := MakeAction(AkActivate, uint8(s.perms[ea].obj), 0, uint8(ai))
							cost, ex, need, ok := s.grantPlayCost(k, 0, a, true)
							if !ok {
								continue
							}
							var g grantSrch
							found := s.grantSearch(k, 0, &cost, ex, s.grantPayDesc(a), need, &g)
							if found {
								continue
							}
							nRef++
							var dd Decision
							s.priorityOptions(k, &dd)
							offered := false
							for i := 0; i < int(dd.N); i++ {
								if dd.Options[i] == a {
									offered = true
								}
							}
							if offered {
								nOffer++
							}
							if g.nodes >= potentialScriptNodes {
								t.Logf("REF tea=%v wlh=%v wat=%v cub=%v nl=%d tap=%d pool=%v offered=%v nodes=%d", tea, wlh, wat, cub, nl, tapMode, ps, offered, g.nodes)
							}
							if offered && g.nodes >= potentialScriptNodes {
								nFound++
							}
						}
					}
				}
				}
			}
		}
	}
	t.Logf("ZZFOUND %d refusals %d offered %d", nFound, nRef, nOffer)
}

// TestZZReal7 rebuilds the seed-7 divergence board (all untapped, empty pool)
// and reports the grant search and the decision.
func TestZZReal7(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Great Divide Guide", Count: 1}, {Name: "Rumble Arena", Count: 1}, {Name: "Earthen Ally", Count: 1},
		{Name: "Badgermole Cub", Count: 1}, {Name: "Kyoshi Island Plaza", Count: 1}, {Name: "Katara, Bending Prodigy", Count: 1},
		{Name: "Gran-Gran", Count: 1}, {Name: "The Mechanist, Aerial Artisan", Count: 1}, {Name: "Wan Shi Tong, Librarian", Count: 1},
		{Name: "Platypus-Bear", Count: 1}, {Name: "Forecasting Fortune Teller", Count: 1}, {Name: "Island", Count: 20},
		{Name: "Forest", Count: 10},
	}
	tbl, _, _, err := cardsettest.Synth(t, "std-tla").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	var ea Slot
	for _, n := range []string{"Island", "Forest", "Forecasting Fortune Teller", "Island", "Rumble Arena",
		"Badgermole Cub", "Great Divide Guide", "Forest", "Kyoshi Island Plaza", "Island", "Forest",
		"Katara, Bending Prodigy", "Gran-Gran", "Forest", "The Mechanist, Aerial Artisan", "Wan Shi Tong, Librarian",
		"Earthen Ally", "Island", "Island", "Platypus-Bear"} {
		o := sc.perm(id[n], 0)
		if n == "Earthen Ally" {
			ea = sc.slot(o)
		}
	}
	s, k := sc.s, sc.k
	var ai int = -1
	for i := range k.t.Cards[id["Earthen Ally"]].Abilities {
		if k.t.Cards[id["Earthen Ally"]].Abilities[i].Cost.Mana.Generic > 0 {
			ai = i
			break
		}
	}
	a := MakeAction(AkActivate, uint8(s.perms[ea].obj), 0, uint8(ai))
	cost, ex, need, ok := s.grantPlayCost(k, 0, a, true)
	var g grantSrch
	found, nodes := false, 0
	if ok {
		found = s.grantSearch(k, 0, &cost, ex, s.grantPayDesc(a), need, &g)
		nodes = g.nodes
	}
	var dd Decision
	s.priorityOptions(k, &dd)
	offered := false
	for i := 0; i < int(dd.N); i++ {
		if dd.Options[i] == a {
			offered = true
		}
	}
	t.Logf("REAL7 ok=%v found=%v nodes=%d offered=%v", ok, found, nodes, offered)
}
