package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The as-enters replacement chains with a per-node Condition gate
// Dust Animus ("enters with two +1/+1 counters and a
// lifelink counter if you control five or more untapped lands", each counter
// node gated on its own) and Slumbering Trudge ("enters with 3 - X stun
// counters; if X is 2 or less it enters tapped"). The gate skips its node
// only (gorge effects/registry.go Resolve), the chain continues.
var ketbnodecondA = []cardset.Entry{
	{Name: "Dust Animus", Count: 8}, {Name: "Slumbering Trudge", Count: 6}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Savannah Lions", Count: 4}, {Name: "Plains", Count: 9}, {Name: "Forest", Count: 9},
}

var ketbnodecondB = []cardset.Entry{
	{Name: "Hill Giant", Count: 6}, {Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Giant Growth", Count: 2},
	{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
}

var ketbnodecondDecks = [][2][]cardset.Entry{
	{ketbnodecondA, ketbnodecondB},
	{ketbnodecondB, ketbnodecondA},
}

// ketbnodecondTally counts, on gorge's side and at an entered object's first
// observation, how each carrier entered.
type ketbnodecondTally struct {
	games                        int
	animusCtr, animusBare        int // Dust Animus with / without its counters
	trudgeTapped, trudgeUntapped int // Slumbering Trudge entered tapped / untapped
	trudgeNoStun                 int // entered with no stun counters (X >= 3)
}

// ketbnodecondPolicy plays a land whenever it can, else casts at random (X
// and every other choice uniformly at random); every fourth seed plays fully
// at random.
func ketbnodecondPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	if seed%4 == 0 {
		return rnd
	}
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DPriority {
			for _, want := range [...]CanonKind{ActPlayLand, ActCast} {
				buf = buf[:0]
				for _, a := range c.D.Options {
					if a.Kind() == want {
						buf = append(buf, a)
					}
				}
				if len(buf) > 0 {
					d := *c.D
					d.Options = buf
					return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
				}
			}
		}
		return rnd.Pick(c)
	})
}

func ketbnodecondRun(t *testing.T, n uint64, report bool, tal *ketbnodecondTally, mut func(*kop.Instr)) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("etbnodecond")
	if err != nil {
		t.Fatal(err)
	}
	div := 0
	for mi, md := range ketbnodecondDecks {
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
		if mut != nil {
			for ci := range tbl.Cards {
				for ai := range tbl.Cards[ci].Abilities {
					ab := &tbl.Cards[ci].Abilities[ai]
					if ab.Kind != kop.AbReplacement {
						continue
					}
					for ii := range ab.Prog {
						mut(&ab.Prog[ii])
					}
				}
			}
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ts-a", "ts-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			seen := map[state.ObjID]bool{}
			check := func(ref *GorgeSide, _ Side) *Divergence {
				if tal == nil {
					return nil
				}
				g := ref.E.G
				for _, p := range g.AliveFrom(0) {
					for _, id := range g.Zone(state.ZBattlefield, p) {
						o := g.Obj(id)
						if o == nil || seen[id] {
							continue
						}
						name := cardName(o.Card)
						if name != "Dust Animus" && name != "Slumbering Trudge" {
							continue
						}
						seen[id] = true
						var plus, stun int32
						for _, c := range o.Counters {
							switch c.Kind {
							case "P1P1":
								plus += c.N
							case "STUN":
								stun += c.N
							}
						}
						switch {
						case name == "Dust Animus" && plus > 0:
							tal.animusCtr++
						case name == "Dust Animus":
							tal.animusBare++
						case o.Tapped:
							tal.trudgeTapped++
						default:
							tal.trudgeUntapped++
						}
						if name == "Slumbering Trudge" && stun == 0 {
							tal.trudgeNoStun++
						}
					}
				}
				return nil
			}
			res := Lockstep(m, seed, k, ketbnodecondPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40, Check: check})
			CloseSide(k)
			if tal != nil {
				tal.games++
			}
			if res.Stall != "" && res.Stall != "turns" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				div++
				if !report {
					t.Logf("matchup %d seed %d diverges (decision %d: %s)", mi, seed, d.At, d.What)
					return div
				}
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div
}

func TestEtbnodecondLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	var tal ketbnodecondTally
	ketbnodecondRun(t, 200, true, &tal, nil)
	t.Logf("%+v", tal)
	for _, c := range []struct {
		s string
		n int
	}{
		{"Dust Animus with counters", tal.animusCtr}, {"Dust Animus without counters", tal.animusBare},
		{"Slumbering Trudge tapped", tal.trudgeTapped}, {"Slumbering Trudge untapped", tal.trudgeUntapped},
		{"Slumbering Trudge with no stun counters", tal.trudgeNoStun},
	} {
		if c.n < 5 {
			t.Errorf("%s: %d, want at least 5", c.s, c.n)
		}
	}
}

// TestEtbnodecondMutation: with every replacement SkipIf turned into a Nop
// (each node runs whatever its gate says) some seed diverges.
func TestEtbnodecondMutation(t *testing.T) {
	n := ketbnodecondRun(t, 150, false, nil, func(in *kop.Instr) {
		if in.Op == kop.OpSkipUnless {
			in.Op = kop.OpNop
		}
	})
	if n == 0 {
		t.Error("mutation (replacement SkipUnless dropped): no seed diverged")
	}
}
