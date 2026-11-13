package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// k7Table is an FDN Plains table with test cards appended: two creatures,
// a bolster 2 sorcery, a proliferate sorcery (n times), and an enchantment
// with "whenever you proliferate, gain 1 life".
type actProliferateCards struct{ bear, big, bolster, prolif, prolif2, watcher CardID }

func actProliferateScen(t *testing.T) (*scen, actProliferateCards) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	c := actProliferateCards{bear: n, big: n + 1, bolster: n + 2, prolif: n + 3, prolif2: n + 4, watcher: n + 5}
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Big", Types: kop.TCreature, Power: 3, Toughness: 3},
		kop.CardDef{Name: "Test Bolster", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Prog: []kop.Instr{{Op: kop.OpBolster, A: kop.Const(2), C: int16(kop.CtrP1P1)}}}}},
		kop.CardDef{Name: "Test Proliferate", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Prog: []kop.Instr{{Op: kop.OpProliferate, A: kop.Const(1)}}}}},
		kop.CardDef{Name: "Test Proliferate Twice", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Prog: []kop.Instr{{Op: kop.OpProliferate, A: kop.Const(2)}}}}},
		kop.CardDef{Name: "Test Watcher", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigProliferate, Whose: kop.WhoseYou},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}})
	return newScenTable(t, tbl, Config{NoElide: true}), c
}

// actProliferateCast casts the sorcery card c from p0's hand and passes both priorities.
func (sc *scen) actProliferateCast(t *testing.T, c CardID) {
	t.Helper()
	o := sc.hand(c, 0)
	sc.do(t, AkCast, TCode(o), scAny)
	sc.do(t, AkPass, scAny, scAny)
	sc.do(t, AkPass, scAny, scAny)
}

// TestProliferate: the any-number pick over the permanents and players
// with counters, closed by Done (gorge effProliferate); each pick
// gets one more of each kind; "whenever you proliferate" fires once per
// action, also when nothing was eligible.
func TestProliferate(t *testing.T) {
	sc, c := actProliferateScen(t)
	sc.perm(c.watcher, 0)
	b1, b2, ob := sc.perm(c.bear, 0), sc.perm(c.bear, 0), sc.perm(c.bear, 1)
	sc.s.perms[sc.slot(b1)].addCtrN(sc.s, int(kop.CtrP1P1), 1)
	sc.s.perms[sc.slot(ob)].addCtrN(sc.s, int(kop.CtrM1M1), 1)
	sc.s.g.poison[1], sc.s.g.energy[0] = 2, 1
	sc.s.perms[sc.slot(b2)].flags |= pfRegen // a regeneration shield is no counter
	sc.actProliferateCast(t, c.prolif)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenProlif || d.N != 5 {
		t.Fatalf("decision kind %d sub %d with %d options, want the proliferate pick (b1, the opponent's bear, both players, Done)", d.Kind, d.Sub, d.N)
	}
	sc.do(t, AkChoose, TCode(b1), scAny)
	if d := sc.s.Step(); d.Sub != ChGenProlif || d.N != 4 {
		t.Fatalf("after one pick: sub %d with %d options, want 4", d.Sub, d.N)
	}
	sc.do(t, AkChoose, TCode(PlayerTarget0+1), scAny)
	sc.do(t, AkDone, scAny, scAny)
	if n := sc.s.perms[sc.slot(b1)].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Fatalf("b1 has %d +1/+1 counters, want 2", n)
	}
	if n := sc.s.perms[sc.slot(ob)].ctrLow(int(kop.CtrM1M1)); n != 1 {
		t.Fatalf("the unpicked bear has %d -1/-1 counters, want 1", n)
	}
	if sc.s.g.poison[1] != 3 || sc.s.g.energy[0] != 1 {
		t.Fatalf("poison %d energy %d, want 3 and 1", sc.s.g.poison[1], sc.s.g.energy[0])
	}
	life := sc.s.pl[0].life
	sc.do(t, AkPass, scAny, scAny)
	sc.do(t, AkPass, scAny, scAny)
	if sc.s.pl[0].life != life+1 {
		t.Fatalf("life %d, want %d: the proliferate trigger", sc.s.pl[0].life, life+1)
	}
}

// TestProliferateNoneEligible: with no counter anywhere nothing is asked,
// and proliferating twice still fires the trigger twice.
func TestProliferateNoneEligible(t *testing.T) {
	sc, c := actProliferateScen(t)
	sc.perm(c.watcher, 0)
	sc.perm(c.bear, 0)
	life := sc.s.pl[0].life
	sc.actProliferateCast(t, c.prolif2)
	if sc.s.npend != 2 && sc.s.sp < 2 {
		d := sc.s.Step()
		t.Fatalf("pending %d, stack %d (decision %d sub %d), want two proliferate triggers", sc.s.npend, sc.s.sp, d.Kind, d.Sub)
	}
	for i := 0; i < 8 && (sc.s.sp > 0 || sc.s.npend > 0); i++ {
		d := sc.s.Step() // the order of the two triggers, then passes
		sc.s.Apply(d.Options[0])
	}
	if sc.s.pl[0].life != life+2 {
		t.Fatalf("life %d, want %d", sc.s.pl[0].life, life+2)
	}
}
