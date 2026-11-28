package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRememberedGateAlternative: a ConditionDefined$ Remembered gate over a
// sacrificed creature player 1 controlled and player 0 owns. An alternative
// with both a controller and an ownership predicate reads the captured
// controller for the first and the real owner for the second (gorge
// rememberedControllerLKI rewrites the controller only; CR 108.3, 608.2h),
// and a chosen-type alternative reads the source permanent's choice, as
// refCardMatch does.
func TestRememberedGateAlternative(t *testing.T) {
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
	sub := len(tbl.Subtypes) // a Cat subtype the table's filters did not need
	cp.Subtypes = append(append([]string(nil), tbl.Subtypes...), "Cat")
	cp.Filters = append(append([]kop.Filter(nil), tbl.Filters...),
		kop.Filter{Types: kop.TCreature},
		kop.Filter{Ctrl: kop.CtrlYou, Owner: kop.CtrlOpp},
		kop.Filter{Ctrl: kop.CtrlYou, Owner: kop.CtrlYou},
		kop.Filter{Ctrl: kop.CtrlYou, Flags2: kop.F2ChosenType})
	n := len(cp.Filters)
	creature, ctrlNotOwn, ctrlAndOwn, chosen := kop.FilterID(n-4), kop.FilterID(n-3), kop.FilterID(n-2), kop.FilterID(n-1)
	in := kop.Instr{Op: kop.OpSacrificeAll, F: creature, C: kop.MvRemember}
	cp.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	cp.Cards[id["Savannah Lions"]].Subtypes[sub>>6] |= 1 << (sub & 63)
	cp.Cards[id["Plains"]].Abilities = []kop.Ability{{Kind: kop.AbSpell, Prog: []kop.Instr{in}}}

	sc := newScenTableLib(t, &cp, id["Plains"], Config{NoElide: true})
	s := sc.s
	e := stackEntry{card: id["Plains"], ctrl: 0, obj: NoObj, kind: seSpell, trigObj: NoObj}
	for i := range e.tgt {
		e.tgt[i] = TCode(NoTarget)
	}
	s.res = resCtx{e: e, active: true, lastTok: NoObj, eachObj: NoObj}
	stolen := sc.obj(id["Savannah Lions"], 0)
	sl := s.enter(sc.k, stolen, 1, false, kop.ZoneNone)
	s.perms[sl].flags &^= pfSick
	s.npend = 0
	s.execGen(sc.k, &in)
	if c, ok := s.remCtl(stolen); !ok || c != 1 || s.objs[stolen].owner() != 0 || s.objs[stolen].zone != kop.ZoneGraveyard || s.g.nrem != 1 {
		t.Fatalf("sacrifice: capture %d %v, owner %d, zone %d, remembered %v", c, ok, s.objs[stolen].owner(), s.objs[stolen].zone, s.g.rem[:s.g.nrem])
	}
	ctx := evalCtx{ctrl: 1, src: NoObj, srcSl: NoSlot, e: &s.res.e}
	if got := s.countRefMatches(sc.k, kop.RefRemembered, ctrlNotOwn, ctx); got != 1 {
		t.Errorf("Card.YouCtrl+OppOwn for the last controller: %d, want 1", got)
	}
	if got := s.countRefMatches(sc.k, kop.RefRemembered, ctrlAndOwn, ctx); got != 0 {
		t.Errorf("Card.YouCtrl+YouOwn for the last controller: %d, want 0 (player 0 owns it)", got)
	}

	// the source permanent's chosen type, as in a table with a chosen-type
	// card target
	src := sc.obj(id["Plains"], 1)
	srcSl := s.enter(sc.k, src, 1, false, kop.ZoneNone)
	s.npend = 0
	sc.k.gen.chosenCardTgt = true
	ctx.src, ctx.srcSl = src, srcSl
	for _, c := range []struct {
		choice, want int
	}{{sub, 1}, {255, 0}} {
		s.perms[srcSl].chosen = objWord(c.choice)
		if got := int(s.srcChosen(srcSl, src)); got != c.choice {
			t.Fatalf("source choice %d, want %d", got, c.choice)
		}
		if got := s.countRefMatches(sc.k, kop.RefRemembered, chosen, ctx); got != c.want {
			t.Errorf("Card.YouCtrl+ChosenType with choice %d: %d, want %d", c.choice, got, c.want)
		}
	}
}
