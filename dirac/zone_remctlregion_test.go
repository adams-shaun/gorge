package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRememberedControllerRegionNeedsAReader: beside a remembering sacrifice,
// the captured-controller cold region (coldRemCtl) is laid out only for a
// table with something that can read the controller. A condition over the
// remembered set that tests only what the card is takes no State bytes (the
// mbc pool sits at the State bar, TestEveryPoolFitsTheStateBar); a condition
// that reads the controller, or the remembered controller as a player, keeps
// the region.
func TestRememberedControllerRegionNeedsAReader(t *testing.T) {
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 20}, {Name: "Plains", Count: 20}}
	tbl, da, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	plains := CardID(0)
	for _, c := range da {
		if tbl.Cards[c].Name == "Plains" {
			plains = c
		}
	}
	if tableReadsRemCtl(tbl) {
		t.Fatal("the base table already reads a remembered controller")
	}
	n := kop.FilterID(len(tbl.Filters))
	creature, youCtrl, creatureOrYours := n, n+1, n+2
	filters := append(append([]kop.Filter(nil), tbl.Filters...),
		kop.Filter{Types: kop.TCreature},
		kop.Filter{Ctrl: kop.CtrlYou},
		kop.Filter{Types: kop.TCreature, Or: youCtrl})
	sac := kop.Instr{Op: kop.OpSacrifice, Obj: kop.RefController, A: kop.ValueRef{N: 1}, B: kop.ValueRef{N: 1}}
	cond := func(f kop.FilterID) []kop.Cond {
		return []kop.Cond{{Kind: kop.CondRefMatches, Ref: kop.RefRemembered, F: f}}
	}
	for _, c := range []struct {
		name  string
		conds []kop.Cond
		extra []kop.Instr
		want  bool
	}{
		{name: "no reader"},
		{name: "card type condition", conds: cond(creature)},
		{name: "controller condition", conds: cond(youCtrl), want: true},
		{name: "controller alternative", conds: cond(creatureOrYours), want: true},
		{name: "unfiltered condition", conds: cond(0), want: true},
		{name: "remembered controller", extra: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefRememberedController, A: kop.ValueRef{N: 1}}}, want: true},
		{name: "card loop over a ref", extra: []kop.Instr{{Op: kop.OpRepeatEach, Obj: kop.RefImprinted}}, want: true},
		{name: "battlefield loop beside the sacrifice", extra: []kop.Instr{{Op: kop.OpRepeatEach, F: creature}}, want: true},
	} {
		t.Run(c.name, func(t *testing.T) {
			cp := *tbl
			cp.Filters = filters
			cp.Conds = append(append([]kop.Cond(nil), tbl.Conds...), c.conds...)
			cp.Cards = append([]kop.CardDef(nil), tbl.Cards...)
			cp.Cards[plains].Abilities = []kop.Ability{{Kind: kop.AbSpell, Prog: append([]kop.Instr{sac}, c.extra...)}}
			s := newScenTableLib(t, &cp, plains, Config{NoElide: true}).s
			if g := s.k().gen; !g.anyRemSac || g.anyRemEach {
				t.Fatalf("anyRemSac %v anyRemEach %v, want a remembering sacrifice and no card loop", g.anyRemSac, g.anyRemEach)
			}
			if got := len(coldRemCtl.of(s)) != 0; got != c.want {
				t.Errorf("controller capture region laid out: %v, want %v", got, c.want)
			}
		})
	}
}
