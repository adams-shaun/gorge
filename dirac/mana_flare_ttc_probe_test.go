package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Probe of the dr-mbc-abbb focus-abs2 pair 1042 divergence (decision 171):
// Three Tree City's {T}: Add {C} beside its zero-yield {2},{T} dynamic
// any-colour arm, one Island, an opponent's Mana Flare on the battlefield,
// Absorb Vis ({1}{B} basic landcycling) in hand, pool empty. The tap's
// produced types are {C} and {U}, so no black mana exists and the cycle must
// not be offered. The kernel offered it in the lockstep game.
func TestMBCAbsorbVisFlareOfferProbe(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Three Tree City", Count: 1}, {Name: "Mana Flare", Count: 1},
		{Name: "Island", Count: 20}, {Name: "Absorb Vis", Count: 2},
		{Name: "Swamp", Count: 17},
	}
	set, err := cardset.SynthByName("dr-mbc-abbb")
	if err != nil {
		t.Skipf("synth pool unavailable: %v", err)
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true, GorgeAutopaySurface: true})
	sc.perm(id["Three Tree City"], 0)
	if d := sc.s.Step(); d.Kind == DkChoose { // TTC's as-enters type choice: answered, then set
		sc.s.Apply(d.Options[0])
	}
	sc.perm(id["Island"], 0)
	sc.perm(id["Mana Flare"], 1)
	abs := sc.hand(id["Absorb Vis"], 0)
	s := sc.s
	var d Decision
	var av avail
	s.priorityGen(sc.k, &d, 0, true, &av, false)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && Obj(a.Source()) == abs {
			t.Fatalf("kernel offered the {1}{B} cycle with no black mana on the board; options %v, avail %v", d.Options[:d.N], av.mask[:av.n])
		}
	}
}
