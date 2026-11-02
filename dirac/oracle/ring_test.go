package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/spellbench/payexec"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The ring lockstep: Sol Ring and Ring of the Lucii ({T}: add {C}{C}) beside
// cheap spells. The planner taps lands before a Ring, so a Ring meets a one-mana
// remainder once the lands run out, and the extra colourless floats (CR 106.4).
// A land played after the cast is the untapped land beside it.

var kringDecks = [2][]cardset.Entry{
	{{Name: "Sol Ring", Count: 8}, {Name: "Ring of the Lucii", Count: 2}, {Name: "Coral Merfolk", Count: 14}, {Name: "Island", Count: 16}},
	{{Name: "Sol Ring", Count: 8}, {Name: "Ring of the Lucii", Count: 2}, {Name: "Walking Corpse", Count: 14}, {Name: "Swamp", Count: 16}},
}

func TestRingLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ring")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kringDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(kringDecks[0], kringDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	bad := 0
	for seed := uint64(1); seed <= 1500; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kring-a", "kring-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		hp := HeuristicPolicy(seed)
		var pol Policy = PolicyFunc(func(c *PickCtx) Canon { kringCount(c); return hp.Pick(c) })
		if seed%4 != 0 {
			pol = kringPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 14})
		CloseSide(k)
		if d := res.Div; d != nil {
			bad++
			if bad <= 3 {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d diverged of 1500, casts offered over floating mana %d", bad, kringFloat)
	if kringFloat < 500 {
		t.Errorf("only %d casts offered over floating mana", kringFloat)
	}
}

// kringCount counts the priority decisions of the mechanic itself: a player
// holds one floating mana (a Ring of the Lucii tap left it over), a two-mana
// creature in hand and an untapped basic land, and gorge offers the cast: its
// planner starts from the floating artifact mana (CR 106.4) and taps the land
// for the rest, so the one mana alone not paying the spell withholds nothing.
func kringCount(c *PickCtx) {
	if c.G == nil || c.D.Kind != DPriority {
		return
	}
	offered := false
	for _, a := range c.D.Options {
		offered = offered || a.Kind() == ActCast
	}
	if !offered {
		return
	}
	if !kringFloats(c) {
		return
	}
	p := c.D.Player
	spell, land := false, false
	for i := range c.G.E.G.Objs {
		ob := &c.G.E.G.Objs[i]
		if ob.Face() == nil || int(ob.Controller) != int(p) {
			continue
		}
		switch n := ob.Face().Name; {
		case ob.Zone == state.ZHand && (n == "Coral Merfolk" || n == "Walking Corpse"):
			spell = true
		case ob.Zone == state.ZBattlefield && (n == "Island" || n == "Swamp") && !ob.Tapped:
			land = true
		}
	}
	if spell && land {
		kringFloat++
	}
}

// kringFloats: the player to decide holds floating mana.
func kringFloats(c *PickCtx) bool {
	if c.G == nil {
		return false
	}
	for _, n := range payexec.PoolOf(c.G.E, state.PlayerID(c.D.Player)) {
		if n != 0 {
			return true
		}
	}
	return false
}

// kringPolicy casts or activates whenever it can, then plays a land, else
// random.
var kringFloat int

func kringPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		kringCount(c)
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			for _, a := range c.D.Options {
				if a.Kind() == ActPlayLand {
					return a
				}
			}
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}
