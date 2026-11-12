package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The until-end-of-turn combat Effect trigger lockstep:
// Mistway Spy turned face up ("until end of turn, whenever a creature you
// control deals combat damage to a player, investigate") and Dalkovan
// Encampment's activation ("whenever you attack this turn, create two
// tapped and attacking Warriors"), each the kernel's EOT emblem fired by
// the combat damage and attack declaration hooks
// (dirac/trig_effectuntileot.go). Bident of Thassa's and
// Reconnaissance Mission's printed combat-damage triggers share the event
// with the Mistway emblem (both pending together: the trigger_order ask),
// the path the hook after the battlefield listener walk serves. Deck B plays Spies too, and
// both decks attack. Known gap: the emblem is only ever held on its owner's
// own turn here (the policy turns Mistway up from the Tiger's floating
// mana, main phase), so "the opponent's creatures hitting the owner do not
// fire the emblem" is covered only by the mutation (the filter matched for
// the owner's opponent), not by an opponent-turn turn-up.

var keffeottrigDecks = [2][]cardset.Entry{
	{
		{Name: "Mistway Spy", Count: 4}, {Name: "Iridescent Tiger", Count: 4}, {Name: "Dalkovan Encampment", Count: 3},
		{Name: "Bident of Thassa", Count: 1}, {Name: "Reconnaissance Mission", Count: 5}, {Name: "Wind Drake", Count: 2},
		{Name: "Highspire Bell-Ringer", Count: 3}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Mistway Spy", Count: 4}, {Name: "Iridescent Tiger", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Reconnaissance Mission", Count: 4}, {Name: "Giant Spider", Count: 2},
		{Name: "Walking Corpse", Count: 2}, {Name: "Forest", Count: 3}, {Name: "Mountain", Count: 5},
		{Name: "Swamp", Count: 2}, {Name: "Island", Count: 7},
	},
}

// keffeottrigPolicy turns a face-down card up and casts Mistway Spy face
// down whenever it can (both are priced from the floating pool alone:
// Iridescent Tiger's enters trigger floats WUBRG; the {3} cast spends its
// W, U and B, so the {1}{U} turn-up waits for a later Tiger), casts the
// Tiger whenever it can, activates Dalkovan's Effect, attacks and blocks with a random subset
// (at least one creature) whenever it can, else casts or plays randomly.
func keffeottrigPolicy(seed uint64, st *keffeottrigCounts) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		var up, down, dalk, tiger, listener, done Canon
		haveUp, haveDown, haveDalk, haveTiger, haveListener, haveDone := false, false, false, false, false, false
		buf = buf[:0]
		for _, a := range c.D.Options {
			n := optionFaceName(c, a)
			switch a.Kind() {
			case ActSpecial:
				if a.Mode()&15 == kfacedownTurnUp {
					up, haveUp = a, true
				}
			case ActCastFace:
				if n == "Mistway Spy" {
					down, haveDown = a, true
				}
			case ActCast:
				switch n {
				case "Iridescent Tiger":
					tiger, haveTiger = a, true
				case "Bident of Thassa", "Reconnaissance Mission":
					listener, haveListener = a, true
				}
			case ActActivate:
				if a.Mode() == 1 && n == "Dalkovan Encampment" {
					dalk, haveDalk = a, true
				}
			case ActAttack, ActBlock:
				buf = append(buf, a)
			case ActDone:
				done, haveDone = a, true
			}
		}
		switch {
		case haveUp:
			st.ups++
			return up
		case haveDown:
			st.downs++
			return down
		case haveDalk:
			return dalk
		case haveTiger:
			return tiger
		case haveListener:
			return listener // a printed combat damage listener beside the emblem
		case len(buf) > 0:
			if haveDone && len(buf) > 1 {
				buf = append(buf, done) // stop early: some creatures stay back
			}
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCastFace, ActPlayLand:
				buf = append(buf, a)
			case ActCast:
				if optionFaceName(c, a) != "Mistway Spy" {
					buf = append(buf, a)
				}
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

// keffeottrigCounts counts the policy's Mistway Spy plays.
type keffeottrigCounts struct{ downs, ups int }

func runEffeottrig(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st dirac.EffEOTTrigStats, pc keffeottrigCounts) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("effeottrig")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugEffEOTTrigOppOwner
	dirac.DebugEffEOTTrigOppOwner = mutate
	dirac.DebugEffEOTTrigStats = &st
	defer func() {
		dirac.DebugEffEOTTrigOppOwner = old
		dirac.DebugEffEOTTrigStats = nil
	}()
	var decks [2][]*cards.Card
	for s, d := range keffeottrigDecks {
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
	tbl, _, _, err := set.Matchup(keffeottrigDecks[0], keffeottrigDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"keffeottrig-a", "keffeottrig-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, keffeottrigPolicy(seed, &pc), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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
	return games, divs, stalls, st, pc
}

func TestEffeottrigLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st, pc := runEffeottrig(t, 512, false)
	t.Logf("%d games, %d divergences, %d stalls, Mistway face-down casts %d, turn-ups %d, Mistway emblem firings %d (%d beside a battlefield listener, %d beside another pending trigger of the owner's), Dalkovan emblem firings %d",
		games, divs, stalls, pc.downs, pc.ups, st.DmgFires, st.DmgWithListener, st.DmgBeside, st.AtkFires)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	if st.DmgFires < 30 {
		t.Errorf("only %d Mistway emblem firings (need 30)", st.DmgFires)
	}
	if st.DmgWithListener < 30 {
		t.Errorf("only %d Mistway emblem firings beside a battlefield DamageDone listener (need 30)", st.DmgWithListener)
	}
	if st.DmgBeside < 30 {
		t.Errorf("only %d Mistway emblem firings beside another pending trigger of the owner's (need 30)", st.DmgBeside)
	}
	if st.AtkFires < 30 {
		t.Errorf("only %d Dalkovan emblem firings (need 30)", st.AtkFires)
	}
}

// TestEffeottrigMutation matches the emblem's source filter for the
// owner's opponent (dirac.DebugEffEOTTrigOppOwner): the owner's creatures
// then never fire the Mistway emblem, so the games must diverge.
func TestEffeottrigMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _, _ := runEffeottrig(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation matched the emblem filter for the owner's opponent in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
