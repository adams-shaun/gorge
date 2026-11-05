package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func fdnSheetNames(t *testing.T, reg *cards.Registry) []string {
	t.Helper()
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("setdata FDN: %v", err)
	}
	seen := map[string]bool{}
	for _, sheet := range spec.Config.Sheets {
		for _, c := range sheet.Cards {
			seen[c.Name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func fdnSpells(t *testing.T, reg *cards.Registry, colour uint8, n int, onePip bool) []string {
	t.Helper()
	var out []string
	for _, name := range fdnSheetNames(t, reg) {
		c, ok := reg.Lookup(name)
		if !ok || c == nil || len(c.Faces) == 0 {
			continue
		}
		f := c.Faces[0]
		if f.IsLand() || c.ColourIdentity() != colour {
			continue
		}
		if onePip {
			total := int32(0)
			for i := 0; i < 5; i++ {
				total += cards.ManaCostChroma(f.ManaCost, 1<<i)
			}
			if total != 1 {
				continue
			}
		} else if cards.ManaCostChroma(f.ManaCost, colour) == 0 {
			continue
		}
		out = append(out, name)
		if len(out) == n {
			break
		}
	}
	if len(out) < n {
		t.Fatalf("FDN has %d qualifying spells of colour %05b, want %d", len(out), colour, n)
	}
	return out
}

func expectedShape(t *testing.T, reg *cards.Registry, names []string) ([6]int, metaTypes) {
	t.Helper()
	var curve [6]int
	var types metaTypes
	for _, name := range names {
		c, ok := reg.Lookup(name)
		if !ok || c == nil || len(c.Faces) == 0 {
			t.Fatalf("corpus has no card %q", name)
		}
		f := c.Faces[0]
		if f.IsLand() {
			t.Fatalf("%q is a land, want a spell", name)
		}
		switch mv := int(f.Cmc()); {
		case mv <= 1:
			curve[0]++
		case mv >= 6:
			curve[5]++
		default:
			curve[mv-1]++
		}
		switch {
		case f.IsCreature():
			types.Creature++
		case f.IsInstant():
			types.Instant++
		case f.IsSorcery():
			types.Sorcery++
		case f.IsPlaneswalker():
			types.Planeswalker++
		case f.IsBattle():
			types.Battle++
		case f.IsEnchantment():
			types.Enchantment++
		case f.IsArtifact():
			types.Artifact++
		default:
			types.Other++
		}
	}
	return curve, types
}

func TestDescribeHandBuiltDeck(t *testing.T) {
	corpus := testutil.CorpusDirOrSkip(t)
	reg := testutil.CorpusRegistry(t)
	lab, err := draftlab.Open(corpus, draftlab.Options{})
	if err != nil {
		t.Fatal(err)
	}

	white := fdnSpells(t, reg, cards.ColourWhite, 10, false)
	blue := fdnSpells(t, reg, cards.ColourBlue, 10, false)
	red := fdnSpells(t, reg, cards.ColourRed, 2, true)
	spellNames := append(append(append([]string{}, white...), blue...), red...)

	pool := make([]limited.CardRef, 0, len(spellNames)+1)
	ids := make([]string, 0, len(spellNames))
	add := func(name string) string {
		id := fmt.Sprintf("c%d", len(pool))
		pool = append(pool, limited.CardRef{ID: id, Name: name, Set: "FDN", Rarity: limited.Common})
		return id
	}
	for _, name := range spellNames {
		ids = append(ids, add(name))
	}
	cove := add("Tranquil Cove")

	curve, types := expectedShape(t, reg, spellNames)
	d := draftlab.Deck{Build: &build.Deck{Main: append(append([]string{}, ids...), cove), Basics: [5]int{8, 6, 0, 3, 0}}}
	if got := d.Build.Total(); got != 40 {
		t.Fatalf("hand-built deck has %d cards, want 40", got)
	}
	m := describe(lab, pool, d)
	if m.Colours != "WU" || m.Splash != "R" || m.Archetype != "WU+R" {
		t.Errorf("colours/splash/archetype = %q/%q/%q, want WU/R/WU+R", m.Colours, m.Splash, m.Archetype)
	}
	if m.ColourCount != 3 || m.SplashCards != 2 || m.Unsourced != 0 {
		t.Errorf("colour_count/splash_cards/unsourced = %d/%d/%d, want 3/2/0", m.ColourCount, m.SplashCards, m.Unsourced)
	}
	if m.Spells != 22 || m.LandsBasic != 17 || m.LandsNonbasic != 1 {
		t.Errorf("spells/basic/nonbasic = %d/%d/%d, want 22/17/1", m.Spells, m.LandsBasic, m.LandsNonbasic)
	}
	if m.Curve != curve {
		t.Errorf("curve = %v, want %v", m.Curve, curve)
	}
	if m.Types != types {
		t.Errorf("types = %+v, want %+v", m.Types, types)
	}

	noCove := draftlab.Deck{Build: &build.Deck{Main: append([]string{}, ids...), Basics: [5]int{9, 9, 0, 0, 0}}}
	if got := noCove.Build.Total(); got != 40 {
		t.Fatalf("hand-built deck without Cove has %d cards, want 40", got)
	}
	if got := describe(lab, pool, noCove).Unsourced; got != 2 {
		t.Errorf("unsourced without red sources = %d, want 2", got)
	}
}

func TestSummaryFormat(t *testing.T) {
	metas := []meta{
		{
			ColourCount: 1, LandsBasic: 17,
			Curve: [6]int{1, 2, 3, 4, 5, 6},
			Types: metaTypes{Creature: 10, Instant: 0, Sorcery: 1, Enchantment: 2, Artifact: 0},
		},
		{
			ColourCount: 2, Splash: "R", Unsourced: 1, LandsBasic: 16, LandsNonbasic: 1,
			Curve: [6]int{0, 1, 2, 3, 4, 5},
			Types: metaTypes{Creature: 20, Instant: 1, Sorcery: 2, Enchantment: 0, Artifact: 1},
		},
		{
			ColourCount: 4, Splash: "G", LandsBasic: 15, LandsNonbasic: 3,
			Curve: [6]int{2, 4, 6, 8, 10, 12},
			Types: metaTypes{Creature: 30, Instant: 2, Sorcery: 3, Enchantment: 1, Artifact: 2},
		},
	}
	want := "" +
		"creatures min 10 mean 20.00 max 30\n" +
		"curve mean <=1 1.00 2 2.33 3 3.67 4 5.00 5 6.33 6+ 7.67\n" +
		"instant 0:1 1:1 2+:1\n" +
		"sorcery 0:0 1:1 2+:2\n" +
		"enchantment 0:1 1:1 2+:1\n" +
		"artifact 0:1 1:1 2+:1\n" +
		"colours 1:1 2:1 3:0 4+:1\n" +
		"splash 2 lands17 2 lands18 1 unsourced 1\n"
	if got := summarise(metas); got != want {
		t.Errorf("summarise:\n%s\nwant:\n%s", got, want)
	}
}

func TestDeckpoolTRKShape(t *testing.T) {
	corpus := testutil.CorpusDirOrSkip(t)
	base := t.TempDir()
	runPool := func(name, drafter, builder string) ([]indexEntry, string, string) {
		out := filepath.Join(base, name)
		var stdout bytes.Buffer
		o := options{set: "TRK", pods: 25, seed: 1000, drafter: drafter, builder: builder, corpus: corpus, out: out}
		if err := run(o, &stdout); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return readIndex(t, out), stdout.String(), out
	}
	a, summaryA, outA := runPool("a", "balanced", "balanced")
	b, summaryB, _ := runPool("b", "rating-colour", "best23")
	t.Logf("A (balanced/balanced):\n%s", summaryA)
	t.Logf("B (rating-colour/best23):\n%s", summaryB)
	for _, e := range a {
		raw, err := os.ReadFile(filepath.Join(outA, e.File))
		if err != nil {
			t.Fatal(err)
		}
		f, err := deck.Parse(raw)
		if err != nil {
			t.Fatalf("%s: %v", e.File, err)
		}
		if f.Archetype != e.Archetype {
			t.Errorf("%s deck file archetype %q, index says %q", e.ID, f.Archetype, e.Archetype)
		}
	}

	n := func(idx []indexEntry, pred func(indexEntry) bool) int {
		count := 0
		for _, e := range idx {
			if pred(e) {
				count++
			}
		}
		return count
	}
	if got := n(a, func(e indexEntry) bool { return e.ColourCount >= 4 }); got > 5 {
		t.Errorf("A colour_count >= 4: %d decks, want <= 5", got)
	}
	if got := n(b, func(e indexEntry) bool { return e.ColourCount >= 4 }); got < 30 {
		t.Errorf("B colour_count >= 4: %d decks, want >= 30", got)
	}
	for _, tc := range []struct {
		name string
		get  func(indexEntry) int
	}{
		{"instant", func(e indexEntry) int { return e.Types.Instant }},
		{"enchantment", func(e indexEntry) int { return e.Types.Enchantment }},
		{"artifact", func(e indexEntry) int { return e.Types.Artifact }},
	} {
		na := n(a, func(e indexEntry) bool { return tc.get(e) <= 1 })
		nb := n(b, func(e indexEntry) bool { return tc.get(e) <= 1 })
		if na >= nb {
			t.Errorf("%s <= 1: A %d, B %d, want A < B", tc.name, na, nb)
		}
	}
	if got := n(a, func(e indexEntry) bool { return e.Unsourced > 0 }); got > 5 {
		t.Errorf("A unsourced > 0: %d decks, want <= 5", got)
	}
	for _, e := range a {
		if e.Spells != 22 && e.Spells != 23 {
			t.Errorf("A deck %s has %d spells, want 22 or 23", e.ID, e.Spells)
		}
		if got := e.Spells + e.LandsBasic + e.LandsNonbasic; got != 40 {
			t.Errorf("A deck %s has %d cards, want 40", e.ID, got)
		}
	}
}
