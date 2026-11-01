package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// gorge (the second half) makes a chained
// Counter link whose OWN optional target answered EMPTY apply the unpaid
// orientation with NO unless_pay ask: there is no "its controller" to charge
// (effects/unless.go unlessProceed, the ownTargets != nil && len == 0 arm).
// Repulsive Mutation (MKM) is the carrier: its DBCounter is "counter up to one
// target spell unless ITS controller pays". The pool pits it against creature
// spells and burn so the link can target a spell; the policy answers the
// chained target ask empty on half the casts and targets an offered spell on
// the other half, so both the no-payer path and the spell-controller payer
// path run.

var repulsiveMutationDecks = [][2][]cardset.Entry{{
	{
		{Name: "Repulsive Mutation", Count: 12},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 12}, {Name: "Forest", Count: 12},
	},
	{
		{Name: "Raging Goblin", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Child of Night", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 4},
	},
}}

type repulsiveMutationStats struct {
	casts       int // the policy cast Repulsive Mutation
	empty       int // the chained target ask answered with the explicit empty set
	spellTarget int // the chained target ask answered with an offered spell
	unlessAsks  int // a chained Counter's unless_pay ask was posed
}

// repulsiveMutationPolicy casts Repulsive Mutation aggressively (in response to an
// opponent's spell, or a quarter of the time from hand), answers its chained
// target ask empty half the time (forceEmpty: always), and picks pay/decline
// on the unless ask. The rest is the seeded random policy.
func repulsiveMutationPolicy(seed uint64, st *repulsiveMutationStats, forceEmpty bool) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6b666f6c3339)) // "kfol39"
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		if c.D.Kind == DTarget {
			if d := c.G.Pending(); d != nil && d.Kind == decision.KTarget && d.Min == 0 &&
				chName(c.G.E.G, d.Source) == "Repulsive Mutation" {
				var done Canon
				hasDone := false
				buf = buf[:0]
				for _, a := range c.D.Options {
					switch a.Kind() {
					case ActDone:
						done, hasDone = a, true
					case ActTarget:
						buf = append(buf, a)
					}
				}
				if forceEmpty || len(buf) == 0 || prng.IntN(2) == 0 {
					if hasDone {
						st.empty++
						return done
					}
				} else {
					st.spellTarget++
					return buf[prng.IntN(len(buf))]
				}
			}
			return rnd.Pick(c)
		}
		if c.D.Kind == DModes {
			if d := c.G.Pending(); d != nil && d.ResumeKind == "unless_pay" {
				st.unlessAsks++
				want := "unless_decline"
				if prng.IntN(2) == 0 {
					want = "unless_pay"
				}
				for i, o := range d.Options {
					if o.Mode != want {
						continue
					}
					for _, a := range c.D.Options {
						if a.Kind() == ActMode && int(a.Mode()) == i {
							return a
						}
					}
				}
			}
			return rnd.Pick(c)
		}
		if c.D.Kind != DPriority {
			return rnd.Pick(c)
		}
		g := c.G.E.G
		var top *state.Object
		if n := len(g.Stack); n > 0 {
			top = g.Obj(g.Stack[n-1])
		}
		oppSpell := top != nil && top.Controller != state.PlayerID(c.D.Player)
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() != ActCast {
				continue
			}
			if o, ok := c.G.GorgeOption(a); ok && chName(g, o.Obj) == "Repulsive Mutation" {
				buf = append(buf, a)
			}
		}
		if len(buf) > 0 && (oppSpell || prng.IntN(4) == 0) {
			st.casts++
			return buf[prng.IntN(len(buf))]
		}
		return rnd.Pick(c)
	})
}

// repulsiveMutationRun plays seeds 1..n both seatings. mut patches the generated
// table before play.
func repulsiveMutationRun(t *testing.T, n uint64, st *repulsiveMutationStats, forceEmpty, stopAtDiv, quiet bool, mut func(*kop.Table)) int {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("unlesschain")
	if err != nil {
		t.Fatal(err)
	}
	divs := 0
	for mi, md0 := range repulsiveMutationDecks {
		for swap := 0; swap < 2; swap++ {
			md := md0
			if swap == 1 {
				md = [2][]cardset.Entry{md0[1], md0[0]}
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
				mut(tbl)
			}
			for seed := uint64(1); seed <= n; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"deck-a", "deck-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, repulsiveMutationPolicy(seed, st, forceEmpty), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					t.Errorf("matchup %d swap %d seed %d: kernel stall %q", mi, swap, seed, res.Stall)
				}
				if d := res.Div; d != nil {
					divs++
					if quiet {
						t.Logf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v",
							mi, swap, seed, d.At, d.What, d.Field, d.Diff)
					} else {
						t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
							mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					}
					if stopAtDiv {
						return divs
					}
				}
			}
		}
	}
	return divs
}

func TestRepulsiveMutationLockstep(t *testing.T) {
	t.Parallel()
	var st repulsiveMutationStats
	divs := repulsiveMutationRun(t, 64, &st, false, false, false, nil)
	t.Logf("128 games, %d divergences, %+v", divs, st)
	if st.casts < 8 || st.empty < 2 || st.spellTarget < 2 || st.unlessAsks < 2 {
		t.Errorf("coverage: casts %d, empty chained target %d, spell chained target %d, unless asks %d; want at least 8, 2, 2, 2",
			st.casts, st.empty, st.spellTarget, st.unlessAsks)
	}
	if divs != 0 {
		t.Fatalf("%d divergences, want 0", divs)
	}
}

// TestRepulsiveMutationPayerMutation is the old payer: the chained link's unless gate
// charges the resolving controller instead of reading its own target's
// controller. The policy leaves every chained target empty, so this is
// exactly gorge's pre-820ecf0de fallback on the empty own target; the pool
// must diverge.
func TestRepulsiveMutationPayerMutation(t *testing.T) {
	t.Parallel()
	patched := 0
	divs := repulsiveMutationRun(t, 64, &repulsiveMutationStats{}, true, true, true, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			if tbl.Cards[i].Name != "Repulsive Mutation" {
				continue
			}
			for j := range tbl.Cards[i].Abilities {
				for k := range tbl.Cards[i].Abilities[j].Prog {
					if in := &tbl.Cards[i].Abilities[j].Prog[k]; in.Op == kop.OpUnlessPaid && in.Obj == kop.RefTgtCtrl1 {
						in.Obj = kop.RefController
						patched++
					}
				}
			}
		}
	})
	if patched == 0 {
		t.Fatal("no unless instruction in Repulsive Mutation was patched")
	}
	if divs == 0 {
		t.Fatal("no seed diverged with the chained link's payer reverted to the resolving controller on an empty own target")
	}
	t.Logf("mutation diverged on %d of 128 games", divs)
}
