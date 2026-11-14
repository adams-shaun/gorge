package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPrintedProtection: printed protection from a colour (CardDef.ProtColors,
// CR 702.16; gorge protectedFrom) stops blocking by and damage from a source
// of that colour, not of another; none once the bearer lost its abilities.
func TestPrintedProtection(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	knight, black, green := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Knight", Types: kop.TCreature, Power: 2, Toughness: 2, Colors: kop.ColW, ProtColors: kop.ColB},
		kop.CardDef{Name: "Test Black Bear", Types: kop.TCreature, Power: 2, Toughness: 2, Colors: kop.ColB},
		kop.CardDef{Name: "Test Green Bear", Types: kop.TCreature, Power: 2, Toughness: 2, Colors: kop.ColG})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	a := sc.perm(knight, 0)
	bb := sc.perm(black, 1)
	gb := sc.perm(green, 1)
	if sc.s.canBlock(sc.k, sc.slot(bb), sc.slot(a)) {
		t.Fatal("a black creature blocked protection from black")
	}
	if !sc.s.canBlock(sc.k, sc.slot(gb), sc.slot(a)) {
		t.Fatal("a green creature could not block protection from black")
	}
	if n := sc.s.replDamage(sc.k, 1, sc.slot(bb), black, TCode(a), false, 3); n != 0 {
		t.Fatalf("damage from a black source: %d, want 0 (prevented)", n)
	}
	if n := sc.s.replDamage(sc.k, 1, sc.slot(gb), green, TCode(a), false, 3); n != 3 {
		t.Fatalf("damage from a green source: %d, want 3", n)
	}
	sc.s.perms[sc.slot(a)].ovr.flags |= ovLoseAllEOT
	if !sc.s.canBlock(sc.k, sc.slot(bb), sc.slot(a)) {
		t.Fatal("lost its abilities: want blockable")
	}
}

// TestPrintedProtectionQuality: printed K:Protection:<spec> (CardDef.ProtFrom)
// stops blocking by and damage from a source matching the filter
// (protection from artifacts: an artifact creature, not a plain one).
func TestPrintedProtectionQuality(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TArtifact})
	f := kop.FilterID(len(tbl.Filters) - 1)
	chosen, golem, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Chosen", Types: kop.TCreature, Power: 2, Toughness: 2, ProtFrom: f},
		kop.CardDef{Name: "Test Golem", Types: kop.TCreature | kop.TArtifact, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	a := sc.perm(chosen, 0)
	g := sc.perm(golem, 1)
	b := sc.perm(bear, 1)
	if sc.s.canBlock(sc.k, sc.slot(g), sc.slot(a)) {
		t.Fatal("an artifact creature blocked protection from artifacts")
	}
	if !sc.s.canBlock(sc.k, sc.slot(b), sc.slot(a)) {
		t.Fatal("a plain creature could not block")
	}
	if n := sc.s.replDamage(sc.k, 1, sc.slot(g), golem, TCode(a), false, 3); n != 0 {
		t.Fatalf("damage from an artifact: %d, want 0", n)
	}
	if n := sc.s.replDamage(sc.k, 1, NoSlot, golem, TCode(a), false, 3); n != 0 {
		t.Fatalf("damage from a departed artifact source: %d, want 0", n)
	}
	if n := sc.s.replDamage(sc.k, 1, sc.slot(b), bear, TCode(a), false, 3); n != 3 {
		t.Fatalf("damage from a plain creature: %d, want 3", n)
	}
}
