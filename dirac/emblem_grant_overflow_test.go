package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEmblemGrantOverflowStalls: globalGrantsGen's emblem pass holds one
// grant per emblem the state can hold (MaxSeats * maxEmbTotal). Every seat
// holds maxEmbTotal emblems of a Plains whose Continuous statics grant any
// permanent the Plains' emblem-making ability. When the grants just fill the
// pass, all of them reach the callback and nothing stalls. One more grant per
// emblem overflows it, and the game stalls StallGrants instead of dropping
// the rest.
func TestEmblemGrantOverflowStalls(t *testing.T) {
	const capacity = MaxSeats * maxEmbTotal
	for _, extra := range []int{0, 1} {
		var per int
		sc, id := newBLBScenTweak(t, []cardset.Entry{{Name: "Plains", Count: 8}}, func(tbl *kop.Table, id map[string]CardID) {
			per = capacity/(2*maxEmbTotal) + extra // the scenario seats two players
			tbl.Filters = append(tbl.Filters[:len(tbl.Filters):len(tbl.Filters)], kop.Filter{})
			any := kop.FilterID(len(tbl.Filters) - 1)
			cd := &tbl.Cards[id["Plains"]]
			grant := uint8(len(cd.Abilities) + 1) // the emblem maker below
			var e kop.Emblem
			for i := 0; i < per; i++ {
				e.Abilities = append(e.Abilities, kop.Ability{Kind: kop.AbStatic,
					Static: kop.Static{Kind: kop.StatContinuous, Filter: any, Grant: grant}})
			}
			tbl.Emblems = append(tbl.Emblems[:len(tbl.Emblems):len(tbl.Emblems)], e)
			cd.Abilities = append(append([]kop.Ability(nil), cd.Abilities...), kop.Ability{Kind: kop.AbActivated, Prog: []kop.Instr{
				{Op: kop.OpCreateEmblem, Obj: kop.RefController, C: int16(len(tbl.Emblems) - 1)},
			}})
		})
		s := sc.s
		s.g.on = true
		if !sc.k.gen.anyEmbGrant || s.embCap() != maxEmbTotal || s.nSeats() != 2 {
			t.Fatalf("precondition: emblem grant %v, capacity %d, seats %d; want true, %d, 2", sc.k.gen.anyEmbGrant, s.embCap(), s.nSeats(), maxEmbTotal)
		}
		pl := sc.perm(id["Plains"], 0)
		in := kop.Instr{Op: kop.OpCreateEmblem, Obj: kop.RefController, C: int16(len(sc.k.t.Emblems) - 1)}
		for p := uint8(0); p < 2; p++ {
			for i := 0; i < maxEmbTotal; i++ {
				s.res.e.ctrl, s.res.e.card, s.res.e.obj = p, id["Plains"], pl
				s.execGen(sc.k, &in)
			}
		}
		if s.g.nemb[0] != maxEmbTotal || s.g.nemb[1] != maxEmbTotal || s.g.stalled != 0 {
			t.Fatalf("precondition: %d and %d emblems, stall %d; want %d each and none", s.g.nemb[0], s.g.nemb[1], s.g.stalled, maxEmbTotal)
		}
		n := 0
		s.globalGrantsGen(sc.k, s.objs[pl].slot, false, 0, func(CardID, uint8, int, uint16) { n++ })
		over := 2*maxEmbTotal*per > capacity
		if wantN := min(2*maxEmbTotal*per, capacity); n != wantN || (s.g.stalled != 0) != over || over && s.g.stalled != StallGrants {
			t.Errorf("%d grants per emblem: %d emitted, stall %d (%s); want %d emitted, stalled %v (StallGrants)",
				per, n, s.g.stalled, StallName(s.g.stalled), wantN, over)
		}
	}
}
