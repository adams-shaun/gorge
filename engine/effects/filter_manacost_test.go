package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestManaCostWord pins the ManaCost<N> card property (Urza's Saga Chapter
// III, "Artifact.ManaCost0,Artifact.ManaCost1"): an artifact whose printed
// mana cost has mana value N. A land that prints no mana cost has mana value
// 0 but no mana cost {0}, so it never matches ManaCost0.
func TestManaCostWord(t *testing.T) {
	g := state.NewGame([]string{"you", "them"})
	mk := func(src string) state.ObjID {
		c, d := cards.ParseBytes("t.txt", []byte(src))
		if len(d) != 0 {
			t.Fatalf("parse: %v", d)
		}
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		o := g.AddObject(c, 0)
		o.Zone = state.ZLibrary
		return o.ID
	}
	zero := mk("Name:Zero\nManaCost:0\nTypes:Artifact Creature Thopter\nPT:0/2\nOracle:x\n")
	one := mk("Name:One\nManaCost:1\nTypes:Artifact\nOracle:x\n")
	two := mk("Name:Two\nManaCost:1 U\nTypes:Artifact\nOracle:x\n")
	noCost := mk("Name:Citadel\nManaCost:no cost\nTypes:Artifact Land\nOracle:x\n")
	if got := UnknownPredicates("Artifact.ManaCost0,Artifact.ManaCost1"); len(got) != 0 {
		t.Fatalf("ManaCost0/1 still unknown: %v", got)
	}
	if got := UnknownPredicates("Artifact.ManaCost=Equipped"); len(got) != 1 {
		t.Fatalf("ManaCost=Equipped must stay unknown, got %v", got)
	}
	const spec = "Artifact.ManaCost0,Artifact.ManaCost1"
	for _, tc := range []struct {
		name string
		id   state.ObjID
		want bool
	}{
		{"{0} artifact", zero, true},
		{"{1} artifact", one, true},
		{"{1}{U} artifact", two, false},
		{"artifact land with no mana cost", noCost, false},
	} {
		if got := MatchesSpecCtx(g, spec, tc.id, NewSpecContext(0, tc.id)); got != tc.want {
			t.Errorf("%s: %s = %v, want %v", tc.name, spec, got, tc.want)
		}
	}
	if MatchesSpecCtx(g, "Artifact.ManaCost0", noCost, NewSpecContext(0, noCost)) {
		t.Error("a no-cost artifact land matched ManaCost0")
	}
	g.Obj(zero).FaceDown = true
	if MatchesSpecCtx(g, "Artifact.ManaCost0", zero, NewSpecContext(0, zero)) {
		t.Error("a face-down object matched ManaCost0")
	}
}
