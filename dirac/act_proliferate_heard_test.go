package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

type actProliferateHeardCards struct{ bear, prolif, watcher CardID }

// actProliferateHeardScen is an FDN Plains table with a creature, a proliferate
// sorcery and an enchantment whose counter-added trigger (gain 1 life) hears
// kind hears: kop.CtrLore stands for a Saga's chapters, which hear lore only.
func actProliferateHeardScen(t *testing.T, hears int16) (*scen, actProliferateHeardCards) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	c := actProliferateHeardCards{bear: n, prolif: n + 1, watcher: n + 2}
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Proliferate", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Prog: []kop.Instr{{Op: kop.OpProliferate, A: kop.Const(1)}}}}},
		kop.CardDef{Name: "Test Watcher", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigCounterAddedOnce, N: hears},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}})
	return newScenTable(t, tbl, Config{NoElide: true}), c
}

// actProliferateTwoKinds proliferates a bear holding a +1/+1 and a lifelink
// counter (Unexpected Fangs) beside the watcher, picking the bear.
func actProliferateTwoKinds(t *testing.T, hears int16) (*scen, Obj) {
	t.Helper()
	sc, c := actProliferateHeardScen(t, hears)
	sc.perm(c.watcher, 0)
	bear := sc.perm(c.bear, 0)
	sl := sc.slot(bear)
	sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrP1P1), 1)
	sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrLifelink), 1)
	if !sc.s.subs[kop.TrigCounterAddedOnce].any() || sc.s.permRealCtrs(sl) != 2 {
		t.Fatalf("precondition: counter-added subscribed %v, bear kinds %d; want a listener and two kinds",
			sc.s.subs[kop.TrigCounterAddedOnce].any(), sc.s.permRealCtrs(sl))
	}
	sc.actProliferateCast(t, c.prolif)
	if d := sc.s.Step(); d.Kind != DkChoose || d.Sub != ChGenProlif || d.N != 2 {
		t.Fatalf("decision kind %d sub %d with %d options, want the proliferate pick (the bear, Done)", d.Kind, d.Sub, d.N)
	}
	sc.do(t, AkChoose, TCode(bear), scAny)
	return sc, bear
}

// TestProliferateTwoKindsUnheardOrder: gorge gives a two-kind recipient its
// kinds in insertion order, the kernel ascending; the order is seen only
// between kinds a counter-added trigger hears (CR 603.3b). A lore-only
// listener (a Saga's chapters, dr-mbc pair 506: Wanderer's Strike over a
// Butcher Ghoul wearing Unexpected Fangs beside The Birth of Meletis) and a
// +1/+1-only one hear at most one of +1/+1 and lifelink, so the pick
// proliferates both; an any-kind listener hears both and still stalls.
func TestProliferateTwoKindsUnheardOrder(t *testing.T) {
	for _, tc := range []struct {
		name  string
		hears int16
		gain  int16
	}{{"lore", int16(kop.CtrLore), 0}, {"p1p1", int16(kop.CtrP1P1), 1}} {
		t.Run(tc.name, func(t *testing.T) {
			sc, bear := actProliferateTwoKinds(t, tc.hears)
			life := sc.s.pl[0].life
			if r := sc.s.StallReason(); r != 0 {
				t.Fatalf("stalled %d on a recipient with one heard kind at most", r)
			}
			p := &sc.s.perms[sc.slot(bear)]
			if p.ctrN(sc.s, int(kop.CtrP1P1)) != 2 || p.ctrN(sc.s, int(kop.CtrLifelink)) != 2 {
				t.Fatalf("bear +1/+1 %d lifelink %d, want 2 and 2", p.ctrN(sc.s, int(kop.CtrP1P1)), p.ctrN(sc.s, int(kop.CtrLifelink)))
			}
			for i := 0; i < 6 && (sc.s.sp > 0 || sc.s.npend > 0); i++ {
				sc.do(t, AkPass, scAny, scAny)
			}
			if sc.s.pl[0].life != life+tc.gain {
				t.Fatalf("life %d, want %d: the watcher fires once per heard kind", sc.s.pl[0].life, life+tc.gain)
			}
		})
	}
	sc, _ := actProliferateTwoKinds(t, kop.TrigNAnyCounter)
	if r := sc.s.StallReason(); r != StallProlif {
		t.Fatalf("two kinds under an any-kind trigger: stall %d, want StallProlif", r)
	}
}
