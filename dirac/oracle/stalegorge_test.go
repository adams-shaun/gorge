package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The stale-gorge-refusal lockstep: two kgen
// refusals guarded gorge defects gorge has since fixed. Matchup 0 plays the
// Discard$ Permanent.nonLand mode (Dai Li Indoctrination, Solve for
// Disappointment) against a hand of nonland permanent cards, lands and
// instants; matchup 1 plays North Pole Patrol's `Waterbend<3> T` activation,
// whose source must not be one of its own waterbend picks (gorge castasks.go
// SourceTaps).

const kstalegorgeSeeds = 200

var kstalegorgeDecks = [][2][]cardset.Entry{{
	{
		{Name: "Solve for Disappointment", Count: 6}, {Name: "Dai Li Indoctrination", Count: 6},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Swamp", Count: 10}, {Name: "Island", Count: 10},
	}, {
		{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Pacifism", Count: 4},
		{Name: "Ornithopter", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4},
	},
}, {
	{
		{Name: "North Pole Patrol", Count: 8}, {Name: "Ornithopter", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Divination", Count: 2}, {Name: "Island", Count: 10}, {Name: "Forest", Count: 8},
	}, {
		{Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Savannah Lions", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	},
}}

type kstalegorgeTally struct {
	games     int
	discards  int // Dai Li / Solve discards of a nonland permanent card
	waterbend int // North Pole Patrol waterbend activations
}

// kstalegorgePolicy activates North Pole Patrol and casts Dai Li (in its
// Discard mode) and Solve whenever offered, picks waterbend taps randomly,
// and plays the rest at random.
func kstalegorgePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		switch c.D.Kind {
		case DPriority:
			for _, a := range c.D.Options {
				if k := a.Kind(); (k == ActCast || k == ActActivate) && c.G != nil {
					if o, ok := c.G.GorgeOption(a); ok {
						if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
							switch ob.Face().Name {
							case "North Pole Patrol", "Dai Li Indoctrination", "Solve for Disappointment":
								buf = append(buf, a)
							}
						}
					}
				}
			}
		case DModes:
			for _, a := range c.D.Options {
				if a.Kind() == ActMode && a.Mode() == 0 {
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

func isNonlandPermanent(f *cards.Face) bool {
	if f == nil {
		return false
	}
	perm, land := false, false
	for _, t := range f.Types {
		switch t {
		case "Land":
			land = true
		case "Creature", "Artifact", "Enchantment", "Planeswalker", "Battle":
			perm = true
		}
	}
	return perm && !land
}

// kstalegorgeRun plays 200 seeds of every matchup in lockstep. mut (may be
// nil) edits every instruction of the lowered table; it returns the diverged
// games, and with !report stops a matchup at its first divergence.
func kstalegorgeRun(t *testing.T, n uint64, report bool, tal *kstalegorgeTally, only int, mut func(*kop.Instr)) (div, stalls int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("stalegorge")
	if err != nil {
		t.Fatal(err)
	}
	for mi, md := range kstalegorgeDecks {
		if only >= 0 && mi != only {
			continue
		}
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
					for ii := range tbl.Cards[ci].Abilities[ai].Prog {
						mut(&tbl.Cards[ci].Abilities[ai].Prog[ii])
					}
				}
			}
		}
		for seed := uint64(1); seed <= n; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kstalegorge-a", "kstalegorge-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			seen := 0
			cause := "" // the spell the last stack push cast
			check := func(ref *GorgeSide, _ Side) *Divergence {
				g := ref.E.G
				evs := ref.E.L.Events
				for ; seen < len(evs); seen++ {
					ev := evs[seen]
					if tal == nil {
						continue
					}
					switch {
					case ev.Kind == events.StepChange:
						cause = ""
					case ev.Kind == events.AbilityPush:
						// an activated ability's stack object (its Obj is the source)
						if ob := g.Obj(ev.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "North Pole Patrol" {
							tal.waterbend++
						}
					case ev.Kind == events.PutOnStack:
						cause = ""
						if ob := g.Obj(ev.Obj); ob != nil && ob.Face() != nil {
							cause = ob.Face().Name
						}
					case events.IsDiscard(ev) && (cause == "Dai Li Indoctrination" || cause == "Solve for Disappointment"):
						if ob := g.Obj(ev.Obj); ob != nil && isNonlandPermanent(ob.Face()) {
							tal.discards++
						}
					}
				}
				return nil
			}
			res := Lockstep(m, seed, k, kstalegorgePolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40, Check: check})
			CloseSide(k)
			if tal != nil {
				tal.games++
			}
			if res.Stall != "" && res.Stall != "turns" {
				stalls++
				if report {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				div++
				if !report {
					t.Logf("matchup %d seed %d diverges (decision %d: %s)", mi, seed, d.At, d.What)
					break
				}
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div, stalls
}

func TestStalegorgeLockstep(t *testing.T) {
	t.Parallel()
	var tal kstalegorgeTally
	div, stalls := kstalegorgeRun(t, kstalegorgeSeeds, true, &tal, -1, nil)
	t.Logf("%d games: %d diverged, %d stalls, %d nonland-permanent discards, %d North Pole Patrol waterbend activations",
		tal.games, div, stalls, tal.discards, tal.waterbend)
	if tal.discards < 20 {
		t.Errorf("only %d Dai Li / Solve discards of a nonland permanent card, want >= 20", tal.discards)
	}
	if tal.waterbend < 20 {
		t.Errorf("only %d North Pole Patrol waterbend activations, want >= 20", tal.waterbend)
	}
}

// TestStalegorgeMutation: offering a {T} ability's source as its own
// waterbend pick, or dropping the Discard filter (lands and instants become
// choosable), must diverge some seed.
func TestStalegorgeMutation(t *testing.T) {
	t.Run("waterbend source offered", func(t *testing.T) {
		old := dirac.DebugWaterbendSrcTap
		dirac.DebugWaterbendSrcTap = true
		defer func() { dirac.DebugWaterbendSrcTap = old }()
		if div, _ := kstalegorgeRun(t, kstalegorgeSeeds, false, nil, 1, nil); div == 0 {
			t.Fatal("DebugWaterbendSrcTap did not diverge the waterbend matchup")
		}
	})
	t.Run("discard filter dropped", func(t *testing.T) {
		clear := func(in *kop.Instr) {
			if in.Op == kop.OpDiscard {
				in.F = 0
			}
		}
		if div, _ := kstalegorgeRun(t, kstalegorgeSeeds, false, nil, 0, clear); div == 0 {
			t.Fatal("clearing the Discard filter did not diverge the discard matchup")
		}
	})
}
