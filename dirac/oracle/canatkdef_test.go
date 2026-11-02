package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The CanAttackDefender lockstep: the five
// Defender carriers of S:Mode$ CanAttackDefender | ValidCard$ Card.Self with
// a condition (an artifact entered under your control this turn, a counter
// on it, a power-4 creature, a Lesson in your graveyard) against artifact
// makers, counter sources, power-4 creatures, a Lesson and burn, so the
// kernel's cold canAttackDefender decides at every declaration the way
// gorge's AttackAllowedThroughDefender does.

var kcanatkdefDecks = [2][]cardset.Entry{
	// U/W/b: Clue makers (Thraben Inspector) and the artifact, counter and Demon Wall carriers
	{
		{Name: "Mechan Shieldmate", Count: 4}, {Name: "Shipwreck Sentry", Count: 4}, {Name: "Demon Wall", Count: 4},
		{Name: "Thraben Inspector", Count: 11},
		{Name: "Island", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Swamp", Count: 6},
	},
	// G/R/u power-4 creatures and Lessons
	{
		{Name: "Bristlepack Sentry", Count: 4}, {Name: "Platypus-Bear", Count: 4}, {Name: "Craw Wurm", Count: 2},
		{Name: "Colossal Dreadmaw", Count: 2}, {Name: "Earth Elemental", Count: 1}, {Name: "Grizzly Bears", Count: 1}, {Name: "Battlegrowth", Count: 2},
		{Name: "Hill Giant", Count: 3}, {Name: "Boomerang Basics", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Lightning Strike", Count: 1}, {Name: "Forest", Count: 7}, {Name: "Mountain", Count: 4},
		{Name: "Island", Count: 4},
	},
}

var kcanatkdefCarriers = map[string]bool{"Mechan Shieldmate": true, "Shipwreck Sentry": true,
	"Demon Wall": true, "Bristlepack Sentry": true, "Platypus-Bear": true}

type kcanatkdefStats struct {
	games, divs, stalls int
	attacks             map[string]int // attacks by a Defender carrier
	notOffered          int            // declarations where an untapped carrier was not offered
}

// kcanatkdefPolicy attacks whenever it can (so a lifted wall is used), plays
// lands and casts spells otherwise, else random; it counts the carrier
// attacks and the declarations that withheld a ready carrier.
func kcanatkdefPolicy(seed uint64, st *kcanatkdefStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	prevAttack := false
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		hasAtk := false
		for _, a := range c.D.Options {
			if a.Kind() == ActAttack {
				hasAtk = true
				buf = append(buf, a)
			}
		}
		if hasAtk && !prevAttack && c.G != nil {
			offered := map[state.ObjID]bool{}
			for _, a := range buf {
				if o, ok := c.G.GorgeOption(a); ok {
					offered[o.Obj] = true
				}
			}
			for _, id := range c.G.E.G.Zone(state.ZBattlefield, state.PlayerID(c.D.Player)) {
				ob := c.G.E.G.Obj(id)
				if ob == nil || ob.Face() == nil || !kcanatkdefCarriers[ob.Face().Name] || ob.Tapped || ob.SummonSick {
					continue
				}
				if !offered[id] {
					st.notOffered++
				}
			}
		}
		prevAttack = hasAtk
		if !hasAtk {
			for _, a := range c.D.Options {
				if k := a.Kind(); k == ActPlayLand || k == ActCast || k == ActActivate {
					buf = append(buf, a)
				}
			}
		}
		a := Canon(0)
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActAttack && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && kcanatkdefCarriers[ob.Face().Name] {
					st.attacks[ob.Face().Name]++
				}
			}
		}
		return a
	})
}

// runCanatkdef plays seeds 1..seeds of the constructed matchup. mutate sets
// DebugCanAtkDefNoCond (the mutation must diverge).
func runCanatkdef(t *testing.T, seeds int, mutate bool) *kcanatkdefStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("canatkdef")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugCanAtkDefNoCond
	dirac.DebugCanAtkDefNoCond = mutate
	defer func() { dirac.DebugCanAtkDefNoCond = old }()
	st := &kcanatkdefStats{attacks: map[string]int{}}
	var decks [2][]*cards.Card
	for s, d := range kcanatkdefDecks {
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
	tbl, _, _, err := set.Matchup(kcanatkdefDecks[0], kcanatkdefDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcanatkdef-a", "kcanatkdef-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kcanatkdefPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestCanatkdefLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runCanatkdef(t, 256, false)
	total := 0
	for _, n := range st.attacks {
		total += n
	}
	t.Logf("%d games, %d divergences, %d stalls, carrier attacks %d %v, declarations withholding a ready carrier %d",
		st.games, st.divs, st.stalls, total, st.attacks, st.notOffered)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if total < 40 {
		t.Errorf("only %d Defender-carrier attacks", total)
	}
	if st.notOffered < 40 {
		t.Errorf("only %d declarations withheld a ready carrier", st.notOffered)
	}
}

// TestCanatkdefMutation lifts the defender wall whatever the static's
// condition says: the games must diverge. A green mutation (no divergence)
// means the lockstep never reaches a declaration where the condition is
// false.
func TestCanatkdefMutation(t *testing.T) {
	st := runCanatkdef(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation ignored the static's condition in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
