package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// copyExwScen is a table with a source (ExiledWith), a plain creature card,
// an object-enchanting Aura card, and the filter Card.ExiledWithSource, plus
// two creatures on the battlefield (two legal Aura hosts).
func copyExwScen(t *testing.T) (sc *scen, in kop.Instr, id map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlAny, Zone: kop.ZoneBattlefield})
	host := kop.FilterID(len(tbl.Filters) - 1)
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "T Saga", ExiledWith: true, Types: kop.TEnchantment},
		kop.CardDef{Name: "T Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "T Aura", Types: kop.TEnchantment | kop.TAura,
			Enchant: kop.TargetSpec{Kind: kop.TgtObject, Filter: host}})
	id = map[string]CardID{"saga": n, "bear": n + 1, "aura": n + 2}
	tbl.Filters = append(tbl.Filters, kop.Filter{Zone: kop.ZoneExile, Flags: kop.FExiledWith})
	f := kop.FilterID(len(tbl.Filters) - 1)
	sc = newScenTable(t, tbl, Config{NoElide: true})
	in = kop.ExtInstr(kop.ExtCopyExiledWith)
	in.F = f
	return sc, in, id
}

func copyExwTokens(s *State) int {
	n := 0
	for m := s.occ; m.any(); m = m.rest() {
		if s.objs[s.perms[m.first()].obj].of&ofCopy != 0 {
			n++
		}
	}
	return n
}

// TestCopyExiledWithOrder: the copy source is every card exiled with the
// resolving source; a second plain card is copied too, and an Aura whose copy
// asks for its host parks when it is the only card, but stalls loudly when an
// earlier copy of the same op already entered (parking would drop that
// copy's enters check: the entry batch is fired only at the op's end).
func TestCopyExiledWithOrder(t *testing.T) {
	t.Run("two plain cards", func(t *testing.T) {
		sc, in, id := copyExwScen(t)
		src := sc.perm(id["saga"], 0)
		exwExile(sc, id["bear"], 0, src)
		exwExile(sc, id["bear"], 1, src)
		sc.s.res.active, sc.s.res.e.obj, sc.s.res.e.ctrl = true, src, 0
		sc.s.copyExiledWith(sc.k, &in)
		if got := copyExwTokens(sc.s); got != 2 || sc.s.g.stalled != 0 {
			t.Fatalf("copies %d stalled %d, want 2 copies and no stall", got, sc.s.g.stalled)
		}
	})
	t.Run("lone aura with two hosts parks", func(t *testing.T) {
		sc, in, id := copyExwScen(t)
		src := sc.perm(id["saga"], 0)
		sc.perm(id["bear"], 0)
		sc.perm(id["bear"], 1)
		exwExile(sc, id["aura"], 0, src)
		sc.s.res.active, sc.s.res.e.obj, sc.s.res.e.ctrl = true, src, 0
		if !sc.s.copyExiledWith(sc.k, &in) || sc.s.res.ask != raGenEnchant || sc.s.g.stalled != 0 {
			t.Fatalf("parked ask %d stalled %d, want a parked host ask", sc.s.res.ask, sc.s.g.stalled)
		}
	})
	t.Run("plain card then parking aura stalls", func(t *testing.T) {
		sc, in, id := copyExwScen(t)
		src := sc.perm(id["saga"], 0)
		sc.perm(id["bear"], 0)
		sc.perm(id["bear"], 1)
		exwExile(sc, id["bear"], 0, src)
		exwExile(sc, id["aura"], 0, src)
		sc.s.res.active, sc.s.res.e.obj, sc.s.res.e.ctrl = true, src, 0
		sc.s.copyExiledWith(sc.k, &in)
		if sc.s.g.stalled != StallCopyAllAsk {
			t.Fatalf("stalled %d, want StallCopyAllAsk (%d)", sc.s.g.stalled, StallCopyAllAsk)
		}
	})
}
