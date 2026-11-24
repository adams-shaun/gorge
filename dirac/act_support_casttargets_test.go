package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSupportCastTargets: OpSupport's cast-time-target form (C&0x1000, an
// instant or sorcery's Support, CR 701.41a with 601.2c) applies one counter
// to each target already announced for its slot and poses no resolution
// pick. The primary and one extra target both receive a +1/+1 counter; a
// creature that was never announced stays untouched.
func TestSupportCastTargets(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	bear := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	// The lowering's cast-time slot: 0..2 creature targets on the spell's
	// ability, so an extra target's legality recheck can find its spec.
	k.t.Cards[bear].Abilities = []kop.Ability{{
		Kind: kop.AbSpell, NTargets: 1,
		Targets: [kop.MaxTargets]kop.TargetSpec{{
			Kind: kop.TgtObject, Filter: kop.FilterID(len(tbl.Filters) - 1), Min: 0, Max: 2,
		}},
	}}
	b1, b2, other := sc.perm(bear, 0), sc.perm(bear, 0), sc.perm(bear, 1)
	src := sc.perm(bear, 0)
	s.res.e.obj, s.res.e.ctrl, s.res.e.srcVer = src, 0, s.objs[src].ver
	s.res.e.card, s.res.e.ab = bear, 0
	// The cast announced b1 as the slot's primary target and b2 as an extra.
	s.res.e.tgt[0] = TCode(b1)
	xt := s.xtMut(&s.res.e)
	xt.slot[0], xt.code[0], xt.ver[0] = 0, TCode(b2), s.objs[b2].ver
	xt.n = 1

	in := kop.Instr{Op: kop.OpSupport, Obj: kop.RefTarget0, A: kop.Const(2),
		C: int16(kop.CtrP1P1) | 0x1000}
	// The generic each-target loop runs the instruction once per announced
	// target (interp_gen.go); supportAnnounced places one counter per call.
	s.execEachTarget(k, &in, 0)
	if got := s.perms[s.objs[b1].slot].ctrLow(int(kop.CtrP1P1)); got != 1 {
		t.Fatalf("primary target +1/+1 counters = %d, want 1", got)
	}
	if got := s.perms[s.objs[b2].slot].ctrLow(int(kop.CtrP1P1)); got != 1 {
		t.Fatalf("extra target +1/+1 counters = %d, want 1", got)
	}
	if got := s.perms[s.objs[other].slot].ctrLow(int(kop.CtrP1P1)); got != 0 {
		t.Fatalf("untargeted creature +1/+1 counters = %d, want 0", got)
	}
}
