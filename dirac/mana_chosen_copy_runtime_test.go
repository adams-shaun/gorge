package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCopiedQuirionElvesKeepsFixedMana: a permanent that becomes a copy of
// Quirion Elves (or a token copy) never made the card's as-enters colour
// choice, and the choice is no copiable value (CR 707.6), so its chosen
// word is unset. The card prints two mana abilities -- "{T}: Add {G}" and
// "{T}: Add one mana of the chosen color" -- and only the second one depends
// on the choice: the fixed {G} production is still legal and must be priced.
// manaPCGen once returned noManaPC for the whole source, hiding the fixed
// {G} beside the chosen one (pcard.manaFixedChosen).
func TestCopiedQuirionElvesKeepsFixedMana(t *testing.T) {
	set := cardsettest.Synth(t, "dr-mbc-bbaa")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := func(name string) CardID {
		for c := range tbl.Cards {
			if tbl.Cards[c].Name == name {
				return CardID(c)
			}
		}
		t.Fatalf("%s not in the dr-mbc-bbaa table", name)
		return 0
	}
	quirion, plains := id("Quirion Elves"), id("Plains")
	sc := newScenTableLib(t, tbl, plains, Config{NoElide: true})
	q := sc.perm(quirion, 0)
	sl := sc.slot(q)
	sc.s.perms[sl].chosen = 255 // a copy records no as-enters choice (CR 707.6)

	// Precondition: the table lowered Quirion Elves' two-ability pair, a
	// plain fixed {G} first and the chosen-colour twin second.
	first, pair := -1, false
	for j := range tbl.Cards[quirion].Abilities {
		if tbl.Cards[quirion].Abilities[j].Kind != kop.AbMana {
			continue
		}
		if first < 0 {
			first = j
		} else if kop.ChosenSecondManaAb(&tbl.Cards[quirion], first, j) {
			pair = true
			break
		}
	}
	if !pair {
		t.Fatalf("Quirion Elves is not the fixed {G} + chosen pair in the table")
	}

	pc := sc.s.manaPCGen(sc.k, sl)
	if !pc.manaChosen || !pc.manaFixedChosen {
		t.Fatalf("mana facts chosen %v fixed-chosen %v ability %d, want the chosen twin beside the fixed {G}",
			pc.manaChosen, pc.manaFixedChosen, pc.manaAb)
	}
	if got := sc.s.srcMaskAt(pc, sl); got != 1<<kop.G {
		t.Fatalf("choiceless copy prices mask %#x, want the fixed {G}", got)
	}
	if !sc.s.canTapForMana(sc.k, sl) {
		t.Fatal("choiceless copy is not a usable mana source: the fixed {G} ability is still legal")
	}
}

// TestWindowGapRestrictedComboChosenKeepsFixedColour: the manual payment
// window's restricted-source arm (windowGap's gfRestr branch) reads the
// restricted ability directly, not through manaPCGen. gfRestr is set only
// when restrTapMana sees a nonzero ManaColors, so the abilities this arm
// meets are a fixed colour or a Combo <C> Chosen -- never a bare "Chosen".
// A Combo G Chosen source that chose no colour is still a window gap: its
// fixed {G} production is legal (CR 707.6 removes only the chosen one).
// A gate keyed on AfManaChosen alone refused it entirely.
func TestWindowGapRestrictedComboChosenKeepsFixedColour(t *testing.T) {
	tbl, id := tapsDeferShapeTable(t, func(*kop.Table, map[string]CardID) {})
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Types: kop.TCreature})
	gate := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Chosen Gate", Types: kop.TLand,
		Abilities: []kop.Ability{{
			Kind: kop.AbMana, AFlags: kop.AfManaChosen | kop.AfManaNonSpell,
			Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1,
			ManaRestrict: kop.FilterID(len(tbl.Filters) - 1), // the restricted tap shape (gfRestr)
		}}})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	o := sc.perm(gate, 0)
	sl := sc.slot(o)
	a := &tbl.Cards[gate].Abilities[sc.k.gen.restr(gate)]
	if a.ManaColors != kop.ColG || a.AFlags&kop.AfManaChosen == 0 {
		t.Fatalf("precondition: %+v is not the Combo G Chosen restricted shape", a)
	}

	sc.s.perms[sl].chosen = 255 // a copy records no as-enters choice (CR 707.6)
	if !sc.s.windowGap(sc.k, 0, sl) {
		t.Error("window skips a restricted Combo G Chosen source that chose nothing: its fixed {G} is still legal")
	}
	sc.s.perms[sl].chosen = 3 // red: the chosen production joins the fixed one
	if !sc.s.windowGap(sc.k, 0, sl) {
		t.Error("window skips a restricted chosen-colour source with a colour chosen")
	}
}
