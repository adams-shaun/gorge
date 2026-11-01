package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The unless-discard sacrifice lockstep (masticore): constructed
// matchups over the generated masticore pool. Masticore, Molten-Tail
// Masticore, Razormane Masticore and The Great Juggernaut pose an upkeep
// "UnlessCost$ Discard<1/Card>" ask and sacrifice the source when it is
// declined; Bebop & Rocksteady poses the same ask from its attack and
// block triggers. gorge's shared unless gate prices the discard, so both
// answers run through the lockstep and are counted here.
var kmasticoreDecks = [2][]cardset.Entry{
	{
		{Name: "Masticore", Count: 4}, {Name: "Molten-Tail Masticore", Count: 4},
		{Name: "Razormane Masticore", Count: 3}, {Name: "The Great Juggernaut", Count: 3},
		{Name: "Bebop & Rocksteady", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Forest", Count: 6}, {Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 5},
	},
	{
		// A goldfish opponent: no creatures, so combat never reaches the
		// multi-block damage-division projection (unrelated to this pool).
		{Name: "Divination", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Island", Count: 8},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
	},
}

// masticoreCarriers are the pool's unless-discard carriers (their casts are
// counted).
var masticoreCarriers = map[string]bool{
	"Masticore": true, "Molten-Tail Masticore": true, "Razormane Masticore": true,
	"The Great Juggernaut": true, "Bebop & Rocksteady": true,
}

// masticorePolicy alternates pay/decline on every unless-pay ask and
// prefers casting a carrier not yet seen, so every family member reaches
// the battlefield and both answers are exercised.
func masticorePolicy(seed uint64, counts map[string]int, played map[string]int) Policy {
	rnd := RandomPolicy(seed)
	flip := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		var d *decision.Decision
		if c.G != nil {
			d = c.G.Pending()
		}
		if d != nil && d.ResumeKind == "unless_pay" {
			want := uint8(0) // decline
			if flip%2 == 0 {
				want = 1 // pay
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActMode && a.Mode() == want {
					counts["unless"]++
					if want == 1 {
						counts["pay"]++
					} else {
						counts["decline"]++
					}
					flip++
					return a
				}
			}
		}
		a := rnd.Pick(c)
		if c.G != nil {
			for _, o := range c.D.Options {
				if k := o.Kind(); k != ActCast && k != ActCastFace {
					continue
				}
				gop, ok := c.G.GorgeOption(o)
				if !ok {
					continue
				}
				ob := c.G.E.G.Obj(gop.Obj)
				if ob == nil || ob.Face() == nil {
					continue
				}
				name := ob.Face().Name
				if masticoreCarriers[name] && played[name] == 0 {
					a = o
					break
				}
			}
		}
		if k := a.Kind(); (k == ActCast || k == ActCastFace) && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					played[ob.Face().Name]++
				}
			}
		}
		return a
	})
}

func TestMasticoreLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("masticore")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kmasticoreDecks {
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
	tbl, _, _, err := set.Matchup(kmasticoreDecks[0], kmasticoreDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(512)
	if s := os.Getenv("KMASTICORE_SEEDS"); s != "" {
		seeds = 0
		for _, ch := range s {
			seeds = seeds*10 + uint64(ch-'0')
		}
	}
	counts, played := map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kmasticore-a", "kmasticore-b"}, decks, 1)
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
		pol := masticorePolicy(seed, counts, played)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d seeds; unless asks %d (pay %d, decline %d); carrier casts %v", seeds, counts["unless"], counts["pay"], counts["decline"], played)
	for n := range masticoreCarriers {
		if played[n] == 0 {
			t.Errorf("carrier %s never cast", n)
		}
	}
	if counts["pay"] == 0 || counts["decline"] == 0 {
		t.Errorf("unless-discard never %s (pay %d, decline %d)", "exercised both ways", counts["pay"], counts["decline"])
	}
}
