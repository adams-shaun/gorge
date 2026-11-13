package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: DealDamage `Defined$ Opponent |
// DamageSource$ <trigger object>` (Champion of the Path: an Elemental's
// power; Pyrotechnic Performer: a turned-face-up creature's power; Ingris
// Stingerquill: 1 from an attacker). Vampire Nighthawk and Child of Night (lifelink; the 2/1 dies to a Shock) and Plague
// Stinger (infect) attack under Ingris, so the source's keywords matter;
// Shock, cast in response to a trigger, makes the source leave first.
var kdmgsrcplDecks = [2][]cardset.Entry{
	{
		{Name: "Champion of the Path", Count: 4}, {Name: "Pyrotechnic Performer", Count: 3}, {Name: "Ingris Stingerquill", Count: 3},
		{Name: "Child of Night", Count: 3}, {Name: "Vampire Nighthawk", Count: 3},
		{Name: "Plague Stinger", Count: 2}, {Name: "Shock", Count: 4}, {Name: "Priest of Urabrask", Count: 3},
		{Name: "Mountain", Count: 9}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Pyrotechnic Performer", Count: 4}, {Name: "Champion of the Path", Count: 3}, {Name: "Ashmouth Hound", Count: 4},
		{Name: "Kolaghan Stormsinger", Count: 4}, {Name: "Shock", Count: 5}, {Name: "Priest of Urabrask", Count: 3},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Mountain", Count: 16},
	},
}

// kdmgsrcplCounts are what gorge's event log showed.
type kdmgsrcplCounts struct {
	champion, pyro, ingris, gone, goneLL int
}

// kdmgsrcplScan counts the carriers' trigger resolutions; gone counts those
// whose trigger object (the TriggerPush's Remembered ids: the entering,
// turned-up or attacking creature, the damage source) left the battlefield
// while the trigger waited on the stack: the last-known-information path.
// goneLL counts the gone ones whose object had lifelink (a lifelink source's
// LKI keywords and controller decide who gains life). The stack is replayed
// from the log: a TriggerPush or a PutOnStack pushes, a Resolve pops.
func kdmgsrcplScan(g *GorgeSide) kdmgsrcplCounts {
	var c kdmgsrcplCounts
	if g == nil {
		return c
	}
	type frame struct {
		src  string        // the carrier whose trigger this is ("" otherwise)
		ids  []state.ObjID // the trigger objects
		gone bool          // one of them left the battlefield while it waited
		ll   bool          // ... and it had lifelink
	}
	var stack []frame
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.MoveZone:
			if ev.From != state.ZBattlefield {
				break
			}
			for i := range stack {
				for _, id := range stack[i].ids {
					if id == ev.Obj {
						stack[i].gone = true
						stack[i].ll = stack[i].ll || chName(g.E.G, ev.Obj) == "Child of Night" || chName(g.E.G, ev.Obj) == "Vampire Nighthawk"
					}
				}
			}
		case events.TriggerPush:
			stack = append(stack, frame{src: chName(g.E.G, ev.Obj), ids: ev.IDs})
		case events.PutOnStack:
			stack = append(stack, frame{})
		case events.Resolve:
			if len(stack) == 0 {
				break
			}
			f := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			switch f.src {
			case "Champion of the Path":
				c.champion++
			case "Pyrotechnic Performer":
				c.pyro++
			case "Ingris Stingerquill":
				c.ingris++
			default:
				continue
			}
			if f.gone {
				c.gone++
				if f.ll {
					c.goneLL++
				}
			}
		}
	}
	return c
}

// kdmgsrcplPolicy takes a turn-face-up special action whenever offered
// (Pyrotechnic Performer's trigger needs one; both it and a face-down cast are
// paid from floating mana only, which a Priest of Urabrask supplies), else a
// face-down cast, else a Priest, else
// castFirstPolicy's casts and activations, else random.
func kdmgsrcplPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var up, down, priest, shock, buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		up, down, priest, shock, buf = up[:0], down[:0], priest[:0], shock[:0], buf[:0]
		for _, a := range c.D.Options {
			switch k := a.Kind(); {
			case k == ActSpecial && a.Mode()&15 == kfacedownTurnUp:
				up = append(up, a)
			case k == ActCastFace:
				down = append(down, a)
			case k == ActCast && optionFaceName(c, a) == "Shock" && c.G != nil && len(c.G.E.G.Stack) > 0:
				shock = append(shock, a) // in response: the trigger's source may die before it resolves
			case k == ActCast && optionFaceName(c, a) == "Priest of Urabrask":
				priest = append(priest, a)
				buf = append(buf, a)
			case k == ActCast && optionFaceName(c, a) == "Shock":
				// held for a response; with an empty stack it stays in hand
			case k == ActCast || k == ActActivate || k == ActPlayLand:
				buf = append(buf, a)
			}
		}
		pool := buf
		switch {
		case len(shock) > 0:
			pool = shock
		case len(up) > 0:
			pool = up
		case len(down) > 0:
			pool = down
		case len(priest) > 0:
			pool = priest
		}
		if len(pool) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = pool
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// kdmgsrcplRun plays seeds 1..seeds (decks swapped on odd seeds) and returns
// the summed counts, the first divergence ("" if none) and the kernel stalls.
func kdmgsrcplRun(t *testing.T, seeds uint64) (tot kdmgsrcplCounts, games int, div string, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("dmgsrcpl")
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		ents := kdmgsrcplDecks
		if seed%2 == 1 {
			ents[0], ents[1] = ents[1], ents[0]
		}
		var decks [2][]*cards.Card
		for s, d := range ents {
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
		tbl, _, _, err := set.Matchup(ents[0], ents[1])
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%4 < 2}
		m, err := NewMatchup(reg, [2]string{"kdmgsrcpl-a", "kdmgsrcpl-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		pol := kdmgsrcplPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		wrapped := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return pol.Pick(c)
		})
		res := Lockstep(m, seed, k, wrapped, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		c := kdmgsrcplScan(gs)
		tot.champion += c.champion
		tot.pyro += c.pyro
		tot.ingris += c.ingris
		tot.gone += c.gone
		tot.goneLL += c.goneLL
		if d := res.Div; d != nil {
			lo := max(d.At-8, 0)
			t.Logf("seed %d last actions %v\ndiff %v %s", seed, actionStrings(res.Actions[lo:d.At]), d.Diff, d.Detail)
			return tot, games, d.What + " " + d.Field + " seed " + strconv.FormatUint(seed, 10), stalls
		}
	}
	return tot, games, "", stalls
}

func TestDmgsrcplLockstep(t *testing.T) {
	t.Parallel()
	tot, games, div, stalls := kdmgsrcplRun(t, 192)
	t.Logf("%d games; Champion triggers %d; Pyrotechnic triggers %d; Ingris triggers %d; trigger object gone at resolution %d (lifelink %d); stalls %v",
		games, tot.champion, tot.pyro, tot.ingris, tot.gone, tot.goneLL, stalls)
	if div != "" {
		t.Fatalf("diverged: %s", div)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
	if tot.gone < 3 || tot.goneLL < 3 {
		t.Errorf("trigger object gone at resolution %d times (lifelink %d), want at least 3 each", tot.gone, tot.goneLL)
	}
	if tot.champion < 3 || tot.pyro < 3 || tot.ingris < 3 {
		t.Errorf("Champion %d, Pyrotechnic %d, Ingris %d triggers, want at least 3 each", tot.champion, tot.pyro, tot.ingris)
	}
}

// TestDmgsrcplMutation: with the player mode's lifelink gain dropped the lockstep must diverge within 128 seeds.
func TestDmgsrcplMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugDmgSrcPlNoLifelink = true
	defer func() { dirac.DebugDmgSrcPlNoLifelink = false }()
	_, games, div, _ := kdmgsrcplRun(t, 128)
	if div == "" {
		t.Fatalf("no divergence in %d games with DebugDmgSrcPlNoLifelink", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, div)
}

// TestKdmgsrcplMutationLKI: a source that left before the trigger resolved is
// read from its last known information; dropping its keywords or flipping its
// controller must each make the lockstep diverge.
func TestDmgsrcplMutationLKIKw(t *testing.T)   { kdmgsrcplLKIMutation(t, 1) }
func TestDmgsrcplMutationLKICtrl(t *testing.T) { kdmgsrcplLKIMutation(t, 2) }

func kdmgsrcplLKIMutation(t *testing.T, mode uint8) {
	if isolate(t) {
		return
	}
	dirac.DebugDmgSrcPlLKI = mode
	defer func() { dirac.DebugDmgSrcPlLKI = 0 }()
	_, games, div, _ := kdmgsrcplRun(t, 192)
	if div == "" {
		t.Fatalf("no divergence in %d games with DebugDmgSrcPlLKI=%d", games, mode)
	}
	t.Logf("mutation %d diverged after %d games: %s", mode, games, div)
}
