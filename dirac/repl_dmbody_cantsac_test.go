package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Dralnu, Lich Lord's sacrifice body beside a CantSacrifice static. gorge's
// effSacrifice drops the forbidden permanents from the eligible set before it
// counts them (cnt > n decides the ask), so the body's count, its no-ask
// stand-in and its "fewer remain, all are sacrificed" arms read the same set
// the options do (sacPool). A test card carries the static: "you can't
// sacrifice lands you control".

// TestDmgBodySacPoolSkipsCantSacrifice: with Dralnu, the test card and three
// Mountains on seat 0, the body owes from a pool of two (Dralnu and the card).
// Two damage is then owed in full with no ask and takes exactly those two; one
// damage asks, offering those two and no Mountain.
func TestDmgBodySacPoolSkipsCantSacrifice(t *testing.T) {
	deck := []cardset.Entry{{Name: "Dralnu, Lich Lord", Count: 1}, {Name: "Walking Corpse", Count: 19}, {Name: "Swamp", Count: 12}, {Name: "Mountain", Count: 8}}
	set := cardsettest.Set(t, "repl-dmgaskbodies")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tb := *tbl
	tb.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Types: kop.TLand, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	qid := CardID(len(tb.Cards))
	tb.Cards = append(append([]kop.CardDef(nil), tbl.Cards...), kop.CardDef{Name: "Test No-Sacrifice", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbStatic,
			Static: kop.Static{Kind: kop.StatCantSacrifice, Filter: kop.FilterID(len(tb.Filters) - 1)}}}})
	sc := newScenTable(t, &tb, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyCantSac {
		t.Fatal("precondition: the table's generated extension has no CantSacrifice static")
	}
	s, k := sc.s, sc.k
	dralnu := sc.perm(id["Dralnu, Lich Lord"], 0)
	card := sc.perm(qid, 0)
	var lands [3]Obj
	for i := range lands {
		lands[i] = sc.perm(id["Mountain"], 0)
	}
	if !s.sacBlocked(k, s.objs[lands[0]].slot) || s.sacBlocked(k, s.objs[dralnu].slot) {
		t.Fatal("precondition: the static forbids the Mountains and not Dralnu")
	}
	if n := s.bodySacLeft(k, 0); n != 2 {
		t.Fatalf("the body owes from %d permanents beside the static, want 2 (Dralnu and the card)", n)
	}
	if !s.sacNoAsk(k, 0, 2, false) || s.sacNoAsk(k, 0, 1, false) {
		t.Fatal("two damage is owed in full with no ask and one damage asks: gorge's eligible count is two")
	}
	var d Decision
	s.replBodyOptions(&d)
	if d.N != 2 {
		t.Fatalf("%d options posed, want Dralnu and the card", d.N)
	}
	s.sacFirst(k, 0, 5)
	for _, o := range lands {
		if s.objs[o].zone != kop.ZoneBattlefield {
			t.Fatal("a Mountain the static protects was sacrificed")
		}
	}
	if s.objs[dralnu].zone == kop.ZoneBattlefield || s.objs[card].zone == kop.ZoneBattlefield {
		t.Fatal("the unprotected permanents were not sacrificed")
	}
}
