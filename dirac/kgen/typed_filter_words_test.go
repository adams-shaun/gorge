package kgen

import (
	"errors"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/forge"
	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// TestTypedFilterCommanderWords: the controller and permanent-card words the
// Commander pools carry take the typed path, and read as the string reader
// reads them.
func TestTypedFilterCommanderWords(t *testing.T) {
	known := func(string) bool { return false }
	isType := func(w string) bool { _, ok := typeBits[w]; return ok }
	for _, tc := range []struct{ text, without string }{
		{"Card.nonLand+Permanent+YouOwn", "Card.nonLand+YouOwn"},
		{"Card.Permanent+YouCtrl", "Card.YouCtrl"},
		{"Creature.RememberedPlayerCtrl", "Creature"},
		{"Land.nonBasic+RememberedPlayerCtrl", "Land.nonBasic"},
		{"Creature.ControlledBy Player.IsRemembered", "Creature"},
		{"Creature.!IsRemembered+ControlledBy Player.IsRemembered", "Creature.!IsRemembered"},
		{"Artifact.EnchantedPlayerCtrl", "Artifact"},
		{"Creature.nonLegendary+EnchantedPlayerCtrl", "Creature.nonLegendary"},
	} {
		want, werr := parseFilter(tc.text, known, isType)
		base, berr := parseFilter(tc.without, known, isType)
		if werr != nil || berr != nil || reflect.DeepEqual(want, base) {
			t.Fatalf("precondition: the string reader reads %q (%v) unlike %q (%v)", tc.text, werr, tc.without, berr)
		}
		f, ref := forge.LowerFilter(tc.text)
		if ref != nil {
			t.Errorf("%q: front end refusal %v", tc.text, ref)
			continue
		}
		if err := ir.ValidateFilter(f); err != nil {
			t.Errorf("%q: %v", tc.text, err)
		}
		var st filterStats
		got, gerr := readFilterTyped(tc.text, known, isType, &st)
		if gerr != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%q:\n typed  %v (%v)\n string %v", tc.text, got, gerr, want)
		}
		if st.typed != 1 {
			t.Errorf("%q: stats %+v, want one typed read", tc.text, st)
		}
	}

	// negations and the combinations a clause cannot hold have no typed
	// reading or the same refusal: the answer is the string reader's
	for _, text := range []string{
		"Card.!Permanent", "Instant.Permanent", "Creature.!RememberedPlayerCtrl",
		"Creature.!EnchantedPlayerCtrl", "Creature.YouCtrl+RememberedPlayerCtrl",
		"Creature.modified+EnchantedPlayerCtrl",
	} {
		if _, ref := forge.LowerFilter(text); ref != nil {
			t.Errorf("%q: front end refusal %v", text, ref)
			continue
		}
		var st filterStats
		got, gerr := readFilterTyped(text, known, isType, &st)
		want, werr := parseFilter(text, known, isType)
		switch {
		case (gerr == nil) != (werr == nil), gerr != nil && gerr.Error() != werr.Error():
			t.Errorf("%q: typed path error %v, string reader %v", text, gerr, werr)
		case gerr == nil && !reflect.DeepEqual(got, want):
			t.Errorf("%q:\n typed  %v\n string %v", text, got, want)
		}
	}

	// the owner twin has no typed mapping: it is not read as a controller
	f, ref := forge.LowerFilter("Creature.RememberedPlayerCtrl")
	if ref != nil {
		return // reported above
	}
	owner := f.Args[len(f.Args)-1]
	if owner.Pred != ir.PControlledBy {
		t.Fatalf("precondition: the last conjunct of %v is the controller word", f)
	}
	owner.Pred = ir.POwnedBy
	if _, err := lFilterFromIR(f, known); !errors.Is(err, errNoTypedReading) {
		t.Errorf("owned by a remembered player: lFilterFromIR = %v, want errNoTypedReading", err)
	}
}
