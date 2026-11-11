package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestTargetedOtherBase pins Forge's Targeted.<props> base (AbilityUtils
// filterListByType: the base becomes Card and the source becomes the first
// targeted card), so Targeted.Other+YouCtrl+Creature is "each creature you
// control other than the target" (CR 608.2c), and an unbound target matches
// nothing.
func TestTargetedOtherBase(t *testing.T) {
	g, id := namedBoard(t)
	spec := "Targeted.Other+YouCtrl+Creature"
	sc := SpecContext{You: 0, Source: id["under"], ResolutionTargets: []state.Target{{Obj: id["bear"]}}, Resolving: true}
	cases := []struct {
		obj  string
		want bool
	}{
		{"bear", false},   // the target itself
		{"hawk2", true},   // another creature you control
		{"bfhawk", true},  // another creature you control
		{"giant", false},  // an opponent's creature
		{"bfleft", false}, // not a creature
	}
	for _, c := range cases {
		if got := MatchesObjectCtx(g, spec, g.Obj(id[c.obj]), sc); got != c.want {
			t.Errorf("%s on %s = %v, want %v", spec, c.obj, got, c.want)
		}
		if got := MatchesSpecCtx(g, spec, id[c.obj], sc); got != c.want {
			t.Errorf("MatchesSpecCtx %s on %s = %v, want %v", spec, c.obj, got, c.want)
		}
	}
	unbound := SpecContext{You: 0, Source: id["under"]}
	if MatchesObjectCtx(g, spec, g.Obj(id["hawk2"]), unbound) {
		t.Error("an unbound Targeted base matched")
	}
}
