package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCopyTokenCounters: ExtCopyTokenCtr's token copy enters with B counters
// of kind C (gorge CopyPermanent WithCountersType$/WithCountersAmount$; CR
// 122.6, 614.1c), none for B <= 0, and the source's own counters are not
// copied (CR 707.2).
func TestCopyTokenCounters(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	ext := kop.ExtInstr(kop.ExtCopyTokenCtr)
	ext.Obj, ext.B, ext.C = kop.RefSelf, kop.Const(2), int16(kop.CtrP1P1)
	mirror := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Counter Mirror", Types: kop.TCreature, Abilities: []kop.Ability{{Kind: kop.AbActivated,
			Prog: []kop.Instr{ext}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	m := sc.perm(mirror, 0)
	if !sc.s.perms[sc.slot(m)].addCtrN(sc.s, int(kop.CtrP1P1), 4) {
		t.Fatal("counters")
	}
	sc.do(t, AkActivate, TCode(m), scAny)
	sc.do(t, AkPass, scAny, scAny)
	sc.do(t, AkPass, scAny, scAny)
	var toks []Slot
	for _, sl := range sc.copiesOf(mirror) {
		if sl != sc.slot(m) {
			toks = append(toks, sl)
		}
	}
	if len(toks) != 1 {
		t.Fatalf("%d token copies, want 1", len(toks))
	}
	if n := sc.s.perms[toks[0]].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Errorf("the token has %d +1/+1 counters, want 2", n)
	}
	if n := sc.s.perms[sc.slot(m)].ctrLow(int(kop.CtrP1P1)); n != 4 {
		t.Errorf("the source has %d +1/+1 counters, want 4", n)
	}
	if sc.s.g.enterCtr.n != 0 {
		t.Errorf("enterCtr left at %d", sc.s.g.enterCtr.n)
	}
}
