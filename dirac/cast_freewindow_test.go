package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFreeCastUnpayableRaiseReverses: a spell cast without paying its mana
// cost still owes a RaiseCost, its per-target charge included (CR 601.2f,
// 118.9d). When the second target's {W}{U} raise has no source the cast
// window can tap, the payment fails and the cast is reversed at once (gorge
// payCast abortCast "cost no longer payable", CR 733.1), instead of panicking
// in payFromPool (Reenact the Crime's free copy of Officious Interrogation,
// std-mkm pair 2359).
func TestFreeCastUnpayableRaiseReverses(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	spell := CardID(len(tbl.Cards))
	wu := kop.ManaCost{}
	wu.Pips[kop.W], wu.Pips[kop.U] = 1, 1
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Interrogation", Types: kop.TInstant, Cost: wu, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatIncreaseCost, Zone: kop.ZoneCommand,
			StFlags: kop.StRaisePerTarget, B: 1<<(kop.ReduceColorBits*int(kop.W)) | 1<<(kop.ReduceColorBits*int(kop.U))}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	o := sc.hand(spell, 0)
	sc.kcastBuild(o, spell, 0)
	e := &sc.s.bld().e
	e.flags |= sfFree
	e.nt, e.tgt[0], e.tgt[1] = 2, TCode(PlayerTarget0), TCode(PlayerTarget0+1)
	if sc.s.ctrlM[0].any() {
		t.Fatal("the battlefield has a mana source: the window could pay the raise")
	}
	cost := sc.s.buildCost(sc.k)
	if cost.Pips[kop.W] != 1 || cost.Pips[kop.U] != 1 {
		t.Fatalf("the free two-target cast owes %+v, want {W}{U} from the per-target raise", cost)
	}
	sc.s.finishBuild(sc.k)
	if sc.s.ctl != ctlPriority || sc.s.sp != 0 {
		t.Fatalf("after the failed payment ctl=%d stack=%d, want priority with the cast reversed", sc.s.ctl, sc.s.sp)
	}
	pl := &sc.s.pl[0]
	if sc.s.objs[o].zone != kop.ZoneHand || pl.handN == 0 || pl.hand[pl.handN-1] != o {
		t.Fatalf("the reversed card zone=%d, want the end of the hand", sc.s.objs[o].zone)
	}
}
