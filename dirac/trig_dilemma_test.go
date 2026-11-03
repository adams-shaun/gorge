package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestReversedBuildTakesBackItsDilemma: a reversal (CR 733.1) takes the count back
// with the trigger the dilemma queued, and leaves a trigger queued after it.
func TestReversedBuildTakesBackItsDilemma(t *testing.T) {
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // a table with Exprs prepares the gen tables
	watcher, charm := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Watcher", Types: kop.TEnchantment, Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigFacesDilemma},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}},
		kop.CardDef{Name: "Test Charm", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell, NModes: 2,
			Prog: []kop.Instr{{Op: kop.OpNop}}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s := sc.s
	sc.perm(watcher, 0)
	o := sc.hand(charm, 0)
	*s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: charm, ctrl: 0, srcVer: s.objs[o].ver, mode: 1}}
	s.bld().e.tgt = noTargets
	s.ctl = ctlBuild
	s.buildModesChosen(sc.k, &sc.k.t.Cards[charm].Abilities[0])
	if s.g.dilemmas[0] != 1 || s.npend != 1 || s.pend[0].card != watcher {
		t.Fatalf("after the choice: %d dilemmas, %d triggers pending, want 1 and the watcher's", s.g.dilemmas[0], s.npend)
	}
	s.pend[1] = pendingTrig{card: vanilla.GrizzlyBears}
	s.npend = 2
	s.reverseCast(sc.k)
	if s.g.dilemmas[0] != 0 {
		t.Fatalf("%d dilemmas after the reversal, want 0", s.g.dilemmas[0])
	}
	if s.npend != 1 || s.pend[0].card != vanilla.GrizzlyBears {
		t.Fatalf("%d triggers pending, first of card %d: want only the later one", s.npend, s.pend[0].card)
	}
	if s.objs[o].zone != kop.ZoneHand {
		t.Fatalf("the reversed spell is in zone %d, want its hand", s.objs[o].zone)
	}
}
