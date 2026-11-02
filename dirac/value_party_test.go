package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPartySize: the most roles different creatures fill (gorge
// partySize): a Cleric-Wizard and a Cleric fill two; a changeling fills
// any one more, even a role outside the matchup's subtypes.
func TestPartySize(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	base := len(tbl.Subtypes)
	tbl.Subtypes = append(append([]string(nil), tbl.Subtypes...), "Cleric", "Wizard")
	var cw, cl kop.SubMask
	cw[base>>6] |= 1 << (base & 63)
	cw[(base+1)>>6] |= 1 << ((base + 1) & 63)
	cl[base>>6] |= 1 << (base & 63)
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Cleric Wizard", Types: kop.TCreature, Power: 1, Toughness: 1, Subtypes: cw},
		kop.CardDef{Name: "Test Cleric", Types: kop.TCreature, Power: 1, Toughness: 1, Subtypes: cl},
		kop.CardDef{Name: "Test Changeling", Types: kop.TCreature, Power: 1, Toughness: 1, AllCreatureTypes: true})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil {
		t.Skip("no generated prep")
	}
	sc.perm(n, 0)
	if got := sc.s.partySize(sc.k, 0); got != 1 {
		t.Fatalf("one Cleric-Wizard: party %d", got)
	}
	sc.perm(n+1, 0)
	if got := sc.s.partySize(sc.k, 0); got != 2 {
		t.Fatalf("Cleric-Wizard and Cleric: party %d", got)
	}
	sc.perm(n+2, 0)
	if got := sc.s.partySize(sc.k, 0); got != 3 {
		t.Fatalf("with a changeling: party %d", got)
	}
	if got := sc.s.partySize(sc.k, 1); got != 0 {
		t.Fatalf("the opponent's party %d", got)
	}
}
