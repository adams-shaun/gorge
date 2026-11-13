package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Ward—Sacrifice a creature beside a CantSacrifice static (CR 702.21a,
// 701.21a; gorge settleWardPayment pays through pay.PaySacrificeCost, whose
// candidates leave out what sacrificeBlocked forbids). The kernel's ward_sac
// candidates are sacMatchGen's, which reads StatCantSacrifice. No lowered card
// reaches the pair in a game (Quark, the one lowered carrier, forbids Treasure
// tokens, which no ward—sacrifice filter matches), so a test card carries the
// static: "you can't sacrifice creatures you control".

// TestWardSacSkipsCantSacrifice: Vein Ripper's ward pick for player 1 (two
// Grizzly Bears) offers both Bears; with the test card on player 1's side it
// offers neither, and with sacBlocked switched off both again.
func TestWardSacSkipsCantSacrifice(t *testing.T) {
	deck := []cardset.Entry{{Name: "Vein Ripper", Count: 1}, {Name: "Grizzly Bears", Count: 2}, {Name: "Swamp", Count: 12}}
	set := cardsettest.Set(t, "wardsac")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tb := *tbl
	tb.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Zone: kop.ZoneBattlefield})
	qid := CardID(len(tb.Cards))
	tb.Cards = append(append([]kop.CardDef(nil), tbl.Cards...), kop.CardDef{Name: "Test No-Sacrifice", Types: kop.TArtifact,
		Abilities: []kop.Ability{{Kind: kop.AbStatic,
			Static: kop.Static{Kind: kop.StatCantSacrifice, Filter: kop.FilterID(len(tb.Filters) - 1)}}}})
	if tb.Cards[id["Vein Ripper"]].WardSac == 0 {
		t.Fatal("precondition: Vein Ripper has no ward—sacrifice filter")
	}
	sc := newScenTable(t, &tb, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyCantSac {
		t.Fatal("precondition: the table's generated extension has no CantSacrifice static")
	}
	s := sc.s
	ripper := sc.perm(id["Vein Ripper"], 0)
	sc.perm(id["Grizzly Bears"], 1)
	sc.perm(id["Grizzly Bears"], 1)
	s.res.e.card, s.res.e.obj = id["Vein Ripper"], ripper
	s.g.pickP = 1
	if n := s.wardSacCands(sc.k, nil); n != 2 {
		t.Fatalf("precondition: %d ward pick candidates without the static, want the two Bears", n)
	}
	sc.perm(qid, 1)
	if n := s.wardSacCands(sc.k, nil); n != 0 {
		t.Fatalf("%d ward pick candidates under the static, want none (CR 701.21a)", n)
	}
	DebugCantSacOff = true
	defer func() { DebugCantSacOff = false }()
	if n := s.wardSacCands(sc.k, nil); n != 2 {
		t.Fatalf("%d candidates with sacBlocked off, want 2: the static is the cause", n)
	}
}
