package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestDigMissRemembersRestAndAtEOTPromises: a one-card Dig whose window holds
// no creature takes nothing, moves the rest card to its remainder zone, and
// remembers it (gorge effDig digRemember remembers every card the dig moved).
// The chained AtEOT$ Remembered then promises the next-end-step sacrifice on
// that remembered card even though it never reached the battlefield: gorge
// registers the untracked __kwEncoreSacrifice promise, so the end step puts a
// no-op trigger on the stack (CR 603.7). Impromptu Raid is the corpus carrier.
func TestDigMissRemembersRestAndAtEOTPromises(t *testing.T) {
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
	if sc.s.g.nrem != 1 || sc.s.g.rem[0] != land {
		t.Fatalf("remembered set %v (n=%d), want the rest card %d", sc.s.g.rem[:sc.s.g.nrem], sc.s.g.nrem, land)
	}

	// The chained AtEOT$ Remembered promises the sacrifice on the remembered
	// card even though it is in the graveyard, not on the battlefield.
	n0 := sc.s.ndly
	eot := kop.Instr{Op: kop.OpAtEOT, Obj: kop.RefRemembered, C: 0}
	sc.s.exec(sc.k, &eot)
	if sc.s.ndly != n0+1 {
		t.Fatalf("delayed entries %d -> %d, want one sacrifice promise", n0, sc.s.ndly)
	}
	if d := sc.s.dly[n0]; d.obj != land || d.when != dlyUntracked {
		t.Fatalf("promise %+v, want an untracked sacrifice on %d", d, land)
	}
}
