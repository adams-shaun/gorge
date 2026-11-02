package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The chain target union lockstep (union): constructed matchups over the
// generated union pool, whose spells and abilities act on every target of
// a SubAbility$ chain (Defined$ Targeted: gorge's chainTargetUnion). The
// expected answer at every decision is gorge's. Decimate is not in the pool:
// its four slots exceed kop.MaxTargets.

var kunionDecks = [2][]cardset.Entry{
	{
		{Name: "Plague Spores", Count: 4}, {Name: "Spiteful Blow", Count: 3},
		{Name: "Boom", Count: 4}, {Name: "Stomp and Howl", Count: 2}, {Name: "Boom Box", Count: 1},
		{Name: "Darksteel Myr", Count: 2}, {Name: "Ornithopter", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Walking Corpse", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Arm the Cathars", Count: 3}, {Name: "Relive the Past", Count: 2}, {Name: "Glorious Anthem", Count: 2},
		{Name: "Honor of the Pure", Count: 2}, {Name: "Indestructibility", Count: 2}, {Name: "Bottle Gnomes", Count: 2},
		{Name: "Savannah Lions", Count: 3}, {Name: "Giant Spider", Count: 2}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Ornithopter", Count: 2}, {Name: "Darksteel Myr", Count: 1}, {Name: "Stomp and Howl", Count: 2},
		{Name: "Plains", Count: 8}, {Name: "Forest", Count: 6},
	},
}

// unionCards are the pool's chain-union carriers (their casts are counted).
var unionCards = map[string]bool{"Plague Spores": true, "Spiteful Blow": true, "Boom": true,
	"Stomp and Howl": true, "Boom Box": true, "Arm the Cathars": true, "Relive the Past": true}

// castFirstPolicy casts a spell, a face or activates an ability whenever
// offered (the carriers are expensive, so random play rarely reaches them),
// else random.
func castFirstPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate || k == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestUnionLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("union")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kunionDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kunionDecks[0], kunionDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(512)
	if s := os.Getenv("KUNION_SEEDS"); s != "" {
		seeds = 0
		for _, ch := range s {
			seeds = seeds*10 + uint64(ch-'0')
		}
	}
	casts := map[string]int{}
	var inst []string // instance -> card name (doc.go "Card identity")
	for _, d := range decks {
		for _, c := range d {
			inst = append(inst, cardName(c))
		}
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kunion-a", "kunion-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		for _, a := range res.Actions {
			if k := a.Kind(); k == ActCast || k == ActCastFace || k == ActActivate {
				if x := int(a.Src()); x < len(inst) && unionCards[inst[x]] {
					casts[inst[x]]++
				}
			}
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d seeds; union carrier casts/activations %v", seeds, casts)
	for n := range unionCards {
		if casts[n] == 0 {
			t.Errorf("%s was never cast or activated", n)
		}
	}
}
