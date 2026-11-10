package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// An inline stand-in for "return target creature card, then it explores
// twice" (the real script is Forge's, GPL, and never committed); the hole is
// what the variants change.
const exploreRememberedProbe = `Name:Explore Probe
ManaCost:4 B
Types:Sorcery
A:SP$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | ValidTgts$ Creature.YouOwn PARAMS| SubAbility$ DBExplore
SVar:DBExplore:DB$ Explore | Num$ 2 | Defined$ Remembered | SubAbility$ DBEnd
SVar:DBEnd:DB$ Cleanup | ClearRemembered$ True
Oracle:probe
`

// TestExploreRemembered: Explore Defined$ Remembered lowers to OpExplore of
// RefRemembered behind one single-object Return writer (the returned card
// explores); without the writer, or over a multi-object return, it refuses by
// name. Defossilize lowers clean with two explores of the remembered card.
func TestExploreRemembered(t *testing.T) {
	lc := lowerProbe(t, exploreRememberedProbe, "Explore Probe", "| RememberChanged$ True ")
	if r := probeRefusals(lc); r != "" {
		t.Fatalf("Explore of Remembered refused: %s", r)
	}
	if ops := strings.Join(progOps(lc), " "); ops != "Return Explore Explore ClearRemembered" {
		t.Errorf("lowered to %q, want the remembering Return then two explores", ops)
	}
	for _, in := range lc.Abilities[0].Prog {
		if in.Op == "Explore" && in.Obj.Kind != rRemembered {
			t.Errorf("Explore of %s, want the remembered set", in.Obj)
		}
	}
	for _, tc := range []struct{ params, want string }{
		{"", "Explore of Remembered without one single-object Return writer"},
		{"| RememberChanged$ True | TargetMin$ 0 | TargetMax$ 2 ", "Explore of Remembered over a multi-object Return"},
	} {
		if got := probeRefusals(lowerProbe(t, exploreRememberedProbe, "Explore Probe", tc.params)); !strings.Contains(got, tc.want) {
			t.Errorf("%q: refusals %q, want %q", tc.params, got, tc.want)
		}
	}

	// the corpus card
	const name = "Defossilize"
	lw := lowerNamedPool(t, []string{name})
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range all {
		if c.Token || c.Name != name {
			continue
		}
		if r := lowerRefusals(c); len(r) > 0 {
			t.Fatalf("%s refused: %v", name, r)
		}
		n := 0
		for _, a := range bs[i].def.Abilities {
			for _, in := range a.Prog {
				if in.Op == kop.OpExplore && in.Obj == kop.RefRemembered {
					n++
				}
			}
		}
		if n != 2 {
			t.Errorf("%s: %d explores of the remembered card, want 2", name, n)
		}
		return
	}
	t.Fatalf("%s: not lowered", name)
}
