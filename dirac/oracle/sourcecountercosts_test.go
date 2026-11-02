package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The kc10b lockstep (B/C): resolution costs paid with the resolving
// source's own counters (SubCounter, gorge trigger_cost_subcounter.go /
// unless_cost.go) and a printed trigger's own literal tapXType cost
// (gorge cumulative.go:1386, triggered_literal_tap.go). carriers pay or
// decline a trigger-own SubCounter cost; (Magmatic Sprinter) poses the
// end-step unless ask. C carriers elect N untapped permanents; a payable
// election is the payment (no pay/decline ask), an unpayable one is a
// decline-only ask. Run as its own shard (-run '^TestSourceCounterCostsLockstep$').
var sourcecountercostsDecks = [][2][]cardset.Entry{{
	{
		{Name: "Atraxa's Skitterfang", Count: 2}, {Name: "Forgehammer Centurion", Count: 2},
		{Name: "Guiding Hydra", Count: 2}, {Name: "Ingenious Prodigy", Count: 2},
		{Name: "Lattice-Blade Mantis", Count: 2}, {Name: "Magmatic Sprinter", Count: 2},
		{Name: "Aziza, Mage Tower Captain", Count: 1}, {Name: "Caparocti Sunborn", Count: 1},
		{Name: "Civil Servant", Count: 1}, {Name: "Gitaxian Anatomist", Count: 1},
		{Name: "Raff, Weatherlight Stalwart", Count: 1},
		{Name: "Shock", Count: 2}, {Name: "Giant Growth", Count: 1},
		{Name: "Plains", Count: 3}, {Name: "Island", Count: 3}, {Name: "Swamp", Count: 3},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Atraxa's Skitterfang", Count: 1}, {Name: "Lattice-Blade Mantis", Count: 1},
		{Name: "Magmatic Sprinter", Count: 1},
		{Name: "Meanders Guide", Count: 2},
		{Name: "Raff, Weatherlight Stalwart", Count: 1},
		{Name: "Aziza, Mage Tower Captain", Count: 1}, {Name: "Caparocti Sunborn", Count: 1},
		{Name: "Civil Servant", Count: 1}, {Name: "Gitaxian Anatomist", Count: 1},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Gray Ogre", Count: 2},
		{Name: "Hill Giant", Count: 1},
		{Name: "Shock", Count: 2}, {Name: "Giant Growth", Count: 1},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 7}, {Name: "Swamp", Count: 2},
		{Name: "Mountain", Count: 4}, {Name: "Forest", Count: 3},
	},
}}

// sourcecountercostsB carriers pay a trigger-own SubCounter cost; sourcecountercostsC carriers pose a
// literal tapXType election; sourcecountercostsUnless carries the end-step unless ask.
var (
	sourcecountercostsB = map[string]bool{
		"Atraxa's Skitterfang": true, "Forgehammer Centurion": true,
		"Guiding Hydra": true, "Ingenious Prodigy": true,
		"Lattice-Blade Mantis": true, "Magmatic Sprinter": true,
	}
	sourcecountercostsC = map[string]bool{
		"Aziza, Mage Tower Captain": true, "Caparocti Sunborn": true,
		"Civil Servant": true, "Gitaxian Anatomist": true,
		"Meanders Guide": true, "Raff, Weatherlight Stalwart": true,
	}
	sourcecountercostsUnless = map[string]bool{"Magmatic Sprinter": true}
)

func sourcecountercostsName(c *PickCtx, d *decision.Decision) string {
	if c.G == nil || d == nil {
		return ""
	}
	ob := c.G.E.G.Obj(d.Source)
	if ob == nil || ob.Face() == nil {
		return ""
	}
	return ob.Face().Name
}

// sourcecountercostsPolicy plays a land whenever it can, then casts or activates, else
// random, and on the B/C asks answers deliberately so every arm is counted:
// B pays and declines alternately (unpayable = decline-only)
// the unless ask, C elects (the only choice a payable literal tap cost has).
func sourcecountercostsPolicy(seed uint64, counts map[string]int) Policy {
	rnd := RandomPolicy(seed)
	flip := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			if d := c.G.Pending(); d != nil {
				name := sourcecountercostsName(c, d)
				if d.ResumeKind == "unless_pay" && sourcecountercostsUnless[name] {
					want := uint8(0)
					if flip%2 == 0 {
						want = 1
					}
					for _, a := range c.D.Options {
						if a.Kind() == ActMode && a.Mode() == want {
							if want == 1 {
								counts["B2/pay"]++
							} else {
								counts["B2/decline"]++
							}
							flip++
							return a
						}
					}
				}
				hasPay, hasDecline, hasTap := false, false, false
				for _, o := range d.Options {
					switch o.Kind {
					case "trigger_cost_pay":
						hasPay = true
					case "trigger_cost_decline":
						hasDecline = true
					case "trigger_cost_tap":
						hasTap = true
					}
				}
				switch {
				case hasTap && sourcecountercostsC[name]:
					n := int(d.Min)
					switch {
					case n >= 2:
						counts["C/multi"]++
					case len(d.Options) == 1 && d.Options[0].Obj == d.Source:
						counts["C/self"]++
					default:
						counts["C/other"]++
					}
					return rnd.Pick(c)
				case hasPay && sourcecountercostsB[name]:
					want := uint8(0)
					if flip%2 == 0 {
						want = 1
					}
					for i, o := range d.Options {
						switch o.Kind {
						case "trigger_cost_pay":
							if want == 1 {
								counts["B/paid"]++
								flip++
								return c.D.Options[i]
							}
						case "trigger_cost_decline":
							if want == 0 {
								counts["B/declined"]++
								flip++
								return c.D.Options[i]
							}
						}
					}
				case hasDecline && sourcecountercostsB[name]:
					counts["B/unpayable"]++
				case hasDecline && sourcecountercostsC[name]:
					counts["C/unpayable"]++
				}
			}
		}
		var buf []Canon
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate {
					buf = append(buf, a)
				}
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

func sourcecountercostsEnv(t *testing.T) (cardset.Set, func(mi int) (*Matchup, error)) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("enterstapped")
	if err != nil {
		t.Fatal(err)
	}
	build := func(mi int) (*Matchup, error) {
		md := sourcecountercostsDecks[mi]
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
		return NewMatchup(reg, [2]string{"kc10b-a", "kc10b-b"}, decks, 1)
	}
	return set, build
}

func TestSourceCounterCostsLockstep(t *testing.T) {
	t.Parallel()

	set, build := sourcecountercostsEnv(t)
	counts := map[string]int{}
	games := 0
	for mi := range sourcecountercostsDecks {
		md := sourcecountercostsDecks[mi]
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 1024; seed++ {
			m, err := build(mi)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, sourcecountercostsPolicy(seed, counts), RunConfig{Config: cfg, MaxDecisions: 1500, MaxTurns: 16})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, counts %v", games, counts)
	for _, arm := range []string{"B/paid", "B/declined", "B/unpayable", "B2/pay", "B2/decline", "C/other", "C/multi", "C/self", "C/unpayable"} {
		if counts[arm] < 10 {
			t.Errorf("%s: %d, want at least 10", arm, counts[arm])
		}
	}
}

// TestSourceCounterCostsMutation: each lifted B/C arm must diverge when mutated.
// (3) SubCounter pays without draining the source's counter;
// (4) a TapX election does not tap the elected permanent.
func TestSourceCounterCostsMutation(t *testing.T) {
	set, build := sourcecountercostsEnv(t)
	run := func(name string, mode uint8) {
		dirac.SetDebugEntersTappedMutate(mode)
		defer dirac.SetDebugEntersTappedMutate(0)
		diverged := false
		for mi := range sourcecountercostsDecks {
			md := sourcecountercostsDecks[mi]
			tbl, _, _, err := set.Matchup(md[0], md[1])
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= 256 && !diverged; seed++ {
				m, err := build(mi)
				if err != nil {
					t.Fatal(err)
				}
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				res := Lockstep(m, seed, k, sourcecountercostsPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 1500, MaxTurns: 16})
				CloseSide(k)
				if res.Div != nil {
					t.Logf("%s: matchup %d seed %d diverged at decision %d", name, mi, seed, res.Div.At)
					diverged = true
				}
			}
		}
		if !diverged {
			t.Errorf("%s: mutation diverged in no game", name)
		}
	}
	run("counter not drained", 3)
	run("tap not taken", 4)
}
