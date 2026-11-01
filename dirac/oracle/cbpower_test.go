package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The CantBlockBy ValidBlocker$ Creature.power<op><N> lockstep
// Cavern Stomper ("can't be blocked by creatures with
// power 2 or less this turn", activated on itself) and Cheeky House-Mouse's
// Squeak By (power 3 or greater, on a target creature) beside Giant Growth
// (a blocker's power changes after the grant: gorge reads the blocker's
// CURRENT power at block declaration), Agility Bobblehead (a haste record and a
// power record on one attacker) and creatures of power 1, 2 and 3.
var kcbpowerDeck = []cardset.Entry{
	{Name: "Cavern Stomper", Count: 4}, {Name: "Cheeky House-Mouse", Count: 4}, {Name: "Giant Growth", Count: 4},
	{Name: "Agility Bobblehead", Count: 2}, {Name: "Fugitive Wizard", Count: 3}, {Name: "Savannah Lions", Count: 3},
	{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Plains", Count: 4},
	{Name: "Forest", Count: 5}, {Name: "Island", Count: 2}, {Name: "Mountain", Count: 3},
}

type kcbpowerStats struct {
	games, divs, stalls int
	grants              int // power-form grants (Stomper activations, Squeak By targets)
	refused             int // a creature inside the band of a granted attacker, not offered the block
	legal               int // a block picked by a creature outside the band of a granted attacker
	leaked              int // a creature inside the band offered the block (gorge's own miss)
}

type kcbpowerGrant struct {
	turn int
	le   bool
	n    int32
}

// kcbpowerPolicy casts, activates, attacks and blocks whenever it can, else
// random; it tracks the power-form grants it makes and counts, at each
// declare-blockers decision, the blocks gorge refuses and allows against them.
func kcbpowerPolicy(seed uint64, st *kcbpowerStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	grants := map[state.ObjID]kcbpowerGrant{}
	atk := map[state.ObjID]bool{}
	squeak := false
	prevBlock := false
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActCastFace, ActActivate, ActPlayLand, ActAttack, ActBlock, ActTarget:
				buf = append(buf, a)
			}
		}
		a := c.D.Options[0]
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if c.G == nil {
			return a
		}
		turn := c.G.Turn()
		if c.D.Kind == DBlockers {
			if !prevBlock {
				kcbpowerCount(c, st, grants, atk, turn)
			}
			prevBlock = a.Kind() == ActBlock
			if prevBlock {
				if o, ok := c.G.GorgeOption(a); ok {
					if g, in := grants[o.Attacker]; in && g.turn == turn && !kcbpowerBand(g, c.G.E.Power(o.Obj)) {
						st.legal++
					}
				}
			}
			return a
		}
		prevBlock = false
		o, ok := c.G.GorgeOption(a)
		if !ok {
			return a
		}
		switch a.Kind() {
		case ActAttack:
			atk[o.Obj] = true
		case ActActivate:
			if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Cavern Stomper" && o.Kind != "mana" {
				grants[o.Obj] = kcbpowerGrant{turn, true, 2}
				st.grants++
			}
		case ActCast, ActCastFace:
			if ob := c.G.E.G.Obj(o.Obj); ob != nil {
				squeak = a.Mode()>>4 != 0 && ob.Face() != nil && ob.Face().Name == "Cheeky House-Mouse"
			}
		case ActTarget:
			if squeak {
				grants[o.Obj] = kcbpowerGrant{turn, false, 3}
				st.grants++
				squeak = false
			}
		}
		return a
	})
}

func kcbpowerBand(g kcbpowerGrant, p int32) bool {
	if g.le {
		return p <= g.n
	}
	return p >= g.n
}

// kcbpowerCount classifies every (untapped defender creature, granted
// attacker) pair of a declare-blockers decision.
func kcbpowerCount(c *PickCtx, st *kcbpowerStats, grants map[state.ObjID]kcbpowerGrant, atk map[state.ObjID]bool, turn int) {
	offered := map[[2]state.ObjID]bool{}
	for _, x := range c.D.Options {
		if x.Kind() != ActBlock {
			continue
		}
		if o, ok := c.G.GorgeOption(x); ok {
			offered[[2]state.ObjID{o.Obj, o.Attacker}] = true
		}
	}
	for i := range c.G.E.G.Objs {
		b := &c.G.E.G.Objs[i]
		if b.Zone != state.ZBattlefield || b.Tapped || int(b.Controller) != int(c.D.Player) || !c.G.E.IsCreature(b.ID) {
			continue
		}
		for a, g := range grants {
			if g.turn != turn || !atk[a] {
				continue
			}
			in := kcbpowerBand(g, c.G.E.Power(b.ID))
			switch {
			case in && offered[[2]state.ObjID{b.ID, a}]:
				st.leaked++
			case in:
				st.refused++
			}
		}
	}
}

func runCbpower(t *testing.T, seeds int, stopAtDiv bool) kcbpowerStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cbpower")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kcbpowerDeck {
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
	tbl, _, _, err := set.Matchup(kcbpowerDeck, kcbpowerDeck)
	if err != nil {
		t.Fatal(err)
	}
	var st kcbpowerStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"kcbpower-a", "kcbpower-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kcbpowerPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		st.games++
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			st.stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			st.divs++
			if !stopAtDiv {
				t.Errorf("seed %d: %s %s at %d: %v %s", seed, d.What, d.Field, d.At, d.Diff, d.Detail)
			}
			if stopAtDiv {
				break
			}
		}
	}
	return st
}

func TestCbpowerLockstep(t *testing.T) {
	t.Parallel()
	st := runCbpower(t, 256, false)
	t.Logf("%+v", st)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.grants < 20 {
		t.Errorf("power-form grants %d, want at least 20", st.grants)
	}
	if st.refused < 10 {
		t.Errorf("blocks refused by the power form %d, want at least 10", st.refused)
	}
	if st.legal < 10 {
		t.Errorf("legal blocks outside the band %d, want at least 10", st.legal)
	}
	if st.leaked != 0 {
		t.Errorf("%d in-band blocks offered by gorge", st.leaked)
	}
}

// TestCbpowerMutation: with the records never read, a creature inside the
// band blocks the granted attacker and some seed diverges.
func TestCbpowerMutation(t *testing.T) {
	dirac.DebugExHasteOff = true
	defer func() { dirac.DebugExHasteOff = false }()
	st := runCbpower(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("no divergence in %d games with DebugExHasteOff", st.games)
	}
	t.Logf("mutation diverged after %d games", st.games)
}
