package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRememberedSacrificeController: a remembering SacrificeAll captures each
// victim's last controller. For a creature player 1 controls and player 0
// owns, a condition's Card.YouCtrl read and RememberedController answer player
// 1 (CR 608.2h; gorge lkiControllerFor), an ownership read and a value count
// answer player 0, and forgetting an earlier remembered object leaves each
// capture with its own object.
func TestRememberedSacrificeController(t *testing.T) {
	deck := []cardset.Entry{{Name: "Savannah Lions", Count: 20}, {Name: "Plains", Count: 20}}
	tbl, da, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	cp := *tbl
	cp.Filters = append(append([]kop.Filter(nil), tbl.Filters...),
		kop.Filter{Types: kop.TCreature}, kop.Filter{Ctrl: kop.CtrlYou}, kop.Filter{Owner: kop.CtrlYou})
	n := len(cp.Filters)
	creature, youCtrl, youOwn := kop.FilterID(n-3), kop.FilterID(n-2), kop.FilterID(n-1)
	in := kop.Instr{Op: kop.OpSacrificeAll, F: creature, C: kop.MvRemember}
	cp.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	cp.Cards[id["Plains"]].Abilities = []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{in}}}

	sc := newScenTableLib(t, &cp, id["Plains"], Config{NoElide: true})
	s := sc.s
	e := stackEntry{card: id["Plains"], ctrl: 0, obj: NoObj, kind: seSpell, trigObj: NoObj}
	for i := range e.tgt {
		e.tgt[i] = TCode(NoTarget)
	}
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	if len(coldRemCtl.of(s)) == 0 {
		t.Fatal("a table with a remembering SacrificeAll has no controller capture region")
	}
	a, b := sc.obj(id["Savannah Lions"], 0), sc.obj(id["Savannah Lions"], 0)
	for i, o := range []Obj{a, b} {
		ctrl := uint8(1 - i) // a is player 1's to control, b player 0's
		sl := s.enter(sc.k, o, ctrl, false, kop.ZoneNone)
		s.perms[sl].flags &^= pfSick
	}
	s.npend = 0
	s.execGen(sc.k, &in)
	if s.objs[a].zone != kop.ZoneGraveyard || s.objs[b].zone != kop.ZoneGraveyard || s.g.nrem != 2 || s.objs[a].owner() != 0 {
		t.Fatalf("sacrifice: zones %d %d, remembered %v", s.objs[a].zone, s.objs[b].zone, s.g.rem[:s.g.nrem])
	}
	ctx := func(p uint8) evalCtx { return evalCtx{ctrl: p, src: NoObj, srcSl: NoSlot, e: &s.res.e} }
	for p, want := range [2]struct{ gate, own, value int }{{1, 2, 2}, {1, 0, 0}} {
		if got := s.countRefMatches(sc.k, kop.RefRemembered, youCtrl, ctx(uint8(p))); got != want.gate {
			t.Errorf("condition Card.YouCtrl for player %d: %d, want %d", p, got, want.gate)
		}
		if got := s.countRefMatches(sc.k, kop.RefRemembered, youOwn, ctx(uint8(p))); got != want.own {
			t.Errorf("condition Card.YouOwn for player %d: %d, want %d", p, got, want.own)
		}
		if got := s.countRefMatchesN(sc.k, kop.RefRemembered, youCtrl, ctx(uint8(p)), false, false); got != want.value {
			t.Errorf("value count Card.YouCtrl for player %d: %d, want %d (the card where it lies)", p, got, want.value)
		}
	}
	// the set is in gorge's victim order, seat 0's permanent first
	if s.g.rem[0] != b || s.g.rem[1] != a {
		t.Fatalf("remembered %v, want [%d %d]", s.g.rem[:s.g.nrem], b, a)
	}
	if got := s.remController(); got != 0 {
		t.Errorf("RememberedController %d, want 0 (b's controller)", got)
	}
	s.forgetRemGen(b)
	if got := s.remController(); got != 1 {
		t.Errorf("RememberedController after forgetting the first: %d, want 1 (a's captured controller, not its owner)", got)
	}
}
