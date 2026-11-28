package kgen

import (
	"bytes"
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/forge"
)

// TestTypedFilterAttachedByFallsBack: the front end reads EnchantedBy,
// EquippedBy and AttachedBy as one node, the string reader builds two flags
// that later checks tell apart, so the positive node has no typed reading
// and the read is the string reader's.
func TestTypedFilterAttachedByFallsBack(t *testing.T) {
	known := func(string) bool { return false }
	isType := func(w string) bool { _, ok := typeBits[w]; return ok }
	for _, text := range []string{"Creature.EquippedBy", "Card.AttachedBy", "Creature.EnchantedBy"} {
		f, ref := forge.LowerFilter(text)
		if ref != nil {
			t.Fatalf("precondition: the front end reads %q (refusal %v)", text, ref)
		}
		if _, err := lFilterFromIR(f, known); !errors.Is(err, errNoTypedReading) {
			t.Errorf("%q: lFilterFromIR = %v, want errNoTypedReading", text, err)
		}
		var st filterStats
		got, gerr := readFilterTyped(text, known, isType, &st)
		want, werr := parseFilter(text, known, isType)
		if gerr != nil || werr != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%q: typed path %v (%v), string %v (%v)", text, got, gerr, want, werr)
		}
		if st.fallbackReader != 1 || st.typed != 0 {
			t.Errorf("%q: stats %+v, want one reader fallback", text, st)
		}
	}
}

// TestTypedFilterRefusalText: a capacity refusal the typed reader decides
// carries the string reader's text, the alternative's prefix included.
func TestTypedFilterRefusalText(t *testing.T) {
	known := func(string) bool { return false }
	isType := func(w string) bool { _, ok := typeBits[w]; return ok }
	for _, text := range []string{"Creature.modified+blocked+equipped", "Creature.YouCtrl+OppCtrl", "Creature.White+Blue"} {
		f, ref := forge.LowerFilter(text)
		if ref != nil {
			t.Fatalf("precondition: the front end reads %q (refusal %v)", text, ref)
		}
		_, terr := lFilterFromIR(f, known)
		_, werr := parseFilter(text, known, isType)
		if terr == nil || errors.Is(terr, errNoTypedReading) || werr == nil {
			t.Fatalf("precondition: %q is a capacity refusal on both readers (typed %v, string %v)", text, terr, werr)
		}
		var st filterStats
		_, gerr := readFilterTyped(text, known, isType, &st)
		if gerr == nil || gerr.Error() != werr.Error() {
			t.Errorf("%q: typed path refuses %v, string reader %q", text, gerr, werr)
		}
	}
}

// TestTypedFilterTablesByteEqualEDH is the byte-equality check on edh, the
// pool whose table the attached-by collapse changed.
func TestTypedFilterTablesByteEqualEDH(t *testing.T) {
	cardsDir := tableCorpus(t)
	strTab, strSup, _ := typedPoolTable(t, "edh", cardsDir, filterReadString)
	typTab, typSup, st := typedPoolTable(t, "edh", cardsDir, filterReadTyped)
	if !bytes.Equal(strTab, typTab) {
		t.Errorf("table differs: %d bytes string, %d typed, first diff at byte %d", len(strTab), len(typTab), firstDiff(strTab, typTab))
	}
	if !slices.Equal(strSup, typSup) {
		t.Errorf("supported lists differ: %d string, %d typed", len(strSup), len(typSup))
	}
	if st.typed == 0 {
		t.Fatal("no typed read: the comparison read nothing through the typed path")
	}
	t.Logf("edh: typed %d, fallbackFrontEnd %d, fallbackReader %d", st.typed, st.fallbackFrontEnd, st.fallbackReader)
}
