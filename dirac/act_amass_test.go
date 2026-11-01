package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAmass: OpAmass (gorge effAmass) with no Army creates the Army token
// and puts the counters on it; the next amass finds that printed Army and
// adds to it; RememberAmass$ (B bit 15) remembers the Army.
func TestAmass(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Subtypes = append(tbl.Subtypes, "Army")
	army := kop.SubMask{}.With(len(tbl.Subtypes) - 1)
	tok := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Army", Types: kop.TCreature, Subtypes: army})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.s.res.e.ctrl = 0
	in := kop.Instr{Op: kop.OpAmass, Obj: kop.RefController, A: kop.Const(2), C: int16(tok), B: kop.Const(-0x8000)}
	sc.s.exec(sc.k, &in)
	a := sc.s.res.lastTok
	if a == NoObj || sc.s.objs[a].zone != kop.ZoneBattlefield || sc.s.perms[sc.slot(a)].ctrLow(int(kop.CtrP1P1)) != 2 {
		t.Fatal("amass 2 with no Army: want a new Army token with two +1/+1 counters")
	}
	if sc.s.g.nrem != 1 || sc.s.g.rem[0] != a {
		t.Fatal("RememberAmass$: the Army not remembered")
	}
	in.A, in.B = kop.Const(1), kop.Const(0)
	sc.s.exec(sc.k, &in)
	if sc.s.res.lastTok != a || sc.s.perms[sc.slot(a)].ctrLow(int(kop.CtrP1P1)) != 3 {
		t.Fatal("amass 1 with an Army: want the same Army with three counters, no new token")
	}
}
