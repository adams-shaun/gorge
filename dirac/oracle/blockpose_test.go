package oracle

import (
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The blocker-pose lockstep (CR 509.1b): gorge computes a blockers
// declaration's whole option list at the pose, while none of the chosen
// creatures is blocking yet; the kernel decomposes it into per-pick decisions
// whose options are re-derived from the current state, where the picks served
// so far already carry pfBlocking. A static whose condition or filter reads
// blocking state — here Crescendo of War's "blocking creatures you control get
// +1/+0" — therefore lifts a served blocker's power across Delney's
// CantBlockBy cap (power-3-or-greater creatures can't block a power-2-or-less
// attacker) between the kernel's picks, and menace (Rograkh, Goblin
// Champion, Goblin Arsonist) needs two able blockers, so the able count drops
// below the floor mid-declaration and the kernel stops offering pairs gorge
// posed.

// kblockposeDecks: the first seat caps (Delney) and attacks with small
// menace creatures; the second seat pumps its blocking creatures (Crescendo
// of War) and blocks with Grizzly Bears.
var kblockposeDecks = [2][]cardset.Entry{
	{
		{Name: "Delney, Streetwise Lookout", Count: 4}, {Name: "Goblin Arsonist", Count: 4},
		{Name: "Goblin Champion", Count: 4}, {Name: "Rakdos Trumpeter", Count: 4},
		{Name: "Rograkh, Son of Rohgahh", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 10}, {Name: "Swamp", Count: 2},
	},
	{
		{Name: "Crescendo of War", Count: 4}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Hill Giant", Count: 2}, {Name: "Wall of Blossoms", Count: 2},
		{Name: "Forest", Count: 12}, {Name: "Plains", Count: 12},
	},
}

// kblockposeMenace are the attackers whose menace floor the pumping served
// blocker must break.
var kblockposeMenace = map[string]bool{"Rograkh, Son of Rohgahh": true, "Rakdos Trumpeter": true}

// kblockposeGuard stays home: Delney (the cap must survive) never attacks or
// blocks; the blocker deck's bears and walls stay back (the walls cannot
// attack anyway), so a pair of them is alive when the window opens.
var kblockposeGuard = map[string]bool{"Delney, Streetwise Lookout": true, "Grizzly Bears": true, "Wall of Blossoms": true}

type kblockposeStats struct {
	games, divs, stalls, arsBlocks, arsFloors int
}

// kblockposePolicy plays lands and casts first; attacks with everything
// except a kblockposeGuard creature; blocks eagerly, serving a Grizzly Bear
// on a menace attacker first (the menace floor is what the pumping served
// blocker must break), with a non-Delney blocker otherwise.
func kblockposePolicy(seed uint64, st *kblockposeStats) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DAttackers {
			for i := range c.D.Options {
				o := &c.D.Options[i]
				if o.Kind() != ActAttack {
					continue
				}
				if c.G != nil && kblockposeGuard[gorgeOptionName(c, *o)] {
					continue
				}
				return *o
			}
			return Pack(ActDone, RefNone, RefNone, 0) // only guards: stay home
		}
		if c.D.Kind == DBlockers {
			bearMenace, bearAny, any, nars := -1, -1, -1, 0
			for i := range c.D.Options {
				o := &c.D.Options[i]
				if o.Kind() != ActBlock {
					continue
				}
				if c.G != nil && gorgeOptionName(c, *o) == "Delney, Streetwise Lookout" {
					continue // the cap must survive
				}
				bear := c.G != nil && gorgeOptionName(c, *o) == "Grizzly Bears"
				menace := c.G != nil && kblockposeMenace[gorgeAttackerName(c, *o)]
				if menace {
					nars++
				}
				if bear && menace && bearMenace < 0 {
					bearMenace = i
				}
				if bear && bearAny < 0 {
					bearAny = i
				}
				if any < 0 {
					any = i
				}
			}
			switch {
			case bearMenace >= 0:
				st.arsBlocks++
				if nars >= 2 {
					st.arsFloors++
				}
				return c.D.Options[bearMenace]
			case bearAny >= 0:
				return c.D.Options[bearAny]
			case any >= 0:
				return c.D.Options[any]
			}
			for i := range c.D.Options {
				if c.D.Options[i].Kind() == ActDone {
					return c.D.Options[i]
				}
			}
		}
		for _, o := range c.D.Options {
			if o.Kind() == ActPlayLand || o.Kind() == ActCast {
				return o
			}
		}
		return rnd.Pick(c)
	})
}

// runBlockPose plays seeds of the constructed matchup. mutate turns the
// pose-board lift off (the mutation must diverge). The kernel-side reads of
// the pose board are the subject: the callers assert the coupling fired.
func runBlockPose(t *testing.T, seeds int, mutate bool) *kblockposeStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("blockpose")
	if err != nil {
		t.Fatal(err)
	}
	// Preconditions, so the lockstep below reads the coupling and not a plain
	// creature: Delney lowers the CantBlockBy cap; Crescendo's blocker pump
	// keys on the blocking state; the small attackers carry menace, and
	// under one strife counter attack at power 2 or less (Delney's own half
	// is powerLE2).
	names := set.Names()
	delney := slices.Index(names, "Delney, Streetwise Lookout")
	crescendo := slices.Index(names, "Crescendo of War")
	arsonist := slices.Index(names, "Goblin Arsonist")
	champion := slices.Index(names, "Goblin Champion")
	rograkh := slices.Index(names, "Rograkh, Son of Rohgahh")
	for _, at := range []int{delney, crescendo, arsonist, champion, rograkh} {
		if at < 0 {
			t.Fatalf("blockpose table misses a coupling card (found %v)", names)
		}
	}
	chaDef := set.Def(champion)
	if chaDef.Types&kop.TCreature == 0 || chaDef.Power != 0 {
		t.Fatalf("Goblin Champion CardDef = types %x power %d, want a power-0 creature", chaDef.Types, chaDef.Power)
	}
	var delneyCap bool
	for _, a := range set.Def(delney).Abilities {
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCantBlockBy {
			delneyCap = true
		}
	}
	if set.Def(rograkh).Keywords&kop.KwMenace == 0 {
		t.Fatal("Rograkh's CardDef lacks the menace keyword")
	}
	arsDef := set.Def(arsonist)
	if arsDef.Types&kop.TCreature == 0 || arsDef.Power != 1 {
		t.Fatalf("Goblin Arsonist CardDef = types %x power %d, want a power-1 creature", arsDef.Types, arsDef.Power)
	}
	if !delneyCap {
		t.Fatal("Delney's CantBlockBy cap did not lower")
	}
	var decks [2][]*cards.Card
	for s, d := range kblockposeDecks {
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
	tbl, _, _, err := set.Matchup(kblockposeDecks[0], kblockposeDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	// Crescendo's blocker half must key on the blocking state itself: its
	// Affected filter carries FBlocking.
	var cresPump bool
	for i := range set.Def(crescendo).Abilities {
		a := &set.Def(crescendo).Abilities[i]
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatContinuous {
			continue
		}
		if fl := &tbl.Filters[a.Static.Filter]; fl.Flags&kop.FBlocking != 0 {
			cresPump = true
		}
	}
	if !cresPump {
		t.Fatal("Crescendo of War's blocker pump did not lower with a blocking-keyed filter")
	}
	st := &kblockposeStats{}
	old := dirac.DebugBlkLiftOff
	dirac.DebugBlkLiftOff = mutate
	defer func() { dirac.DebugBlkLiftOff = old }()
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"blockpose-a", "blockpose-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kblockposePolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		st.games++
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			st.stalls++
			if !mutate {
				t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			st.divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return st
}

// TestBlockPoseLockstep plays the blocker-pose pool through both engines: the
// kernel's mid-declaration option reads must see the pose board's blocking
// marks, or the pumping served blocker breaks the menace floor's able count.
func TestBlockPoseLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runBlockPose(t, 64, false)
	fmt.Fprintf(os.Stderr, "blockpose stats: %d games, %d divergences, %d stalls, menace blocks %d, of which with a floor pair %d\n",
		st.games, st.divs, st.stalls, st.arsBlocks, st.arsFloors)
	t.Logf("%d games, %d divergences, %d stalls, blocks on the menace attacker %d",
		st.games, st.divs, st.stalls, st.arsBlocks)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.arsBlocks < 40 {
		t.Errorf("only %d blocks served on the menace attacker", st.arsBlocks)
	}
	if st.arsFloors < 20 {
		t.Errorf("only %d menace floor pairs posed", st.arsFloors)
	}
}

// TestBlockPoseMutation lets the kernel's mid-declaration reads see the
// blocking marks again: the pumping served blocker breaks the menace floor's
// able count and the games must diverge.
func TestBlockPoseMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runBlockPose(t, 64, true)
	if st.divs == 0 && st.stalls == 0 {
		t.Fatalf("mutation restored the mid-declaration reads in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games, %d stalls", st.divs, st.games, st.stalls)
}
