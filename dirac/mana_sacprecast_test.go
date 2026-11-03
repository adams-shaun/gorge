package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// precastScen is a scenario over the sacprecast pool with board on p0's side and the
// lowering configuration, a Treasure token among them.
func precastScen(t *testing.T, board ...string) (*scen, map[string]CardID, map[string]Obj) {
	t.Helper()
	set := cardsettest.Set(t, "sacprecast")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeLowering: true})
	objs := map[string]Obj{}
	for _, n := range board {
		o := sc.perm(id[n], 0)
		if n == "c_a_treasure_sac" {
			sc.s.objs[o].of |= ofToken
		}
		objs[n] = o
	}
	return sc, id, objs
}

// A kicked Goblin Bushwhacker paid with a Mountain and the Treasure: the lowering taps the witness at
// priority, so the sacrifice trigger of a permanent that asks nothing (Giant Fly's +1/+0,
// Zhao's pump of the team) resolves before the cast, whatever its program is, and no
// trigger is left to go on the stack above the Goblin.
func TestWitnessSacrificeTriggerThatAsksNothingResolvesBeforeTheCast(t *testing.T) {
	for _, watcher := range []string{"Giant Fly", "Zhao, Ruthless Admiral"} {
		sc, id, objs := precastScen(t, "Mountain", "c_a_treasure_sac", watcher)
		before := sc.s.power(sc.k, sc.slot(objs[watcher]))
		b := sc.hand(id["Goblin Bushwhacker"], 0)
		castFor(t, sc, b)
		if sc.onBF(objs["c_a_treasure_sac"]) {
			t.Fatalf("%s: the Treasure was not sacrificed to pay for the Goblin", watcher)
		}
		if got := sc.s.power(sc.k, sc.slot(objs[watcher])); got != before+1 {
			t.Errorf("%s: power %d after the cast, want %d: its sacrifice trigger did not resolve before the cast", watcher, got, before+1)
		}
		if sc.s.npend != 0 {
			t.Errorf("%s: %d trigger(s) still pending after the cast", watcher, sc.s.npend)
		}
	}
}

// castFor plays the cast of hand object b offered at priority, answering each ask of its
// build with the first option, until the game is back at a priority decision.
func castFor(t *testing.T, sc *scen, b Obj) {
	t.Helper()
	d := sc.s.Step()
	cast := -1
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == b {
			cast = i // the last: the kicked one
		}
	}
	if cast < 0 {
		t.Fatal("the cast is not offered")
	}
	sc.s.Apply(d.Options[cast])
	for i := 0; i < 8; i++ {
		d = sc.s.Step()
		if d.Kind == DkPriority {
			return
		}
		sc.s.Apply(d.Options[0])
	}
	t.Fatal("the cast never returned to priority")
}
