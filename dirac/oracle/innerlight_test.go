package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Inner Light lockstep: chapter II's Effect remembers its target creature
// and, for the turn, draws as many cards as that creature deals in combat
// damage to a player (TriggerCount$DamageAmount). The kernel's EOT emblem
// remembers the target (kop.Emblem.Capture, State.g.embN) and its DamageDone
// hook carries the damage dealt (dirac/trig_effectuntileot.go). Deck A plays
// the Saga beside green and white creatures (chapter I returns one, a double
// striker deals in two events); both decks attack, so the target is
// sometimes blocked, sometimes unblocked for more than one damage, and
// sometimes an opponent's creature (its damage draws nothing for the
// Saga's controller).

var kinnerlightDecks = [2][]cardset.Entry{
	{
		{Name: "The Inner Light", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Giant Spider", Count: 3},
		{Name: "Fencing Ace", Count: 3}, {Name: "Wind Drake", Count: 2},
		{Name: "Forest", Count: 9}, {Name: "Plains", Count: 9},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Gray Ogre", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 11}, {Name: "Swamp", Count: 11},
	},
}

// kinnerlightPolicy casts and plays lands whenever it can (the Saga's
// chapter II target is then picked at random among the creatures), attacks
// and blocks with a random non-empty subset, else picks at random.
func kinnerlightPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		var done Canon
		haveDone := false
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActAttack, ActBlock:
				buf = append(buf, a)
			case ActDone:
				done, haveDone = a, true
			}
		}
		if len(buf) > 0 {
			if haveDone && len(buf) > 1 {
				buf = append(buf, done)
			}
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActPlayLand {
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

func runInnerLight(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st dirac.EffEOTTrigStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("innerlight")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugInnerLightNoAmount
	dirac.DebugInnerLightNoAmount = mutate
	dirac.DebugEffEOTTrigStats = &st
	defer func() {
		dirac.DebugInnerLightNoAmount = old
		dirac.DebugEffEOTTrigStats = nil
	}()
	var decks [2][]*cards.Card
	for s, d := range kinnerlightDecks {
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
	tbl, _, _, err := set.Matchup(kinnerlightDecks[0], kinnerlightDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kinnerlight-a", "kinnerlight-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kinnerlightPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Logf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, st
}

func TestTheInnerLightLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runInnerLight(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, remembered-creature firings %d (%d for more than 1 damage), damage events passed over %d", games, divs, stalls, st.DmgFires, st.DmgMulti, st.DmgCapSkips)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	// the precondition: the target connected for more than 1 damage, and
	// other creatures dealt combat damage without drawing
	if st.DmgMulti < 5 {
		t.Errorf("only %d firings for more than 1 damage (want at least 5)", st.DmgMulti)
	}
	if st.DmgCapSkips < 5 {
		t.Errorf("only %d damage events passed over by the remembered target (want at least 5)", st.DmgCapSkips)
	}
}

// TestTheInnerLightMutation queues the emblem trigger with amount 0 again:
// the games must diverge (the draw of that many cards is dropped).
func TestTheInnerLightMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, st := runInnerLight(t, 64, true)
	if st.DmgMulti == 0 {
		t.Fatalf("no emblem firing for more than 1 damage in %d games: the mutation is unreached", games)
	}
	if divs == 0 {
		t.Fatalf("mutation zeroed the damage amount in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
