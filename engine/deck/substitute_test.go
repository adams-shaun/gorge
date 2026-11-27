package deck

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// substituteFixture is a tiny registry for SubstituteBasic: a blue-black
// commander and a colourless one. The scripts are authored here.
func substituteFixture(t *testing.T) *cards.Registry {
	t.Helper()
	r := cards.NewRegistry()
	for name, src := range map[string]string{
		"Dimir Spymaster": "Name:Dimir Spymaster\nManaCost:U B\nTypes:Legendary Creature Human Rogue\nOracle:Dimir Spymaster.\n",
		"Iron Golem Lord": "Name:Iron Golem Lord\nManaCost:5\nTypes:Legendary Artifact Creature Golem\nPT:5/5\n",
		"Spy":             "Name:Spy\nManaCost:U\nTypes:Creature Rogue\nOracle:Spy.\n",
	} {
		c, diags := cards.ParseBytes("fixture.txt", []byte(src))
		if len(diags) > 0 {
			t.Fatalf("fixture %s parse: %v", name, diags)
		}
		r.Add(c)
	}
	return r
}

func TestSubstituteBasic(t *testing.T) {
	r := substituteFixture(t)
	if c, ok := r.Lookup("Dimir Spymaster"); !ok || c.ColourIdentity() != cards.ColourBlue|cards.ColourBlack {
		t.Fatalf("fixture: Dimir Spymaster must resolve with identity UB (ok=%v)", ok)
	}
	if c, ok := r.Lookup("Iron Golem Lord"); !ok || c.ColourIdentity() != 0 {
		t.Fatalf("fixture: Iron Golem Lord must resolve colourless (ok=%v)", ok)
	}
	for _, tc := range []struct {
		name string
		f    File
		want string
	}{
		{"most common wins", File{Commander: "Dimir Spymaster", Cards: []Entry{
			{"Dimir Spymaster", 1}, {"Island", 10}, {"Swamp", 30}, {"Spy", 1}}}, "Swamp"},
		{"tie keeps deck order", File{Commander: "Dimir Spymaster", Cards: []Entry{
			{"Dimir Spymaster", 1}, {"Swamp", 20}, {"Island", 20}}}, "Swamp"},
		{"no basic, blue-black commander", File{Commander: "Dimir Spymaster", Cards: []Entry{
			{"Dimir Spymaster", 1}, {"Spy", 99}}}, "Island"},
		{"no basic, colourless commander", File{Commander: "Iron Golem Lord", Cards: []Entry{
			{"Iron Golem Lord", 1}, {"Spy", 99}}}, "Wastes"},
		{"no basic, unresolved commander", File{Commander: "Nobody At All", Cards: []Entry{
			{"Nobody At All", 1}, {"Spy", 99}}}, "Wastes"},
	} {
		if got := SubstituteBasic(tc.f, r); got != tc.want {
			t.Errorf("%s: SubstituteBasic = %q, want %q", tc.name, got, tc.want)
		}
	}
}
