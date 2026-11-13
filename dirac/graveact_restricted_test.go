package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// graveActScen builds a scenario over the named cards of the std-fra pool.
func graveActScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	var deck []cardset.Entry
	set := cardsettest.Synth(t, "std-fra")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-fra table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTable(t, tbl, Config{NoElide: true}), id
}

// stdMshScen builds a scenario over the named cards of the std-msh pool; its
// libraries hold a Forest of that table.
func stdMshScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	var deck []cardset.Entry
	set := cardsettest.Synth(t, "std-msh")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not in the std-msh table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true}), id
}

// graveActivations counts the activations of o offered at priority.
func (sc *scen) graveActivations(o Obj) (n int) {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == uint8(o) {
			n++
		}
	}
	return n
}

// A mana ability that only funds payments other than a spell cast from hand
// (Heartwood Crafter's {C}) still pays an activated ability worked from the
// graveyard: four Forests and the Crafter fund Puppet Crafting's {4}{G}
// return, as they fund an ability worked from the hand (CR 106.6, 602.2b).
func TestGraveyardActivationCountsARestrictedSource(t *testing.T) {
	sc, id := graveActScen(t, "Puppet Crafting", "Heartwood Crafter", "Forest")
	for i := 0; i < 4; i++ {
		sc.perm(id["Forest"], 0)
	}
	sc.perm(id["Heartwood Crafter"], 0)
	craft := sc.obj(id["Puppet Crafting"], 0)
	sc.s.place(craft, kop.ZoneGraveyard, false)
	sc.s.ctl = ctlPriority
	if n := sc.graveActivations(craft); n != 1 {
		t.Fatalf("Puppet Crafting's graveyard return offered %d times with five mana, want 1", n)
	}
}

// The restricted source is not extra mana: without it four Forests fall short.
func TestGraveyardActivationNeedsTheFifthMana(t *testing.T) {
	sc, id := graveActScen(t, "Puppet Crafting", "Heartwood Crafter", "Forest")
	for i := 0; i < 4; i++ {
		sc.perm(id["Forest"], 0)
	}
	craft := sc.obj(id["Puppet Crafting"], 0)
	sc.s.place(craft, kop.ZoneGraveyard, false)
	sc.s.ctl = ctlPriority
	if n := sc.graveActivations(craft); n != 0 {
		t.Fatalf("Puppet Crafting's return offered %d times with four mana, want 0", n)
	}
}
