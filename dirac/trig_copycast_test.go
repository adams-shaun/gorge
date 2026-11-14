package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSpellCastOrCopy: a Tf2OrCopy SpellCast line fires for the cast and
// again for Thousand-Year Storm's copy (gorge Mode$ SpellCastOrCopy on
// StackCopy); a plain SpellCast line only for the cast.
func TestSpellCastOrCopy(t *testing.T) {
	deck := []cardset.Entry{{Name: "Thousand-Year Storm", Count: 1}, {Name: "Think Twice", Count: 1}, {Name: "Island", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TInstant | kop.TSorcery})
	is := kop.FilterID(len(tbl.Filters) - 1)
	gain := func(n int) []kop.Instr {
		return []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(n)}}
	}
	w := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Magecraft", Types: kop.TEnchantment, Abilities: []kop.Ability{
		{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigSpellCast, Filter: is, Whose: kop.WhoseYou, Flags2: kop.Tf2OrCopy}, Prog: gain(1)},
		{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigSpellCast, Filter: is, Whose: kop.WhoseYou}, Prog: gain(100)}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyOrCopy {
		t.Fatal("genPrep.anyOrCopy not set")
	}
	s := sc.s
	sc.perm(id["Thousand-Year Storm"], 0)
	sc.perm(w, 0)
	s.g.isCast[0] = 1
	sc.lands(0, id["Island"], 2)
	life := s.pl[0].life
	tt := sc.hand(id["Think Twice"], 0)
	sc.do(t, AkCast, TCode(tt), scAny)
	for i := 0; i < 60; i++ {
		d := s.Step()
		if d.Kind == DkNone {
			t.Fatal("the game ended")
		}
		if d.Kind == DkPriority && s.sp == 0 && s.objs[tt].zone == kop.ZoneGraveyard {
			break
		}
		s.Apply(d.Options[0])
	}
	// the cast: 1 + 100; Storm's copy: 1
	if got := s.pl[0].life - life; got != 102 {
		t.Fatalf("gained %d life, want 102 (both lines on the cast, the or-copy line on the copy)", got)
	}
}
