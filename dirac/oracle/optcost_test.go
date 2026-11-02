package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The OptionalCost lockstep: "as an additional cost to cast
// this spell, you may blight 1 / behold a Dragon" (S:Mode$ OptionalCost,
// CR 601.2b). gorge offers the optional cast as its own priority option
// (Mode "optionalcost", AltCostIndex 1) beside the plain one, settles the
// part through the ordinary cost stages and stamps FlagOptionalCostPaid,
// which Count$OptionalGenericCostPaid.<paid>.<unpaid> and Condition$
// OptionalCost read. The kernel's optional cast is the kicked one
// (cost_optional.go).

var koptcostDecks = [][2][]cardset.Entry{{
	// A: the behold carriers, Dragons to behold, and enters-mana creatures
	// whose mana floats at priority (Iridescent Tiger's WUBRG,
	// Burning-Tree Emissary's RG): the pool that pays an optional cast
	{
		{Name: "Dispelling Exhale", Count: 4}, {Name: "Osseous Exhale", Count: 4}, {Name: "Piercing Exhale", Count: 4},
		{Name: "Dragon Moose", Count: 3}, {Name: "Volcanic Dragon", Count: 1},
		{Name: "Iridescent Tiger", Count: 3}, {Name: "Burning-Tree Emissary", Count: 4}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Island", Count: 4}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4},
	},
	// B: the blight carriers, enters-mana creatures to blight (Priest of
	// Urabrask's RRR, Priest of Gix's BBB, Akki Rockspeaker's R), small
	// creatures to destroy
	{
		{Name: "Burning Curiosity", Count: 4}, {Name: "Cinder Strike", Count: 4}, {Name: "Requiting Hex", Count: 4},
		{Name: "Priest of Urabrask", Count: 3}, {Name: "Priest of Gix", Count: 3}, {Name: "Akki Rockspeaker", Count: 2},
		{Name: "Raging Goblin", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Mountain", Count: 9}, {Name: "Swamp", Count: 7},
	},
}}

// koptcostBlight / koptcostBehold name the carriers by part.
var koptcostBlight = map[string]bool{"Burning Curiosity": true, "Cinder Strike": true, "Requiting Hex": true}
var koptcostBehold = map[string]bool{"Dispelling Exhale": true, "Osseous Exhale": true, "Piercing Exhale": true}

// koptcostTally counts the casts the policy chose (gorge's options).
type koptcostTally struct {
	paid, plain, beholds, blights int
}

// koptcostPolicy steers the games to the carriers. gorge's planner prices
// no OptionalCost card ("shape:optional_cost"), so under either oracle
// surface a carrier's casts are offered only when the floating pool pays
// them (the enters-mana creatures' mana): the policy takes an offered
// optional-cost cast two times in three, else a random cast (so the plain
// cast of a carrier gets picked too), else random.
func koptcostPolicy(seed uint64, tl *koptcostTally) Policy {
	rnd := RandomPolicy(seed)
	r := rand.New(rand.NewPCG(seed, 0x6f707463))
	var opt, casts []Canon
	pick := func(c *PickCtx, from []Canon) Canon {
		d := *c.D
		d.Options = from
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		opt, casts = opt[:0], casts[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast:
				casts = append(casts, a)
				if c.G != nil {
					if o, ok := c.G.GorgeOption(a); ok && o.Mode == "optionalcost" {
						opt = append(opt, a)
					}
				}
			}
		}
		var a Canon
		switch {
		case seed%4 == 0 || len(casts) == 0:
			a = rnd.Pick(c)
		case len(opt) > 0 && r.IntN(3) != 0:
			a = pick(c, opt)
		default:
			a = pick(c, casts)
		}
		if c.G == nil || a.Kind() != ActCast {
			return a
		}
		o, ok := c.G.GorgeOption(a)
		if !ok || o.Kind != "cast" {
			return a
		}
		name := c.G.E.G.Obj(o.Obj).Face().Name
		if !koptcostBlight[name] && !koptcostBehold[name] {
			return a
		}
		switch o.Mode {
		case "optionalcost":
			tl.paid++
			if koptcostBlight[name] {
				tl.blights++
			} else {
				tl.beholds++
			}
		case "":
			tl.plain++
		}
		return a
	})
}

// runOptcost plays the matchups for seeds 1..seeds and returns the games,
// the divergences, the kernel capacity stalls and the tally; mutate sets
// DebugOptCostNeverPaid (the mutation must diverge).
func runOptcost(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, tl koptcostTally) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("optcost")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugOptCostNeverPaid
	dirac.DebugOptCostNeverPaid = mutate
	defer func() { dirac.DebugOptCostNeverPaid = old }()
	for mi, md := range koptcostDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"koptcost-a", "koptcost-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, g, k, koptcostPolicy(seed, &tl), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			games++
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					lo := d.At - 12
					if lo < 0 {
						lo = 0
					}
					t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
			CloseSide(k)
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				if !mutate {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
		}
	}
	return games, divs, stalls, tl
}

func TestOptcostLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, tl := runOptcost(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, optional-cost casts %d (behold %d, blight %d), plain carrier casts %d",
		games, divs, stalls, tl.paid, tl.beholds, tl.blights, tl.plain)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if tl.paid < 30 || tl.plain < 30 || tl.beholds < 10 || tl.blights < 10 {
		t.Errorf("thin coverage: optional-cost %d (want 30), plain %d (want 30), behold %d (want 10), blight %d (want 10)",
			tl.paid, tl.plain, tl.beholds, tl.blights)
	}
}

// TestOptcostMutation: DebugOptCostNeverPaid reads every paid optional cost
// as unpaid (Cinder Strike's 2 damage, Requiting Hex's missing life, ...),
// so the lockstep must diverge.
func TestOptcostMutation(t *testing.T) {
	games, divs, _, _ := runOptcost(t, 64, true)
	if divs == 0 {
		t.Fatalf("DebugOptCostNeverPaid: no divergence in %d games", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
