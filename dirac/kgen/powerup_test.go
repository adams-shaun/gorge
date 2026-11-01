package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestPowerUpXShapes: Stature's {X}{U}{U} power-up lowers (a single {X}: the
// reduction leaves X alone); {X}{X} and {X} beside a hybrid
// symbol are refused with a named reason.
func TestPowerUpXShapes(t *testing.T) {
	found := false
	for _, c := range lowerNamed(t, []string{"Stature, Size Shifter"}) {
		if c.Name != "Stature, Size Shifter" {
			continue
		}
		found = true
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "power-up") {
				t.Errorf("Stature refused: %s", r.Reason)
			}
		}
		n := 0
		for _, ab := range c.Abilities {
			if ab.PowerUp {
				n++
				if ab.Limit != 1 || ab.Cost.Mana.String() != "{0UUX}" {
					t.Errorf("Stature power-up: limit %d cost %s, want 1 {0UUX}", ab.Limit, ab.Cost.Mana)
				}
			}
		}
		if n != 1 {
			t.Errorf("Stature has %d power-up abilities, want 1", n)
		}
	}
	if !found {
		t.Fatal("Stature, Size Shifter not lowered")
	}
	for _, c := range []struct{ cost, want string }{
		{"X X U", "power-up with {X}{X} cost"},
		{"X RG", "power-up with {X} and a hybrid symbol"},
	} {
		sa := &cards.SA{Kind: "AB", API: "PutCounter", Params: map[string]string{
			"Cost": c.cost, "Defined": "Self", "CounterType": "P1P1", "CounterNum": "X", "PowerUp": "True"}}
		face := &cards.Face{Name: "Synthetic", ManaCost: "U", Types: []string{"Creature"}, Abilities: []*cards.SA{sa}}
		card := &cards.Card{Faces: []*cards.Face{face}}
		p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
		var got []string
		for _, r := range lowerCard(p, "Synthetic", card, false).Refusals {
			got = append(got, r.Reason)
		}
		if all := strings.Join(got, " | "); !strings.Contains(all, c.want) {
			t.Errorf("cost %q: refusals %q, want one containing %q", c.cost, all, c.want)
		}
	}
}

// TestFightTriggeredObject: a Fight whose Defined$ is the triggering card
// names the trigger object, not the source, unless the trigger is a
// self-trigger. Tolsimir, Friend to Wolves fights with the entering Wolf.
func TestFightTriggeredObject(t *testing.T) {
	want := map[string]refKind{"Tolsimir, Friend to Wolves": rTriggered, "Affectionate Indrik": rSelf}
	seen := 0
	for _, c := range lowerNamed(t, []string{"Tolsimir, Friend to Wolves", "Affectionate Indrik"}) {
		w, ok := want[c.Name]
		if !ok {
			continue
		}
		for _, ab := range c.Abilities {
			for _, in := range ab.Prog {
				if in.Op == "Fight" {
					seen++
					if in.Obj.Kind != w {
						t.Errorf("%s: Fight obj kind %d, want %d", c.Name, in.Obj.Kind, w)
					}
				}
			}
		}
	}
	if seen != 2 {
		t.Fatalf("saw %d Fight instrs, want 2", seen)
	}
}
