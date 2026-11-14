package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The dr-mbc-aaab evidence sweep's pair 105 (seed 3460086283630579194),
// replayed on a table built from its own decks (pool "custom"). You Find Some
// Prisoners exiled the opponent's Vanish into Eternity and let p0 cast it,
// spending mana as though it were any colour. p0 has a Mountain, an Island
// and a Mountain untapped and controls Kyren Negotiations, a noncreature
// target Vanish's own ValidTarget$ RaiseCost leaves at {2}{W} (a creature
// costs {5}{W}). gorge offers the cast under the rider (mayPlayAffordable)
// and keeps Kyren in the target menu; the kernel withheld the offer (the
// target-priced may-play arm priced {W} without the rider) and, offered, had
// emptied the menu (dropTargetsOwing likewise) and reversed the cast.

// TestMayPlayAnyColourTargetRaisedCast replays the game through the cast of
// Vanish into Eternity and its target choice.
func TestMayPlayAnyColourTargetRaisedCast(t *testing.T) {
	t.Parallel()
	const name = "mayplay_any_colour_target_raised_cast"
	requireAnyColourVanishTargeted(t, name)
	stdReproNamed(t, name)
}

// requireAnyColourVanishTargeted replays the recording gorge-only and requires
// the game the lockstep compares to be the one this file names: Vanish into
// Eternity, owned by p1, chooses its target as p0's spell. A replay that never
// reached the cast would pass the lockstep vacuously.
func requireAnyColourVanishTargeted(t *testing.T, name string) {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(synthReg(t))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	rc.MaxDecisions = len(r.Actions) + 1
	var evs []events.Event
	var obj func(id state.ObjID) *state.Object
	rc.Finish = func(g *GorgeSide) {
		evs = append(evs, g.E.L.Events...)
		obj = g.E.G.Obj
	}
	k, err := GorgeKernel(rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	for _, e := range evs {
		if e.Kind != events.TargetsChosen {
			continue
		}
		o := obj(e.Obj)
		if o == nil || o.Card == nil || len(o.Card.Faces) == 0 || o.Card.Faces[0].Name != "Vanish into Eternity" {
			continue
		}
		if o.Owner != 1 || o.Controller != 0 {
			t.Fatalf("precondition: Vanish into Eternity owned by %d and controlled by %d, want p1's card cast by p0", o.Owner, o.Controller)
		}
		return
	}
	t.Fatal("precondition: Vanish into Eternity never chose its targets")
}
