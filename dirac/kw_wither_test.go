package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWither: a printed-wither source's damage to a creature is -1/-1
// counters (fight-style damageBetween and a resolving source's damageTo);
// its damage to a player is ordinary life loss, no poison.
func TestWither(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Witherer", Types: kop.TCreature, Power: 2, Toughness: 2, Wither: true},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 4})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyInfect {
		t.Fatal("a wither table does not turn on the infect paths")
	}
	w, b := sc.perm(n, 0), sc.perm(n+1, 1)
	sc.s.damageBetween(sc.k, sc.slot(w), sc.slot(b), 2)
	pb := &sc.s.perms[sc.slot(b)]
	if pb.damage != 0 || pb.ctrLow(int(kop.CtrM1M1)) != 2 {
		t.Fatalf("fight: damage %d, -1/-1 counters %d", pb.damage, pb.ctrLow(int(kop.CtrM1M1)))
	}
	sc.s.res = resCtx{e: stackEntry{kind: seAbility, obj: w, card: n, ctrl: 0, srcVer: sc.s.objs[w].ver, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	life := sc.s.pl[1].life
	sc.s.damageTo(sc.k, uint8(PlayerTarget0+1), 3)
	if sc.s.pl[1].life != life-3 || sc.s.Poison(1) != 0 {
		t.Fatalf("to a player: life %d -> %d, poison %d", life, sc.s.pl[1].life, sc.s.Poison(1))
	}
	sc.s.damageTo(sc.k, uint8(b), 1)
	if pb := &sc.s.perms[sc.slot(b)]; pb.damage != 0 || pb.ctrLow(int(kop.CtrM1M1)) != 3 {
		t.Fatalf("damageTo a creature: damage %d, -1/-1 counters %d", pb.damage, pb.ctrLow(int(kop.CtrM1M1)))
	}
}
