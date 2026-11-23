package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// relationsProg lowers spec over zone and returns the filter-program literals of its
// head filter (nil when the head is no program).
func relationsProg(t *testing.T, a *AtomCompiler, spec string, zone kop.Zone) []kop.FLit {
	t.Helper()
	id, err := a.Filter(spec, zone)
	if err != nil {
		t.Fatalf("%s in zone %d: %v", spec, zone, err)
	}
	fs, _ := a.Tables()
	if fs[id].Pred != kop.PredProg {
		return nil
	}
	return a.Progs()[fs[id].PredN].Lits
}

// TestFilterWordsRelations: each relation and keyword word lowers to its one
// literal, with the keyword's KwID for the printed-keyword words and Neg for
// the negated spellings, in a battlefield and a card zone.
func TestFilterWordsRelations(t *testing.T) {
	for _, tc := range []struct {
		spec string
		w    kop.FilterWord
		arg  int16
		neg  bool
	}{
		{"Creature.withToxic", kop.FwWithKeyword, int16(kop.KwToxic), false},
		{"Creature.withoutToxic", kop.FwWithKeyword, int16(kop.KwToxic), true},
		{"Creature.!withToxic", kop.FwWithKeyword, int16(kop.KwToxic), true},
		{"Card.withHorsemanship", kop.FwWithKeyword, int16(kop.KwHorsemanship), false},
		{"Card.withMorph", kop.FwWithKeyword, int16(kop.KwMorph), false},
		{"Card.withMegamorph", kop.FwWithKeyword, int16(kop.KwMegamorph), false},
		{"Card.withoutSuspend", kop.FwWithKeyword, int16(kop.KwSuspend), true},
		{"Card.withCascade", kop.FwWithKeyword, int16(kop.KwCascade), false},
		{"Card.withConvoke", kop.FwWithKeyword, int16(kop.KwConvoke), false},
		{"Card.withForetell", kop.FwWithKeyword, int16(kop.KwForetell), false},
		{"Card.withMadness", kop.FwWithKeyword, int16(kop.KwMadness), false},
		{"Card.withMultikicker", kop.FwWithKeyword, int16(kop.KwMultikicker), false},
		{"Card.ChosenCtrl", kop.FwChosenCtrl, 0, false},
		{"Card.EnchantedControllerCtrl", kop.FwEnchantedControllerCtrl, 0, false},
	} {
		zones := []kop.Zone{kop.ZoneBattlefield}
		if tc.w == kop.FwWithKeyword {
			zones = append(zones, kop.ZoneGraveyard, kop.ZoneHand) // the card-zone read
		}
		for _, z := range zones {
			lits := relationsProg(t, NewAtomCompiler(), tc.spec, z)
			if len(lits) != 1 || lits[0].W != tc.w || lits[0].Arg != tc.arg || lits[0].Neg != tc.neg {
				t.Errorf("%s: literals %+v, want word %d arg %d neg %v", tc.spec, lits, tc.w, tc.arg, tc.neg)
			}
		}
	}
	// a comparison already in the clause moves into the program beside the word
	lits := relationsProg(t, NewAtomCompiler(), "Creature.powerGE2+withToxic", kop.ZoneBattlefield)
	if len(lits) != 2 || lits[0].W != kop.FwPowerCmp || lits[1].W != kop.FwWithKeyword {
		t.Errorf("powerGE2+withToxic: literals %+v", lits)
	}
}

// TestFilterWordsRelationsRefuse: the spellings gorge reads otherwise, or the
// kernel keeps no record of, stay unknown properties and so refuse.
func TestFilterWordsRelationsRefuse(t *testing.T) {
	for _, spec := range []string{
		"Creature.withoutFlanking", "Card.withCycling", "Card.withDisturb", // no printed record
		"Card.!ChosenCtrl", "Card.!EnchantedControllerCtrl", // gorge refuses the '!' of the link words
		"Creature.PairedWith", "Creature.!Paired", // need T10's pair record
		"Creature.leastPower", "Card.CanEnchantSource", // no gorge reader
	} {
		if _, err := NewAtomCompiler().Filter(spec, kop.ZoneBattlefield); err == nil {
			t.Errorf("%s lowered", spec)
		}
	}
	// the card zones: a word the manifest declares for perm only is refused there
	for _, spec := range []string{"Card.ChosenCtrl", "Card.EnchantedControllerCtrl"} {
		if _, err := NewAtomCompiler().Filter(spec, kop.ZoneGraveyard); err == nil {
			t.Errorf("%s lowered in a card zone", spec)
		}
	}
	// withFlash is the struct's PredHasKeyword: refused on the battlefield, read off it
	if _, err := NewAtomCompiler().Filter("Card.withFlash", kop.ZoneBattlefield); err == nil {
		t.Errorf("withFlash lowered on the battlefield")
	}
	if _, err := NewAtomCompiler().Filter("Card.withFlash", kop.ZoneHand); err != nil {
		t.Errorf("withFlash in hand: %v", err)
	}
}
