package main

import (
	"bytes"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestExclusionRegister holds the exclusion register (excluded.go) honest:
// every entry names a real corpus card that gorge cannot play, cites the
// paper rule that bans it, and names only primitives the engine really
// lacks. The register can only shrink: when gorge ever supports a card the
// register excludes, this test fails until the entry is removed — and an
// entry added for a card gorge can already play fails the same way.
//
// The paper-rule half is checked against what gorge itself can see:
//   - an "ante" entry's card carries the ante keyword its script prints
//     ("Remove CARDNAME from your deck before playing if you're not playing
//     for ante.", CR 407's marker on the script);
//   - a "dexterity" or "subgame" entry's card is on the machine-readable
//     Commander ban list (deck.CommanderBannedNames), gorge's one compiled
//     paper-ban list.
func TestExclusionRegister(t *testing.T) {
	// Register hygiene: one entry per card, sorted, every field filled and
	// every cause one of the three diagnosed classes.
	names := make([]string, 0, len(excludedRegister))
	for _, e := range excludedRegister {
		names = append(names, e.Name)
		if e.Cause != "ante" && e.Cause != "dexterity" && e.Cause != "subgame" {
			t.Errorf("%s: unknown cause %q", e.Name, e.Cause)
		}
		if e.Citation == "" {
			t.Errorf("%s: empty paper-rule citation", e.Name)
		}
		if len(e.Prims) == 0 {
			t.Errorf("%s: no missing primitives named", e.Name)
		}
	}
	if !slices.IsSorted(names) {
		t.Errorf("register is not sorted by name: %v", names)
	}
	if dup := len(names) - len(slices.Compact(slices.Clone(names))); dup != 0 {
		t.Errorf("register names a card twice")
	}

	dir := testutil.CorpusDirOrSkip(t)
	r := testutil.CorpusRegistry(t)
	r.ShapeGate = etbCopyShapeGate
	sup := effects.Supported()

	for _, e := range excludedRegister {
		c, ok := r.Lookup(e.Name)
		if !ok {
			t.Errorf("%s: not in the corpus (renamed? dropped by a pin move?)", e.Name)
			continue
		}
		miss := r.Unsupported(c, sup)
		if len(miss) == 0 {
			t.Errorf("%s: gorge can play it — remove its register entry (the register can only shrink)", e.Name)
			continue
		}
		supported := map[string]bool{}
		for _, m := range miss {
			supported[m] = true
		}
		for _, p := range e.Prims {
			if !supported[p] {
				t.Errorf("%s: register cites %q, which the engine now supports; update or remove the entry", e.Name, p)
			}
		}
		// The paper rule, as far as gorge can see it (see the doc comment).
		switch e.Cause {
		case "ante":
			if !carriesAnteKeyword(c) {
				t.Errorf("%s: cause ante but the script lost its ante keyword", e.Name)
			}
		default:
			if !deck.IsCommanderBanned(c) {
				t.Errorf("%s: cause %s but not on the Commander ban list", e.Name, e.Cause)
			}
		}
	}

	// The coverage wiring the report relies on: with the hook set, every
	// register entry drops out of Cards/Supported/Missing and is counted in
	// Coverage.Excluded instead — exactly as many cards as the register
	// names, none of them contributing to the gap census.
	r.Excluded = paperExcluded
	cv := r.Coverage(sup)
	if cv.Excluded != len(excludedRegister) {
		t.Errorf("Coverage.Excluded = %d, want %d (a register entry does not resolve to a corpus card)", cv.Excluded, len(excludedRegister))
	}

	// The report prints the excluded cards on their own line.
	var out bytes.Buffer
	if err := reportTo(&out, dir, 5); err != nil {
		t.Fatal(err)
	}
	excluded, detail := excludedCauseCounts()
	want := "excluded: " + strconv.Itoa(excluded) + detail + "\n"
	if !strings.Contains(out.String(), want) {
		t.Errorf("report output lacks the excluded line %q; got:\n%s", want, out.String())
	}
}

// carriesAnteKeyword reports whether the card's script prints the ante
// keyword ("K:Remove CARDNAME from your deck before playing if you're not
// playing for ante."), CR 407's marker.
func carriesAnteKeyword(c *cards.Card) bool {
	for _, f := range c.Faces {
		for _, k := range f.Keywords {
			if strings.HasPrefix(k, "Remove ") && strings.Contains(k, "ante") {
				return true
			}
		}
	}
	return false
}
