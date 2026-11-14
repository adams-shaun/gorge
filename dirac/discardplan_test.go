package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestDiscardCostKeepsHandDemand: gorge plans a cast's sources at
// priority, before the cost's discard is chosen, so the card discarded to
// Thrill of Possibility still counts toward the hand demand (pay/plan.go
// RankPlan key 6): Refute's {U}{U} keeps the Island, and {1}{R} taps both
// Mountains. Before, the kernel discarded first and tapped the Island for
// the generic (pair 1007 and 15 more in the 10k slice).
func TestDiscardCostKeepsHandDemand(t *testing.T) {
	deck := []cardset.Entry{{Name: "Thrill of Possibility", Count: 1}, {Name: "Refute", Count: 1}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 5}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	m1 := sc.perm(id["Mountain"], 0)
	isl := sc.perm(id["Island"], 0)
	m2 := sc.perm(id["Mountain"], 0)
	thrill := sc.hand(id["Thrill of Possibility"], 0)
	ref := sc.hand(id["Refute"], 0)
	sc.do(t, AkCast, TCode(thrill), scAny)
	for i := 0; i < 6 && !(sc.s.objs[thrill].zone == kop.ZoneStack && sc.s.ctl == ctlPriority); i++ {
		d := sc.s.Step()
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[thrill].zone != kop.ZoneStack || sc.s.objs[ref].zone != kop.ZoneGraveyard {
		t.Fatalf("Thrill zone %d, Refute zone %d: want the stack and the graveyard", sc.s.objs[thrill].zone, sc.s.objs[ref].zone)
	}
	if !sc.tapped(m1) || !sc.tapped(m2) || sc.tapped(isl) {
		t.Fatalf("tapped Mountains %v,%v Island %v; want both Mountains, not the Island", sc.tapped(m1), sc.tapped(m2), sc.tapped(isl))
	}
}

// TestFlashbackDiscardCostCountsWholeHand: a granted flashback Thrill of
// Possibility (Sphinx of Forgotten Lore) is cast from the graveyard, so
// its discard cost can take the hand's only card. Before, the kernel
// subtracted the cast card from the hand as for a hand cast and did not
// offer it (pair 1370 and 23 more in the 100k census).
func TestFlashbackDiscardCostCountsWholeHand(t *testing.T) {
	deck := []cardset.Entry{{Name: "Thrill of Possibility", Count: 1}, {Name: "Refute", Count: 1}, {Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Mountain", Count: 5}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.perm(id["Mountain"], 0)
	sc.perm(id["Mountain"], 0)
	sc.hand(id["Refute"], 0)
	thrill := sc.obj(id["Thrill of Possibility"], 0)
	sc.s.place(thrill, kop.ZoneGraveyard, false)
	sc.s.grantFB(sc.k, thrill)
	sc.do(t, AkCast, TCode(thrill), scAny)
}

// TestDiscardPermanentBaseInHand: gorge's discardEligible matches
// DiscardValid$ with matchesInOwnZone (cardflow.go:442, filter.go:3382), so a
// bare `Permanent` base is a permanent card in hand (CR 110.4a; gorge
// 85b06656f) and a ChooseOpp / ChooseSelf discard of Permanent.nonLand asks.
func TestDiscardPermanentBaseInHand(t *testing.T) {
	tbl := *v.Table
	// vanilla is a hand-written table (k.gen nil, the non-generated
	// OpDiscard); a non-nil Exprs gives it a genPrep so the generated
	// OpDiscard under test runs.
	tbl.Exprs = []kop.Expr{}
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	permF := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{
		Types: kop.TLand | kop.TCreature | kop.TArtifact | kop.TEnchantment | kop.TPlaneswalker | kop.TBattle, NotTypes: kop.TLand, Zone: kop.ZoneHand,
	})
	creatureF := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Zone: kop.ZoneHand})
	for _, tc := range []struct {
		name    string
		f       kop.FilterID
		c       int16
		wantAsk bool
	}{
		{"ChooseOpp Permanent.nonLand", permF, kop.ChooseOpp, true},
		{"ChooseSelf Permanent.nonLand", permF, kop.ChooseSelf, true},
		{"ChooseOpp Creature", creatureF, kop.ChooseOpp, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := newScenTable(t, &tbl, Config{NoElide: true})
			bear := sc.hand(v.GrizzlyBears, 1)
			sc.hand(v.GrizzlyBears, 1) // a second matching card, so the self-chosen discard asks
			in := kop.Instr{Op: kop.OpDiscard, Obj: kop.RefTarget0,
				A: kop.Const(1), C: tc.c, F: tc.f}
			sc.s.res = resCtx{e: stackEntry{card: v.GrizzlyBears, ctrl: 0},
				active: true, lastTok: NoObj, eachObj: NoObj}
			sc.s.res.e.tgt[0] = TCode(PlayerTarget0 + 1)
			ok := sc.s.exec(sc.k, &in)
			if tc.wantAsk {
				if ok || sc.s.res.ask != raDiscard {
					t.Fatalf("ok=%v ask=%d, want a discard ask", ok, sc.s.res.ask)
				}
			} else if !ok || sc.s.res.ask != 0 {
				t.Fatalf("ok=%v ask=%d, want no ask", ok, sc.s.res.ask)
			}
			if sc.s.objs[bear].zone != kop.ZoneHand {
				t.Fatalf("card zone %d, want hand", sc.s.objs[bear].zone)
			}
		})
	}
}
