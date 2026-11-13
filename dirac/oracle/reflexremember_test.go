package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: Novel Nunchaku (Attach
// RememberAttached$ gating the reflexive trigger) and Curse of the Werefox
// (its Monster Role an Aura); both reflexive triggers capture the parent's
// target (RememberObjects$ Targeted) for the body's Fight Defined$
// DelayTriggerRemembered. Deck A is a green-blue Aura deck whose own
// Unsummon, cast while its reflexive trigger waits, can bounce the captured
// creature; deck B red-blue-green with Shock and Unsummon. Deck A's Graceful
// Takedown has two Pumps RememberObjects$ ThisTargetedCard (the root's
// enchanted creatures, the sub link's "up to one other"), whose union deals
// damage (EachDamage DefinedDamagers$ Remembered).

var kremw4Decks = [2][]cardset.Entry{
	{
		{Name: "Novel Nunchaku", Count: 4}, {Name: "Curse of the Werefox", Count: 5},
		{Name: "Rancor", Count: 3}, {Name: "Llanowar Elves", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Graceful Takedown", Count: 3}, {Name: "Unsummon", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 5},
	},
	{
		{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Novel Nunchaku", Count: 2}, {Name: "Shock", Count: 4}, {Name: "Unsummon", Count: 5},
		{Name: "Giant Growth", Count: 1}, {Name: "Mountain", Count: 7}, {Name: "Island", Count: 5}, {Name: "Forest", Count: 5},
	},
}

// kremw4Counts: what one game's gorge event log shows of the reflexive fights.
type kremw4Counts struct {
	games         int
	fights        int // Nunchaku / Werefox reflexive fights that dealt damage
	reflexGone    int // reflexive fights whose captured creature had left the battlefield
	reflexResolve int
	takedowns     int // Graceful Takedown resolutions
	takedown2     int // ... dealing damage from at least two damagers
}

var kremw4Fighters = map[string]bool{"Novel Nunchaku": true, "Curse of the Werefox": true}

// kremw4Scan reads gorge's event log: each Nunchaku / Werefox reflexive Fight resolution, split by whether its captured creature (the
// DelayedPush's remembered IDs) is still on the battlefield.
func kremw4Scan(g *GorgeSide, c *kremw4Counts) {
	if g == nil {
		return
	}
	gm := g.E.G
	evs := g.E.L.Events
	zone := map[state.ObjID]state.Zone{}
	captured := map[state.ObjID]state.ObjID{} // source -> captured creature (the latest push)
	for i, ev := range evs {
		switch ev.Kind {
		case events.MoveZone:
			zone[ev.Obj] = ev.To
			continue
		case events.DelayedPush:
			if ev.Counter == "TrigFight" && kremw4Fighters[firstFaceName(gm, ev.Obj)] && len(ev.IDs) == 1 {
				captured[ev.Obj] = ev.IDs[0]
			}
			continue
		case events.Resolve:
		default:
			continue
		}
		dmg := 0
		for j := i + 1; j < len(evs) && evs[j].Kind != events.Priority && evs[j].Kind != events.Resolve; j++ {
			if evs[j].Kind == events.Damage && evs[j].Amount > 0 {
				dmg++
			}
		}
		o := gm.Obj(ev.Obj)
		if o != nil && o.Ability == nil && firstFaceName(gm, ev.Obj) == "Graceful Takedown" {
			// one Damage event per damager (the single target)
			c.takedowns++
			if dmg >= 2 {
				c.takedown2++
			}
			continue
		}
		if o == nil || o.Ability == nil || o.Ability.API != "Fight" || !kremw4Fighters[sourceCardName(gm, ev.Obj)] {
			continue
		}
		cap, ok := captured[o.Source]
		if !ok {
			continue
		}
		delete(captured, o.Source)
		c.reflexResolve++
		switch {
		case zone[cap] != state.ZBattlefield:
			c.reflexGone++
		case dmg > 0:
			c.fights++
		}
	}
}

// kremw4Policy plays deck A cast-first, preferring a cast to a mana
// activation. Deck B answers a waiting ability: on deck A's turn it acts
// (cast-first) only while the top of the stack is an ability (a reflexive
// fight among them), else passes; on every second turn of its own it only
// plays a land, keeping its mana open for deck A's next turn.
func kremw4Policy(seed uint64) Policy {
	rnd, cf := RandomPolicy(seed), castFirstPolicy(seed)
	var buf []Canon
	pass := func(c *PickCtx) (Canon, bool) {
		for _, a := range c.D.Options {
			if a.Kind() == ActPass {
				return a, true
			}
		}
		return 0, false
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if g := c.G; g != nil && c.D.Player == 1 {
			gm := g.E.G
			if int(gm.Active) != 1 {
				if n := len(gm.Stack); n == 0 || gm.Obj(gm.Stack[n-1]) == nil || gm.Obj(gm.Stack[n-1]).Ability == nil {
					if a, ok := pass(c); ok {
						return a
					}
				}
				return cf.Pick(c)
			}
			if gm.Turn%4 >= 2 {
				buf = buf[:0]
				for _, a := range c.D.Options {
					if a.Kind() == ActPlayLand {
						buf = append(buf, a)
					}
				}
				if len(buf) > 0 {
					return buf[0]
				}
				if a, ok := pass(c); ok {
					return a
				}
			}
			return cf.Pick(c)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActCastFace {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return cf.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func kremw4Run(t *testing.T, seeds uint64, mutant bool) (c kremw4Counts, diverged bool, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("reflexremember")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kremw4Decks {
		for _, e := range d {
			cd, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], cd)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kremw4Decks[0], kremw4Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"kremw4-a", "kremw4-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := kremw4Policy(seed)
		if seed%8 == 0 {
			inner = RandomPolicy(seed)
		}
		pol := PolicyFunc(func(pc *PickCtx) Canon {
			if pc.G != nil {
				gs = pc.G
			}
			return inner.Pick(pc)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 14})
		CloseSide(k)
		c.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		kremw4Scan(gs, &c)
		if d := res.Div; d != nil {
			diverged = true
			if mutant {
				t.Logf("mutant: seed %d diverged at decision %d: %s %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.RefOptions, d.KerOptions)
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestReflexiveRememberTargetLockstep(t *testing.T) {
	t.Parallel()
	c, _, stalls := kremw4Run(t, 400, false)
	t.Logf("%d games: %d reflexive fight resolutions: %d fought, %d skipped (captured creature gone); %d Graceful Takedowns, %d with >= 2 damagers; kernel stalls %v",
		c.games, c.reflexResolve, c.fights, c.reflexGone, c.takedowns, c.takedown2, stalls)
	if c.fights < 10 {
		t.Errorf("%d Nunchaku / Werefox fights, want at least 10", c.fights)
	}
	if c.reflexGone < 3 {
		t.Errorf("%d reflexive fights skipped with the captured creature gone, want at least 3", c.reflexGone)
	}
	if c.takedown2 < 1 {
		t.Errorf("%d Graceful Takedowns with >= 2 damagers, want at least 1", c.takedown2)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}

// TestReflexiveRememberTargetMutation: with the reflexive capture dropped no
// Nunchaku / Werefox fight happens in the kernel, so the lockstep must
// diverge.
func TestReflexiveRememberTargetMutation(t *testing.T) {
	dirac.DebugNoReflexCapture = true
	defer func() { dirac.DebugNoReflexCapture = false }()
	if _, diverged, _ := kremw4Run(t, 128, true); !diverged {
		t.Fatal("no divergence in 128 seeds with the reflexive capture dropped")
	}
}
