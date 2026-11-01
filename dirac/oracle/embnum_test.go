package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// An Effect's frozen SetChosenNumber$ read as its emblem ReduceCost
// Amount$ Count$ChosenNumber: Rowan, Scion of War
// ("black and/or red spells cost {X} less, X the life you lost this turn")
// and Will, Scion of Peace (white/blue, life gained). gorge evaluates the
// chosen number ONCE when the Effect resolves (effects/effect.go) and the
// registered ReduceCost static carries it frozen; the kernel freezes the
// same value in State.g.embN and adjustCostGen reads it under kop.StEmbNum.
// The counters below prove the deck reaches the carrier: a discounted cast
// after an activation whose frozen X >= 1, and an activation after which the
// live X grew in the same turn (the freeze is observable).

var kembnumDecks = [2][]cardset.Entry{
	// R/B: Rowan, black/red spells at several mana values, and life-loss
	// sources (Char and Flame Rift always cost their controller life; Sign
	// in Blood does when it targets its controller; Child of Night gains it
	// back).
	{
		{Name: "Rowan, Scion of War", Count: 4}, {Name: "Duress", Count: 3},
		{Name: "Sign in Blood", Count: 3}, {Name: "Shock", Count: 4},
		{Name: "Char", Count: 4}, {Name: "Flame Rift", Count: 3},
		{Name: "Lightning Strike", Count: 3}, {Name: "Murder", Count: 2},
		{Name: "Child of Night", Count: 3},
		{Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 4},
	},
	// W/U: Will, white/blue spells at several mana values, and life-gain
	// sources (Revitalize and Renewed Faith gain unconditionally).
	{
		{Name: "Will, Scion of Peace", Count: 4}, {Name: "Opt", Count: 3},
		{Name: "Counterspell", Count: 4}, {Name: "Divination", Count: 3},
		{Name: "Air Elemental", Count: 2}, {Name: "Pacifism", Count: 3},
		{Name: "Revitalize", Count: 4}, {Name: "Renewed Faith", Count: 3},
		{Name: "Serra Angel", Count: 2},
		{Name: "Plains", Count: 6}, {Name: "Island", Count: 6},
	},
}

type kembnumStats struct {
	games, divs, stalls     int
	activations, discounted int
	casts                   int
	grew                    int // activations the live X outgrew in the same turn
	firstDiv                uint64
}

// kembnumColors is the spell colour the emblem of kind k discounts.
func kembnumColors(k int) kop.ColorMask {
	if k == 1 { // Rowan
		return kop.ColB | kop.ColR
	}
	return kop.ColW | kop.ColU // Will
}

// kembnumX is the live Count$ head each emblem froze: life lost (Rowan) or
// gained (Will) by the emblem's controller this turn, from gorge's side.
func kembnumX(g *state.Game, p, kind int) int {
	if kind == 1 {
		return int(g.Players[p].LifeLostThisTurn)
	}
	return int(g.Players[p].LifeGainedThisTurn)
}

type kembnumAct struct {
	turn int32
	kind int // 1 Rowan, 2 Will, 0 none
	x    int // the frozen X at activation
	grew bool
}

// kembnumPolicy activates Rowan/Will whenever it is offered and X is already
// >= 1; with X still 0 it first casts an unconditional life-change spell so
// the activation freezes a real number, and otherwise casts. Every decision
// checks whether a pending activation's live X has outgrown its frozen one.
func kembnumPolicy(seed uint64, st *kembnumStats) Policy {
	rnd := RandomPolicy(seed)
	var buf, acts, readyActs, lifeCasts, casts []Canon
	var act [2]kembnumAct
	set := rawSet("embnum")
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		g := c.G.E.G
		turn, dp := g.Turn, int(c.D.Player)
		for p := 0; p < 2; p++ {
			if a := &act[p]; a.kind != 0 && a.turn == turn && !a.grew {
				if x := kembnumX(g, p, a.kind); x > a.x {
					a.grew = true
					st.grew++
				}
			}
		}
		actKind := func(a Canon) (int, bool) {
			if a.Kind() != ActActivate {
				return 0, false
			}
			o, ok := c.G.GorgeOption(a)
			if !ok {
				return 0, false
			}
			ob := g.Obj(o.Obj)
			if ob == nil || ob.Face() == nil {
				return 0, false
			}
			switch ob.Face().Name {
			case "Rowan, Scion of War":
				return 1, true
			case "Will, Scion of Peace":
				return 2, true
			}
			return 0, false
		}
		castInfo := func(a Canon) (kop.ColorMask, int, bool) {
			if a.Kind() != ActCast {
				return 0, 0, false
			}
			o, ok := c.G.GorgeOption(a)
			if !ok {
				return 0, 0, false
			}
			ob := g.Obj(o.Obj)
			if ob == nil || ob.Face() == nil {
				return 0, 0, false
			}
			i, ok := set.Lookup(ob.Face().Name)
			if !ok {
				return 0, 0, false
			}
			d := set.Def(i)
			return d.Colors, int(d.Cost.Generic), true
		}
		buf = buf[:0]
		acts, readyActs, lifeCasts, casts = acts[:0], readyActs[:0], lifeCasts[:0], casts[:0]
		for _, a := range c.D.Options {
			if k, ok := actKind(a); ok {
				acts = append(acts, a)
				if kembnumX(g, dp, k) >= 1 {
					readyActs = append(readyActs, a)
				}
				continue
			}
			if a.Kind() == ActCast {
				casts = append(casts, a)
				if _, _, ok := castInfo(a); ok {
					o, _ := c.G.GorgeOption(a)
					if ob := g.Obj(o.Obj); ob != nil && ob.Face() != nil {
						switch ob.Face().Name {
						case "Char", "Flame Rift", "Sign in Blood", "Revitalize", "Renewed Faith":
							lifeCasts = append(lifeCasts, a)
						}
					}
				}
			}
		}
		pick := func(list []Canon) Canon {
			if len(list) == 1 {
				return list[0]
			}
			d := *c.D
			d.Options = append([]Canon(nil), list...)
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		var a Canon
		switch {
		case len(readyActs) > 0:
			a = pick(readyActs)
		case len(lifeCasts) > 0:
			a = pick(lifeCasts)
		case len(acts) > 0:
			a = pick(acts)
		case len(casts) > 0:
			a = pick(casts)
		default:
			a = rnd.Pick(c)
		}
		if k, ok := actKind(a); ok {
			st.activations++
			act[dp] = kembnumAct{turn: turn, kind: k, x: kembnumX(g, dp, k)}
		} else if cols, gen, ok := castInfo(a); ok {
			st.casts++
			if r := &act[dp]; r.kind != 0 && r.turn == turn && r.x >= 1 && gen > 0 && cols&kembnumColors(r.kind) != 0 {
				st.discounted++
			}
		}
		return a
	})
}

// runEmbnum plays the R/B-vs-W/U matchup, seats swapped, seeds 1..seeds.
func runEmbnum(t *testing.T, seeds int, mutate bool) kembnumStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("embnum")
	if err != nil {
		t.Fatal(err)
	}
	var st kembnumStats
	old := dirac.DebugEmbNumLive
	dirac.DebugEmbNumLive = mutate
	defer func() { dirac.DebugEmbNumLive = old }()
	for mi, md := range [][2][]cardset.Entry{kembnumDecks, {kembnumDecks[1], kembnumDecks[0]}} {
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
			m, err := NewMatchup(reg, [2]string{"kembnum-a", "kembnum-b"}, decks, 1)
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
			res := LockstepLed(m, seed, g, k, kembnumPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			st.games++
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				st.stalls++
				if !mutate {
					t.Errorf("matchup %d seed %d: kernel stall %q", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				st.divs++
				if st.firstDiv == 0 {
					st.firstDiv = seed
				}
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
						mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return st
}

// TestEmbnumLockstep: 256 seeds of the R/B-vs-W/U matchup. The deck must
// reach both carriers at least five times each and every decision must agree.
func TestEmbnumLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runEmbnum(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, activations %d, discounted casts %d, activations outgrown by live X %d",
		st.games, st.divs, st.stalls, st.activations, st.discounted, st.grew)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.discounted < 5 {
		t.Errorf("only %d discounted casts after an activation with frozen X >= 1 (want >= 5)", st.discounted)
	}
	if st.grew < 5 {
		t.Errorf("only %d activations outgrown by the live X in the same turn (want >= 5)", st.grew)
	}
}

// TestEmbnumMutation: DebugEmbNumLive re-evaluates the chosen number from
// the live state at read time. The two reads must disagree somewhere within
// 64 seeds; firstDiv is the seed.
func TestEmbnumMutation(t *testing.T) {
	st := runEmbnum(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("live re-read ran %d games with no divergence", st.games)
	}
	t.Logf("the live re-read diverged in %d of %d games (first seed %d)", st.divs, st.games, st.firstDiv)
}
