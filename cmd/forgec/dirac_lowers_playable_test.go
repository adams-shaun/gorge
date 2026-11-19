package main

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestDiracLowersOnlyGorgePlayableCards holds gorge's lead over dirac on the
// report's own terms: every card kgen lowers is a card `forgec report` counts
// playable (Registry.Unsupported against effects.Supported, with the report's
// etbCopyShapeGate). dirac plays a card identically to gorge, decision by
// decision, so a card dirac lowers that gorge reports unplayable is one of two
// bugs: gorge plays it and the report undercounts (a keyword expanded in
// engine/cards but never RegisterNonAPI'd, as kw:Rampage was), or gorge cannot
// play it and dirac has nothing to match. Either way it is fixed, never listed.
//
// Only gorge's unplayable cards are lowered, so the check costs a few hundred
// lowerings, not the corpus.
func TestDiracLowersOnlyGorgePlayableCards(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	dir := testutil.CorpusDirOrSkip(t)
	sup := effects.Supported()
	missing := map[string][]string{}
	var names []string
	for _, c := range r.AllCards() {
		if len(c.Faces) == 0 || c.Faces[0].Name == "" {
			continue
		}
		miss := r.Unsupported(c, sup)
		miss = append(miss, etbCopyShapeGate(c)...)
		if len(miss) == 0 {
			continue
		}
		name := c.Faces[0].Name
		if _, ok := r.Lookup(name); !ok {
			continue
		}
		missing[name] = miss
		names = append(names, name)
	}
	if len(names) == 0 {
		t.Fatal("gorge reports every corpus card playable: the coverage walk is not reading the corpus")
	}
	slices.Sort(names)
	names = slices.Compact(names)
	got, err := kgen.SupportFor(dir, names)
	if err != nil {
		t.Fatal(err)
	}
	var bad []string
	for _, n := range names {
		s, ok := got[n]
		if !ok || s.Status == cardset.StatusNotLowered {
			continue
		}
		bad = append(bad, n+" (dirac "+s.Status.String()+"; gorge lacks "+strings.Join(missing[n], ", ")+")")
	}
	for _, b := range bad {
		t.Errorf("dirac lowers a card gorge reports unplayable: %s", b)
	}
	t.Logf("%d gorge-unplayable cards, %d lowered by dirac", len(names), len(bad))
}
