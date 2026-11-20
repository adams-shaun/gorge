package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCopiedFixedBesideChosenKeepsFixedMana: a permanent that becomes a copy
// of a source with two mana abilities -- "{T}: Add {G}" and "{T}: Add one
// mana of the chosen color" (Quirion Elves' shape) -- never made the card's
// as-enters colour choice, and the choice is no copiable value (CR 707.6),
// so its chosen word is unset. Only the second ability depends on the
// choice: the fixed {G} production is still legal and the actual-availability
// census must count it. A gate on manaChosen alone hid the fixed {G} beside
// the chosen one (pcard.manaFixedChosen).
func TestCopiedFixedBesideChosenKeepsFixedMana(t *testing.T) {
	const name = "Test Quirion Twin"
	tbl, id := tapsDeferShapeTable(t, func(tbl *kop.Table, id map[string]CardID) {
		id[name] = CardID(len(tbl.Cards))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: name, Types: kop.TLand,
			Abilities: []kop.Ability{
				{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1},
				{Kind: kop.AbMana, AFlags: kop.AfManaChosen, Cost: kop.Cost{Flags: kop.CostTap}, ManaN: 1},
			}})
	})
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	q := sc.perm(id[name], 0)
	sl := sc.slot(q)
	sc.s.perms[sl].chosen = 255 // a copy records no as-enters choice (CR 707.6)

	// Precondition: the pair is a plain fixed {G} first and the chosen-colour
	// twin second, the shape prepMultiMana reads.
	cd := &tbl.Cards[id[name]]
	if len(cd.Abilities) < 2 || !kop.ChosenSecondManaAb(cd, 0, 1) {
		t.Fatalf("precondition: %+v is not the fixed {G} + chosen pair", cd.Abilities)
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
	var av avail
	sc.s.availFor(sc.k, 0, &av)
	if av.total != 1 {
		t.Fatalf("census counts %d units, want the fixed {G} beside the unchosen production", av.total)
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
