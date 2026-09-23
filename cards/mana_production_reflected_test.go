package cards

import "testing"

// TestManaProductionSeesReflectedAbilities is the collector half of the
// api:ManaReflected ticket: a reflected-mana ability (AB$ ManaReflected) is a
// mana ability, so the per-face ManaProduction summary must account for it.
// Its colours are computed at resolution, so the honest summary is the
// conditional "Any" shape (mirroring a "Produced$ Any" source) rather than
// silence. Before the fix Face.ManaAbilities listed only AB$ Mana, so a
// reflected source projected as producing nothing and a bot's tap gate never
// aimed a coloured pip at it.
func TestManaProductionSeesReflectedAbilities(t *testing.T) {
	f := &Face{Abilities: []*SA{
		{Kind: "AB", API: "Mana", Params: map[string]string{"Produced": "W"}},
		{Kind: "AB", API: "ManaReflected", Params: map[string]string{"Valid": "Land.OppCtrl", "ReflectProperty": "Produce"}},
	}}
	f.derive()
	p := f.ManaProduction()
	if !p.Any {
		t.Fatal("a reflected ability is invisible to the collector: Any was not set")
	}
	if !p.Reflected {
		t.Fatal("the collector did not flag the reflected ability's source (Reflected unset)")
	}
	if p.IsZero() {
		t.Fatal("the face projects as producing nothing despite a reflected ability")
	}
	if !p.Colourless() {
		t.Fatal("the reflected ability's guaranteed amount should fold as one colourless")
	}

	// Precondition: Reflected tracks the reflected ability, not the mere
	// presence of a mana ability or of a plain Any source. A plain
	// "Produced$ W" face is neither Any nor Reflected, and a
	// Cavern-of-Souls-shaped "Produced$ Any" face is Any but NOT Reflected
	// (the two facts must not collapse, or the tap gate would widen).
	g := &Face{Abilities: []*SA{
		{Kind: "AB", API: "Mana", Params: map[string]string{"Produced": "W"}},
	}}
	g.derive()
	if g.ManaProduction().Any || g.ManaProduction().Reflected {
		t.Fatal("a plain Produced$ W face set Any/Reflected -- the flags do not track reflection")
	}
	if !g.ManaProduction().ProducesColour(0) {
		t.Fatal("precondition failed: the plain W face does not produce white")
	}
	h := &Face{Abilities: []*SA{
		{Kind: "AB", API: "Mana", Params: map[string]string{"Produced": "Any"}},
	}}
	h.derive()
	if !h.ManaProduction().Any {
		t.Fatal("precondition failed: a Produced$ Any face must set Any")
	}
	if h.ManaProduction().Reflected {
		t.Fatal("a plain Produced$ Any face set Reflected -- only AB$ ManaReflected may")
	}
}
