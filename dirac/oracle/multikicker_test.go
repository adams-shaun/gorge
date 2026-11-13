package oracle

import (
	"math/rand/v2"
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Multikicker lockstep (CR 702.43): the pool's cards carry
// K:Multikicker and Count$TimesKicked, the class whose only blocker at land
// e46fc98fa8 was the keyword. gorge offers the multikicked cast (base + one
// payment) and asks the count before X and the targets; the kernel offers the
// same kicked cast and poses the count as DkChoose/ChMultikick
// (kw_multikicker.go).
//
// The Everflowing Chalice deck casts from its lands; the Bloodhusk Ritualist
// deck floats its repeated cost with Dark Ritual first, so the multikicked
// option is gorge's offered option rather than a potential-plan lowering.
// Comet Storm and Strength of the Tajuru are not in the fixture: their {X}
// base cost keeps them refused by name in kgen (the count bound would be read
// before the X announcement).

type kmultikickerDeck struct {
	name  string
	decks [2][]cardset.Entry
}

func kmultikickerDecks() []kmultikickerDeck {
	return []kmultikickerDeck{
		{"everflowing", [2][]cardset.Entry{
			{{Name: "Everflowing Chalice", Count: 4}, {Name: "Forest", Count: 36}},
			{{Name: "Everflowing Chalice", Count: 4}, {Name: "Forest", Count: 36}},
		}},
		{"bloodhusk", [2][]cardset.Entry{
			{{Name: "Bloodhusk Ritualist", Count: 4}, {Name: "Dark Ritual", Count: 8}, {Name: "Swamp", Count: 28}},
			{{Name: "Bloodhusk Ritualist", Count: 4}, {Name: "Dark Ritual", Count: 8}, {Name: "Swamp", Count: 28}},
		}},
	}
}

// kmultikickerTally counts the multikicked casts taken and the count answers.
type kmultikickerTally struct {
	casts  map[string]int
	counts map[int]int
}

// kmultikickerPolicy takes a multikicked cast whenever one is offered, casts
// Dark Ritual first to float the repeated cost, and answers a positive count
// two times in three.
func kmultikickerPolicy(seed uint64, tl *kmultikickerTally) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x6d756c74))
	var opts []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DAttackers || c.D.Kind == DBlockers {
			for _, a := range c.D.Options {
				if a.Kind() == ActDone {
					return a
				}
			}
			return rnd.Pick(c)
		}
		if code, ok := chooseCode("multikick"); ok {
			opts = opts[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActChoose+CanonKind(code) {
					opts = append(opts, a)
				}
			}
			if len(opts) > 0 {
				if len(opts) > 1 && r.IntN(3) != 0 {
					a := opts[1+r.IntN(len(opts)-1)]
					tl.counts[int(a.Mode())]++
					return a
				}
				tl.counts[0]++
				return opts[0]
			}
		}
		opts = opts[:0]
		for _, a := range c.D.Options {
			if a.Kind() != ActCast || c.G == nil {
				continue
			}
			o, ok := c.G.GorgeOption(a)
			if !ok {
				continue
			}
			nm := c.G.E.G.Obj(o.Obj).Face().Name
			if o.Mode == "multikicked" || nm == "Dark Ritual" {
				opts = append(opts, a)
			}
		}
		if len(opts) > 0 && r.IntN(3) != 0 {
			a := opts[r.IntN(len(opts))]
			if o, ok := c.G.GorgeOption(a); ok && o.Mode == "multikicked" {
				tl.casts[c.G.E.G.Obj(o.Obj).Face().Name]++
			}
			return a
		}
		return rnd.Pick(c)
	})
}

// runMultikicker runs the full (not autopay) surface on every seed: the
// multikicked offer and its count ask are gorge's own offered options, so the
// two sides meet without the autopay surface's potential-plan filtering.
func runMultikicker(t *testing.T, decks [2][]cardset.Entry, seeds uint64, policy func(uint64) Policy) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("multikicker")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"multikicker-a", "multikicker-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, policy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
}

// TestMultikickerLockstep plays each deck and requires its card to be cast
// multikicked with a positive count, with no divergence.
func TestMultikickerLockstep(t *testing.T) {
	t.Parallel()
	seeds := uint64(32)
	if v := os.Getenv("KMULTIKICKER_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	for _, d := range kmultikickerDecks() {
		d := d
		t.Run(d.name, func(t *testing.T) {
			tl := &kmultikickerTally{casts: map[string]int{}, counts: map[int]int{}}
			runMultikicker(t, d.decks, seeds, func(seed uint64) Policy {
				return kmultikickerPolicy(seed, tl)
			})
			t.Logf("multikicked casts %v; counts %v", tl.casts, tl.counts)
			if len(tl.casts) == 0 {
				t.Errorf("no multikicked cast")
			}

			positive := 0
			for n, c := range tl.counts {
				if n > 0 {
					positive += c
				}
			}
			if positive == 0 {
				t.Error("no positive multikicker count was answered")
			}
		})
	}
}
