package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestTapPowerStation: Tapestry Warden and Sulu lower their Mode$
// TapPowerValue lines to StatTapPower statics: Warden's your-creatures
// power<toughness filter with Value$ Toughness on Station, and Sulu's two
// alternatives (Station+Spacecraft and Crew+Vehicle) with Value$ X =
// Count$CardPower/Twice read over the tapped creature. A Spell ValidSA still
// refuses, as it has no host permanent the qualifier could name.
func TestTapPowerStation(t *testing.T) {
	found := map[string][]lStatic{}
	for _, c := range lowerNamed(t, []string{"Tapestry Warden", "Sulu, Heroic Helmsman"}) {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
		for _, st := range c.Statics {
			if st.Kind == "TapPower" {
				found[c.Name] = append(found[c.Name], st)
			}
		}
	}

	warden := found["Tapestry Warden"]
	if len(warden) != 1 {
		t.Fatalf("Tapestry Warden: %d TapPower statics, want 1", len(warden))
	}
	w := warden[0]
	if w.F == nil {
		t.Fatal("Tapestry Warden: no ValidCard filter")
	}
	if !hasFlag(w.Flags, "Station") || !hasFlag(w.Flags, "Toughness") {
		t.Fatalf("Tapestry Warden flags %v, want Station and Toughness", w.Flags)
	}
	if w.F2 != nil {
		t.Fatalf("Tapestry Warden host filter %v, want any", w.F2)
	}
	if w.Amount.Kind != vNone {
		t.Fatalf("Tapestry Warden amount %v, want the Toughness sentinel", w.Amount)
	}

	sulu := found["Sulu, Heroic Helmsman"]
	if len(sulu) != 2 {
		t.Fatalf("Sulu: %d TapPower statics, want 2", len(sulu))
	}
	byScope := map[string]lStatic{}
	for _, st := range sulu {
		for _, scope := range []string{"Station", "Crew"} {
			if hasFlag(st.Flags, scope) {
				byScope[scope] = st
			}
		}
		if st.Amount.Kind != vPowerOf || st.Amount.Ref.Kind != rSelf || st.Amount.Mul != 2 {
			t.Fatalf("Sulu amount %v, want the tapped creature's power times two", st.Amount)
		}
	}
	if st, ok := byScope["Station"]; !ok || st.F2 == nil {
		t.Fatalf("Sulu Station static %+v, want a Spacecraft host filter", st)
	}
	if st, ok := byScope["Crew"]; !ok || st.F2 == nil {
		t.Fatalf("Sulu Crew static %+v, want a Vehicle host filter", st)
	}

	// A ValidSA naming a Spell has no permanent host: refuse rather than
	// silently widening the static to every action.
	p := &pool{byName: map[string]*cards.Card{}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lc := &lowerCtx{face: &cards.Face{Name: "Synthetic"}, out: &lCard{}, p: p, known: p.knownSubtype}
	err := lc.tapPowerValueStatic(cards.Static{Mode: "TapPowerValue", Params: map[string]string{
		"Mode": "TapPowerValue", "ValidCard": "Card.Self", "ValidSA": "Activated.Spell", "Value": "2",
	}})
	if err == nil || !strings.Contains(err.Error(), "ValidSA") {
		t.Fatalf("Spell ValidSA: error %v, want a ValidSA refusal", err)
	}
}

func hasFlag(flags []string, want string) bool {
	for _, f := range flags {
		if f == want {
			return true
		}
	}
	return false
}
