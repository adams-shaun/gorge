package cards

import "testing"

// face builds a minimal named face for the breakdown tests and derives the
// values the groupers read (Cmc in particular), the same way every real
// construction path does.
func breakdownFace(name, cost, colors string, types ...string) *Face {
	f := newFace()
	f.Name = name
	f.ManaCost = cost
	f.Colors = colors
	f.Types = types
	f.derive()
	return f
}

func breakdownCard(api string, faces ...*Face) *Card {
	c := &Card{Faces: faces}
	if api != "" {
		faces[0].Abilities = []*SA{{API: api}}
	}
	return c
}

func TestPrimaryTypeGroup(t *testing.T) {
	cases := []struct {
		types []string
		want  string
	}{
		{[]string{"Artifact", "Creature"}, "Creature"},
		{[]string{"Land", "Creature"}, "Creature"},
		{[]string{"Artifact", "Land"}, "Land"},
		{[]string{"Legendary", "Planeswalker"}, "Planeswalker"},
		{[]string{"Battle", "Siege"}, "Battle"},
		{[]string{"Instant"}, "Instant"},
		{[]string{"Sorcery"}, "Sorcery"},
		{[]string{"Enchantment"}, "Enchantment"},
		{[]string{"Artifact"}, "Artifact"},
		{[]string{"Scheme"}, "Other"},
		{nil, "Other"},
	}
	for _, tc := range cases {
		c := breakdownCard("", breakdownFace("x", "", "", tc.types...))
		if got := PrimaryTypeGroup(c); got != tc.want {
			t.Errorf("PrimaryTypeGroup(%v) = %q, want %q", tc.types, got, tc.want)
		}
	}
	if got := PrimaryTypeGroup(nil); got != "Other" {
		t.Errorf("PrimaryTypeGroup(nil) = %q, want Other", got)
	}
}

func TestColourGroup(t *testing.T) {
	cases := []struct {
		cost, colors, want string
	}{
		{"W", "", "White"},
		{"2 U", "", "Blue"},
		{"B B", "", "Black"},
		{"1 R", "", "Red"},
		{"G", "", "Green"},
		{"1 W U", "", "Multicolour"},
		{"3", "", "Colorless"},
		{"", "", "Colorless"},
		// Dryad Arbor: no mana cost, colour indicator only.
		{"", "green", "Green"},
	}
	for _, tc := range cases {
		c := breakdownCard("", breakdownFace("x", tc.cost, tc.colors, "Creature"))
		if got := ColourGroup(c); got != tc.want {
			t.Errorf("ColourGroup(cost=%q colors=%q) = %q, want %q", tc.cost, tc.colors, got, tc.want)
		}
	}
	// A colourless card whose ABILITY costs red is red-identity but is not a
	// red card; the breakdown must report the printed colour, not the identity.
	f := breakdownFace("x", "2", "", "Artifact")
	f.Oracle = "{R}, {T}: It deals 1 damage to any target."
	f.derive()
	c := breakdownCard("", f)
	if c.ColourIdentity() == 0 {
		t.Fatal("test setup: expected a non-empty colour identity")
	}
	if got := ColourGroup(c); got != "Colorless" {
		t.Errorf("ColourGroup of a red-identity colourless artifact = %q, want Colorless", got)
	}
}

func TestManaValueGroup(t *testing.T) {
	cases := []struct{ cost, want string }{
		{"", "0"},
		{"1", "1"},
		{"2 W", "3"},
		{"6", "6"},
		{"7", "7+"},
		{"12 W W", "7+"},
	}
	for _, tc := range cases {
		c := breakdownCard("", breakdownFace("x", tc.cost, "", "Creature"))
		if got := ManaValueGroup(c); got != tc.want {
			t.Errorf("ManaValueGroup(%q) = %q, want %q", tc.cost, got, tc.want)
		}
	}
}

// TestCoverageByAgreesWithCoverage is the invariant that makes the published
// table trustworthy: every row's counts come from the same verdict Coverage
// applies, and the rows sum to the headline numbers. An unnamed card is
// excluded from both.
func TestCoverageByAgreesWithCoverage(t *testing.T) {
	r := NewRegistry()
	r.Add(breakdownCard("Draw", breakdownFace("Good Creature", "1 G", "", "Creature")))
	r.Add(breakdownCard("Unknown", breakdownFace("Bad Creature", "1 G", "", "Creature")))
	r.Add(breakdownCard("Draw", breakdownFace("Good Instant", "U", "", "Instant")))
	r.Add(breakdownCard("", breakdownFace("", "", "", "Creature"))) // unnamed: excluded

	sup := map[string]bool{"api:Draw": true, "api:": true}
	cv := r.Coverage(sup)
	if cv.Cards != 3 || cv.Supported != 2 {
		t.Fatalf("Coverage = %d/%d, want 2/3", cv.Supported, cv.Cards)
	}

	for _, key := range []GroupKey{PrimaryTypeGroup, ColourGroup, ManaValueGroup} {
		groups := r.CoverageBy(sup, key)
		var cards, supported int
		for i, g := range groups {
			cards += g.Cards
			supported += g.Supported
			if i > 0 && groups[i-1].Key >= g.Key {
				t.Errorf("rows are not key-sorted: %q then %q", groups[i-1].Key, g.Key)
			}
		}
		if cards != cv.Cards || supported != cv.Supported {
			t.Errorf("rows sum to %d/%d, want %d/%d", supported, cards, cv.Supported, cv.Cards)
		}
	}

	byType := r.CoverageBy(sup, PrimaryTypeGroup)
	want := map[string]CoverageGroup{
		"Creature": {"Creature", 2, 1},
		"Instant":  {"Instant", 1, 1},
	}
	if len(byType) != len(want) {
		t.Fatalf("byType = %+v", byType)
	}
	for _, g := range byType {
		if w, ok := want[g.Key]; !ok || g != w {
			t.Errorf("byType row %+v, want %+v", g, w)
		}
	}
	if got := (CoverageGroup{"x", 4, 1}).Percent(); got != 25 {
		t.Errorf("Percent = %v, want 25", got)
	}
	if got := (CoverageGroup{Key: "empty"}).Percent(); got != 0 {
		t.Errorf("Percent of an empty group = %v, want 0", got)
	}
}
