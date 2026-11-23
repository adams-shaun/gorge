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
		ref  kop.Ref
	}{
		{"Creature.withToxic", kop.FwWithKeyword, int16(kop.KwToxic), false, 0},
		{"Creature.withoutToxic", kop.FwWithKeyword, int16(kop.KwToxic), true, 0},
		{"Creature.!withToxic", kop.FwWithKeyword, int16(kop.KwToxic), true, 0},
		{"Card.withHorsemanship", kop.FwWithKeyword, int16(kop.KwHorsemanship), false, 0},
		{"Card.withMorph", kop.FwWithKeyword, int16(kop.KwMorph), false, 0},
		{"Card.withMegamorph", kop.FwWithKeyword, int16(kop.KwMegamorph), false, 0},
		{"Card.withoutSuspend", kop.FwWithKeyword, int16(kop.KwSuspend), true, 0},
		{"Card.withCascade", kop.FwWithKeyword, int16(kop.KwCascade), false, 0},
		{"Card.withConvoke", kop.FwWithKeyword, int16(kop.KwConvoke), false, 0},
		{"Card.withForetell", kop.FwWithKeyword, int16(kop.KwForetell), false, 0},
		{"Card.withMadness", kop.FwWithKeyword, int16(kop.KwMadness), false, 0},
		{"Card.withMultikicker", kop.FwWithKeyword, int16(kop.KwMultikicker), false, 0},
		{"Card.ChosenCtrl", kop.FwChosenCtrl, 0, false, 0},
		{"Card.EnchantedControllerCtrl", kop.FwEnchantedControllerCtrl, 0, false, 0},
		// the printed records kgen keeps for the keywords with no KwMask bit
		// (T13b-2): the face's own marker, set at the keyword's lowering arm
		{"Card.withFlanking", kop.FwWithKeyword, int16(kop.KwFlanking), false, 0},
		{"Card.withoutFlanking", kop.FwWithKeyword, int16(kop.KwFlanking), true, 0},
		{"Card.withCycling", kop.FwWithKeyword, int16(kop.KwCycling), false, 0},
		{"Card.withoutCycling", kop.FwWithKeyword, int16(kop.KwCycling), true, 0},
		{"Card.withDisturb", kop.FwWithKeyword, int16(kop.KwDisturb), false, 0},
		{"Card.withMutate", kop.FwWithKeyword, int16(kop.KwMutate), false, 0},
		{"Card.withDecayed", kop.FwWithKeyword, int16(kop.KwDecayed), false, 0},
		{"Card.withPersist", kop.FwWithKeyword, int16(kop.KwPersist), false, 0},
		{"Card.withPhasing", kop.FwWithKeyword, int16(kop.KwPhasing), false, 0},
		{"Card.withUnearth", kop.FwWithKeyword, int16(kop.KwUnearth), false, 0},
		{"Card.withWard", kop.FwWithKeyword, int16(kop.KwPrintedWard), false, 0},
		{"Card.withEnchant", kop.FwWithKeyword, int16(kop.KwEnchant), false, 0},
		{"Card.withDoctor'scompanion", kop.FwWithKeyword, int16(kop.KwDoctorsCompanion), false, 0},
		// the referent words (T13b-2): each carries its referent in R
		{"Creature.sharesCreatureTypeWith Targeted", kop.FwSharesCr, 0, false, kop.RefTargets0123},
		{"Creature.!sharesCreatureTypeWith Targeted", kop.FwSharesCr, 0, true, kop.RefTargets0123},
		{"Creature.sharesCreatureTypeWith Self", kop.FwSharesCr, 0, false, kop.RefSelf},
		{"Creature.sharesCreatureTypeWith RememberedCard", kop.FwSharesCr, 0, false, kop.RefRemembered},
		{"Creature.sharesCreatureTypeWith Imprinted", kop.FwSharesCr, 0, false, kop.RefImprinted},
		{"Card.sharesNameWith Targeted", kop.FwSharesName, 0, false, kop.RefTargets0123},
		{"Card.sharesNameWith RememberedCard", kop.FwSharesName, 0, false, kop.RefRemembered},
		{"Card.sharesNameWith TriggeredCard", kop.FwSharesName, 0, false, kop.RefTriggerObj},
		{"Card.sharesNameWith TriggeredCardLKICopy", kop.FwSharesName, 0, false, kop.RefTriggerObj},
	} {
		zones := []kop.Zone{kop.ZoneBattlefield}
		if tc.w == kop.FwWithKeyword {
			zones = append(zones, kop.ZoneGraveyard, kop.ZoneHand) // the card-zone read
		}
		if tc.w == kop.FwSharesCr || tc.w == kop.FwSharesName {
			zones = append(zones, kop.ZoneGraveyard)
		}
		for _, z := range zones {
			lits := relationsProg(t, NewAtomCompiler(), tc.spec, z)
			if len(lits) != 1 || lits[0].W != tc.w || lits[0].Arg != tc.arg || lits[0].Neg != tc.neg || lits[0].R != tc.ref {
				t.Errorf("%s: literals %+v, want word %d arg %d neg %v ref %d", tc.spec, lits, tc.w, tc.arg, tc.neg, tc.ref)
			}
		}
	}
	// a comparison already in the clause moves into the program beside the word
	lits := relationsProg(t, NewAtomCompiler(), "Creature.powerGE2+withToxic", kop.ZoneBattlefield)
	if len(lits) != 2 || lits[0].W != kop.FwPowerCmp || lits[1].W != kop.FwWithKeyword {
		t.Errorf("powerGE2+withToxic: literals %+v", lits)
	}
	// plain sameName (gorge wordSameName: card.sharesNameWith(source), the
	// resolving ability's source by default) is the struct's sharesNameWith-Self
	// pred, with the ordinary ! negation
	for _, tc := range []struct {
		spec string
		neg  bool
	}{{"Creature.sameName", false}, {"Creature.!sameName", true}} {
		a := NewAtomCompiler()
		id, err := a.Filter(tc.spec, kop.ZoneBattlefield)
		if err != nil {
			t.Errorf("%s: %v", tc.spec, err)
			continue
		}
		fs, _ := a.Tables()
		if fs[id].Pred != kop.PredSameNameSelf || (fs[id].PredN == 1) != tc.neg {
			t.Errorf("%s: pred %d predN %d", tc.spec, fs[id].Pred, fs[id].PredN)
		}
	}
}

// TestFilterWordsRelationsRefuse: the spellings gorge reads otherwise, or the
// kernel keeps no record of, stay unknown properties and so refuse.
func TestFilterWordsRelationsRefuse(t *testing.T) {
	for _, spec := range []string{
		// the referents gorge reads whose object the kernel keeps no record of
		// (the convoked creatures ride stackEntry.prov as a count, never IDs)
		// and the two gorge's own sharesTypeArg never accepts (Sacrificed,
		// Enchanted)
		"Creature.sharesCreatureTypeWith Convoked", "Creature.sharesCreatureTypeWith Sacrificed",
		"Creature.sharesCreatureTypeWith Enchanted",
		// the sameName base-prefix forms read the base's object, not the
		// source (gorge sameNameContextBase); the kernel keeps no read of them
		"Creature.Remembered+sameName", "Creature.Targeted+sameName", "Creature.Triggered+sameName",
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
