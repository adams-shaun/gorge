package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestGrantedMaxHand: a Continuous static's granted CardDef.Granted static
// (S:Mode$ Continuous | AddStaticAbility$, Nerd Rage's "You have no maximum
// hand size") is read with each RECIPIENT as its source: "you" is the
// recipient's controller, not the granting card's. A hand of nine at cleanup
// discards two with no grant in force and none when the active player (seat
// 0) controls a recipient.
//
// The granter is itself a creature its own filter matches (Callaphe's
// shape), so a recipient that is its own source reads the recipient set it is
// part of; the walk reads that set below layer 6 and must neither loop nor
// miss it.
func TestGrantedMaxHand(t *testing.T) {
	tbl := *v.Table
	tbl.Conds = append([]kop.Cond(nil), tbl.Conds...)
	tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondYourTurn}) // a generated table: k.gen != nil
	tbl.Filters = append([]kop.Filter(nil), tbl.Filters...)
	youCreature := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	oppCreature := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlOpp})
	grant := kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatNoMaxHandSize}}
	granter := func(name string, f kop.FilterID) kop.CardDef {
		return kop.CardDef{Name: name, Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: f, GrantSt: 1}}},
			Granted:   []kop.Ability{grant}}
	}
	base := len(tbl.Cards)
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		granter("Test Anthem Granter", youCreature),
		granter("Test Curse Granter", oppCreature),
		kop.CardDef{Name: "Test Bearer", Types: kop.TCreature, Power: 1, Toughness: 1})
	youG, oppG, bearer := CardID(base), CardID(base+1), CardID(base+2)
	type perm struct {
		card CardID
		ctrl uint8
	}
	for _, tc := range []struct {
		name  string
		perms []perm
		want  int // discards the active player (seat 0) must make from nine cards
	}{
		{"no granter", nil, 2},
		{"a bearer and no granter", []perm{{bearer, 0}}, 2},
		{"granter of mine grants itself", []perm{{youG, 0}}, 0},
		{"opponent's anthem granter, my bearer: not their creature", []perm{{youG, 1}, {bearer, 0}}, 2},
		{"opponent's anthem granter alone: nothing of mine to grant", []perm{{youG, 1}}, 2},
		{"opponent's curse granter and my bearer: I am the recipient's controller", []perm{{oppG, 1}, {bearer, 0}}, 0},
		{"opponent's curse granter and no creature of mine", []perm{{oppG, 1}}, 2},
		{"my curse granter and my bearer: bearer is not an opposing creature", []perm{{oppG, 0}, {bearer, 0}}, 2},
		{"my curse granter, opponent's bearer: its controller, not I, has no maximum", []perm{{oppG, 0}, {bearer, 1}}, 2},
	} {
		sc := newScenTable(t, &tbl, Config{NoElide: true})
		if sc.k.gen == nil || !sc.k.gen.anyHandSize {
			t.Fatal("anyHandSize not prepped from the granted static")
		}
		for _, p := range tc.perms {
			sc.perm(p.card, p.ctrl)
		}
		for i := 0; i < 9; i++ {
			sc.hand(v.Plains, 0)
		}
		sc.s.step = kop.StepCleanup
		sc.s.ctl = ctlStepBegin
		sc.s.Step()
		got := 0
		if sc.s.ctl == ctlCleanupDiscard {
			got = int(sc.s.discardLeft)
		}
		if got != tc.want {
			t.Errorf("%s: discardLeft %d, want %d", tc.name, got, tc.want)
		}
	}
}
