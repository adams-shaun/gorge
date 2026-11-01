package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The UntapOtherPlayer lockstep: Bender's Waterskin and
// Thousand Moons Infantry untap during each other player's untap step, after
// the active player's untap action (gorge finishUntapStep). The policy
// attacks, taps for mana and casts instants whenever offered, so the
// Waterskin is tapped on its controller's turn and again on the opponent's.

var kuntapoDecks = [2][]cardset.Entry{
	{
		{Name: "Thousand Moons Infantry", Count: 6}, {Name: "Bender's Waterskin", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Divination", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Plains", Count: 9}, {Name: "Island", Count: 9},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 6}, {Name: "Glory Seeker", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

var kuntapoCarriers = map[string]bool{"Bender's Waterskin": true, "Thousand Moons Infantry": true}

type kuntapoStats struct {
	games, divs, stalls int
	foreign             int // a carrier untapped during the other player's untap step
}

// kuntapoPolicy attacks, activates and casts whenever it can, else random.
func kuntapoPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActAttack || k == ActActivate || k == ActCast {
				buf = append(buf, a)
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

// runUntapo plays the constructed matchup over seeds 1..seeds; off sets the
// DebugUntapOtherOff mutation.
func runUntapo(t *testing.T, seeds int, off bool) kuntapoStats {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("untapo")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugUntapOtherOff
	dirac.DebugUntapOtherOff = off
	defer func() { dirac.DebugUntapOtherOff = old }()
	var decks [2][]*cards.Card
	for s, d := range kuntapoDecks {
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
	tbl, _, _, err := set.Matchup(kuntapoDecks[0], kuntapoDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var st kuntapoStats
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kuntapo-a", "kuntapo-b"}, decks, 1)
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
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				if ev := evs[seen]; ev.Kind == events.Untap {
					if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && kuntapoCarriers[o.Face().Name] && o.Controller != ref.E.G.Active {
						st.foreign++
					}
				}
			}
			return nil
		}
		res := LockstepLed(m, seed, g, k, kuntapoPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: check})
		CloseSide(k)
		st.games++
		if d := res.Div; d != nil {
			st.divs++
			if !off {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			st.stalls++
			if !off {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
	}
	return st
}

func TestUntapoLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := runUntapo(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d foreign untaps of a carrier", st.games, st.divs, st.stalls, st.foreign)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.foreign < 40 {
		t.Errorf("only %d foreign untaps of a carrier, want >= 40", st.foreign)
	}
}

// TestUntapoMutation turns the foreign scan off: a tapped carrier then stays
// tapped through the opponent's untap step, which gorge untaps, so the games
// must diverge.
func TestUntapoMutation(t *testing.T) {
	st := runUntapo(t, 64, true)
	if st.divs == 0 {
		t.Fatalf("mutation left the foreign untap off in %d games with no divergence", st.games)
	}
	t.Logf("mutation diverged in %d of %d games", st.divs, st.games)
}
