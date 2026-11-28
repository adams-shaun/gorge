package ir

import (
	"reflect"
	"testing"
)

// TestCustomAbility covers an ability the typed interface does not describe:
// it round-trips, Equal compares Card and Scope but not Src, and it prints in
// place of a body.
func TestCustomAbility(t *testing.T) {
	ab := func(card, scope, src string) *Ability {
		return &Ability{Kind: AbActivated, Custom: &Custom{Card: card, Scope: scope, Src: src}}
	}
	c := &Card{Faces: []*Face{{Abilities: []*Ability{ab("Alpha", "a0", "src")}}}}
	back := mustDecode(t, EncodeCard(c))
	if !reflect.DeepEqual(c, back) || !Equal(c, back) {
		t.Errorf("custom ability does not round trip\nbefore %s\nafter  %s", c, back)
	}
	if back.Faces[0].Abilities[0].Custom == nil {
		t.Fatal("decoded ability lost Custom")
	}
	if !Equal(ab("Alpha", "a0", "x"), ab("Alpha", "a0", "y")) {
		t.Error("Custom.Src must be ignored")
	}
	if !Equal(&Custom{Card: "Alpha", Scope: "a0", Src: "x"}, &Custom{Card: "Alpha", Scope: "a0"}) {
		t.Error("Equal(*Custom) must ignore Src")
	}
	for _, other := range []*Ability{ab("Beta", "a0", ""), ab("Alpha", "a1", ""), {Kind: AbActivated}} {
		if Equal(ab("Alpha", "a0", ""), other) {
			t.Errorf("%s must differ from %s", ab("Alpha", "a0", ""), other)
		}
	}
	if got, want := ab("Alpha", "t1", "src").String(), `(ability activated :custom (custom "Alpha" t1))`; got != want {
		t.Errorf("print = %s, want %s", got, want)
	}
}
