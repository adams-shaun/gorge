package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestExiledByYouPredicate pins Card.ExiledByYou: it reads the recorded exiler
// (Object.ExiledBy/HasExiledBy), seat 0 is a real answer, and an object with no
// recorded exiler never matches. Before the registry entry the spec was an
// unknown predicate and matched nothing in either direction.
func TestExiledByYouPredicate(t *testing.T) {
	g, _ := board(t)
	c, d := cards.ParseBytes("t.txt", []byte("Name:Exiled\nManaCost:1\nTypes:Artifact\nOracle:x\n"))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	o := g.AddObject(c, 1)
	o.Zone = state.ZExile
	g.SetZone(state.ZExile, 1, append(g.Zone(state.ZExile, 1), o.ID))
	if in := g.Zone(state.ZExile, 1); len(in) != 1 || in[0] != o.ID || o.Zone != state.ZExile {
		t.Fatalf("precondition: object not in exile: %v", in)
	}

	if MatchesSpec(g, "Card.ExiledByYou", o.ID, 0) {
		t.Errorf("no recorded exiler must not match ExiledByYou")
	}
	if !MatchesSpec(g, "Card.!ExiledByYou", o.ID, 0) {
		t.Errorf("no recorded exiler must match !ExiledByYou")
	}

	o.HasExiledBy, o.ExiledBy = true, 0
	if !MatchesSpec(g, "Card.ExiledByYou", o.ID, 0) {
		t.Errorf("seat 0 recorded as exiler must match for you=0")
	}
	if MatchesSpec(g, "Card.ExiledByYou", o.ID, 1) {
		t.Errorf("seat 0 recorded as exiler must not match for you=1")
	}
	if !MatchesSpec(g, "Card.!ExiledByYou", o.ID, 1) {
		t.Errorf("!ExiledByYou must hold for you=1")
	}

	for _, spec := range []string{"Card.ExiledByYou", "Card.!ExiledByYou", "Card.nonCreature+ExiledByYou+counters_GE1_FETCH"} {
		if u := UnknownPredicates(spec); len(u) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want none", spec, u)
		}
	}
}
