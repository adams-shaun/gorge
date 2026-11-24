package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestDigMissRemembersNothing: a one-card Dig whose window holds no creature
// takes nothing and moves the rest card to its remainder zone without
// remembering it: RememberChanged$ is Forge's remZone1, the taken cards only
// (gorge effDig, #4479; the rest joins Remembered only under
// RememberMovedToZone$ 2, which kgen refuses). The chained AtEOT$ Remembered
// then has nothing to promise. Impromptu Raid is the corpus carrier.
func TestDigMissRemembersNothing(t *testing.T) {
	deck := []cardset.Entry{{Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}}
	var digCard CardID
	tblCard := kop.CardDef{
		Name:  "Test Dig Miss",
		Types: kop.TSorcery,
		Abilities: []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{{
			Op: kop.OpDigEx, Obj: kop.RefController,
			A: kop.Const(1), B: kop.Const(1),
			C: int16(kop.ZoneBattlefield) | (int16(kop.ZoneGraveyard)|kop.DigRemember)<<3,
			F: 1, // vanilla fCreature
		}}}},
	}
	sc, _ := newBLBScenTweak(t, deck, func(tbl *kop.Table, _ map[string]CardID) {
		digCard = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, tblCard)
	})

	pl := &sc.s.pl[0]
	land := sc.obj(v.Plains, 0)
	sc.s.objs[land].zone = kop.ZoneLibrary
	pl.libTop(land)

	// The card's DigEx is resolving at pc 0; its window is the one land.
	sc.s.res = resCtx{
		e:      stackEntry{kind: seSpell, ctrl: 0, obj: 0, card: digCard, ab: 0},
		pc:     0,
		active: true,
	}
	sc.s.g.nlook, sc.s.g.lookSel, sc.s.g.pickP = 1, 0, 0
	sc.s.resArm().look[0] = land
	sc.s.resArm().lookOrig[0] = 0

	sc.s.finishDigTail(sc.k)
	if z := sc.s.objs[land].zone; z != kop.ZoneGraveyard {
		t.Fatalf("the missed rest card is in zone %d, want the graveyard", z)
	}
	if sc.s.g.nrem != 0 {
		t.Fatalf("remembered set %v (n=%d), want none: the rest arm is not remembered", sc.s.g.rem[:sc.s.g.nrem], sc.s.g.nrem)
	}

	// The chained AtEOT$ Remembered has no object to promise on.
	n0 := sc.s.ndly
	eot := kop.Instr{Op: kop.OpAtEOT, Obj: kop.RefRemembered, C: 0}
	sc.s.exec(sc.k, &eot)
	if sc.s.ndly != n0 {
		t.Fatalf("delayed entries %d -> %d, want no promise over an empty remembered set", n0, sc.s.ndly)
	}
}
