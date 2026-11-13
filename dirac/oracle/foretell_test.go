package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Foretell lockstep (CR 702.143a): constructed matchups over the
// fixture pool dirac/cardset/pools/fixtures/foretell. The decks carry every
// arm:
//
//   - the {2} special action from the hand on the foretelling player's own
//     turn, at any priority including instant timing and with a stack
//     (gorge's "foretell" cast option, canonical cast mode 12);
//   - the later cast from exile for the foretell cost (gorge's
//     "foretell_cast", canonical CastModeOther) on a later turn, with the
//     card's own timing: an instant at instant speed, a sorcery or creature
//     only in its owner's main phase with an empty stack;
//   - no foretell cast on the foretelling turn, and none for a card exiled
//     by anything but the action.
//
// The counters come from the policy's picks, so the test fails when the
// kernel stops offering any arm; the mutations relax one rule each and must
// diverge, which proves the counters are not vacuous.
var foretellDecks = []fixtureDecks{{
	{ // blue-red: instants, sorceries and creatures that foretell, and rituals to float the mana for the actions
		{Name: "Behold the Multiverse", Count: 3}, {Name: "Ravenform", Count: 3}, {Name: "Alrund's Epiphany", Count: 1},
		{Name: "Saw It Coming", Count: 2}, {Name: "Spectral Deluge", Count: 1}, {Name: "Surge of Brilliance", Count: 1},
		{Name: "Demon Bolt", Count: 3}, {Name: "Dual Strike", Count: 2}, {Name: "Impending Flux", Count: 2},
		{Name: "Frost Fair Lure Fish", Count: 1}, {Name: "Lightning Bolt", Count: 1},
		{Name: "Pyretic Ritual", Count: 4}, {Name: "Seething Song", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Mountain", Count: 8},
	},
	{ // white-black-green: removal and a trampler that foretell
		{Name: "Iron Verdict", Count: 3}, {Name: "Warhorn Blast", Count: 3}, {Name: "Tergrid's Shadow", Count: 2},
		{Name: "Return Upon the Tide", Count: 2}, {Name: "Battle Mammoth", Count: 3}, {Name: "Dark Ritual", Count: 4},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Savannah Lions", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Swamp", Count: 6}, {Name: "Forest", Count: 5},
	},
}, {
	{
		{Name: "Behold the Multiverse", Count: 4}, {Name: "Ravenform", Count: 2}, {Name: "Saw It Coming", Count: 2},
		{Name: "Surge of Brilliance", Count: 2}, {Name: "Demon Bolt", Count: 4}, {Name: "Impending Flux", Count: 2},
		{Name: "Frost Fair Lure Fish", Count: 1}, {Name: "Pyretic Ritual", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Behold the Multiverse", Count: 2}, {Name: "Ravenform", Count: 4}, {Name: "Alrund's Epiphany", Count: 2},
		{Name: "Saw It Coming", Count: 3}, {Name: "Spectral Deluge", Count: 2}, {Name: "Dual Strike", Count: 2},
		{Name: "Demon Bolt", Count: 2}, {Name: "Lightning Bolt", Count: 2}, {Name: "Seething Song", Count: 2},
		{Name: "Pyretic Ritual", Count: 3},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8},
	},
}, {
	{ // red: creatures, tokens and a wrath, with a colourless foretell cost of {0}
		{Name: "Pyretic Ritual", Count: 4}, {Name: "Seething Song", Count: 2}, {Name: "Dwarven Reinforcements", Count: 3},
		{Name: "Doomskar Titan", Count: 2}, {Name: "Demon Bolt", Count: 3}, {Name: "Impending Flux", Count: 2},
		{Name: "Dual Strike", Count: 2}, {Name: "Scorn Effigy", Count: 3}, {Name: "Delayed Blast Fireball", Count: 2},
		{Name: "Mountain", Count: 17},
	},
	{ // black: a flash creature, a discard spell, an instant that reads the creatures that died
		{Name: "Dark Ritual", Count: 4}, {Name: "Tergrid's Shadow", Count: 2}, {Name: "Return Upon the Tide", Count: 2},
		{Name: "Jarl of the Forsaken", Count: 3}, {Name: "Rise of the Dread Marn", Count: 3}, {Name: "Skull Raid", Count: 3},
		{Name: "Vengeful Reaper", Count: 3}, {Name: "Scorn Effigy", Count: 2}, {Name: "Hill Giant", Count: 1},
		{Name: "Swamp", Count: 17},
	},
}, {
	{ // red-white: foretell actions on cards whose later cast the floating red cannot pay
		{Name: "Pyretic Ritual", Count: 4}, {Name: "Doomskar", Count: 2}, {Name: "Doomskar Oracle", Count: 3},
		{Name: "Gods' Hall Guardian", Count: 2}, {Name: "Kaya's Onslaught", Count: 3}, {Name: "Shepherd of the Cosmos", Count: 2},
		{Name: "Delayed Blast Fireball", Count: 3}, {Name: "Hill Giant", Count: 4},
		{Name: "Plains", Count: 9}, {Name: "Mountain", Count: 8},
	},
	{ // green-blue-black
		{Name: "Dark Ritual", Count: 4}, {Name: "Mammoth Growth", Count: 3}, {Name: "Struggle for Skemfar", Count: 3},
		{Name: "Sarulf's Packmate", Count: 3}, {Name: "Augury Raven", Count: 3}, {Name: "Depart the Realm", Count: 3},
		{Name: "Grizzly Bears", Count: 3},
		{Name: "Forest", Count: 7}, {Name: "Swamp", Count: 5}, {Name: "Island", Count: 6},
	},
}, {
	{ // Delayed Blast Fireball cast from exile through a may-play permission reads wasCastFromExile too
		{Name: "Delayed Blast Fireball", Count: 5}, {Name: "Reckless Impulse", Count: 5}, {Name: "Dual Strike", Count: 2},
		{Name: "Demon Bolt", Count: 2}, {Name: "Impending Flux", Count: 2}, {Name: "Pyretic Ritual", Count: 3},
		{Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 17},
	},
	{
		{Name: "Behold the Multiverse", Count: 3}, {Name: "Ravenform", Count: 3}, {Name: "Alrund's Epiphany", Count: 1},
		{Name: "Saw It Coming", Count: 2}, {Name: "Spectral Deluge", Count: 1}, {Name: "Surge of Brilliance", Count: 1},
		{Name: "Demon Bolt", Count: 3}, {Name: "Dual Strike", Count: 2}, {Name: "Impending Flux", Count: 2},
		{Name: "Frost Fair Lure Fish", Count: 1}, {Name: "Lightning Bolt", Count: 1},
		{Name: "Pyretic Ritual", Count: 4}, {Name: "Seething Song", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Mountain", Count: 8},
	},
}, {
	{ // Ultimate Magic: Holy's damage prevention reads wasCastFromExile (cast from exile through a may-play permission)
		{Name: "Ultimate Magic: Holy", Count: 6}, {Name: "Reckless Impulse", Count: 5}, {Name: "Savannah Lions", Count: 5},
		{Name: "Pyretic Ritual", Count: 3}, {Name: "Plains", Count: 11}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Lightning Bolt", Count: 6}, {Name: "Demon Bolt", Count: 3}, {Name: "Hill Giant", Count: 6},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}}

// foretellRituals float the mana the foretell actions and the foretold casts
// need: gorge offers both from the real-pool walk only (its potential planner
// prices no foretell mode), and the lockstep never taps a source at priority,
// so only mana a resolved spell left floating pays them.
var foretellRituals = map[string]bool{"Pyretic Ritual": true, "Seething Song": true, "Dark Ritual": true}

var foretellWatch = map[string]bool{
	"Behold the Multiverse": true, "Ravenform": true, "Battle Mammoth": true, "Alrund's Epiphany": true,
	"Demon Bolt": true, "Dual Strike": true, "Frost Fair Lure Fish": true, "Impending Flux": true,
	"Iron Verdict": true, "Return Upon the Tide": true, "Saw It Coming": true, "Spectral Deluge": true,
	"Surge of Brilliance": true, "Tergrid's Shadow": true, "Warhorn Blast": true,
	"Augury Raven": true, "Depart the Realm": true, "Doomskar": true, "Doomskar Oracle": true, "Doomskar Titan": true,
	"Dwarven Reinforcements": true, "Gods' Hall Guardian": true, "Jarl of the Forsaken": true, "Kaya's Onslaught": true,
	"Mammoth Growth": true, "Rise of the Dread Marn": true, "Sarulf's Packmate": true, "Scorn Effigy": true,
	"Shepherd of the Cosmos": true, "Delayed Blast Fireball": true, "Ultimate Magic: Holy": true, "Skull Raid": true, "Struggle for Skemfar": true, "Vengeful Reaper": true,
}

// wasCastFromExileCards read Count$wasCastFromExile in a spell effect.
var wasCastFromExileCards = map[string]bool{"Delayed Blast Fireball": true, "Ultimate Magic: Holy": true}

// foretellTally counts the picks of each arm.
type foretellTally struct {
	games, divs, stalls int
	actions             int // foretell special actions taken
	casts               int // foretold cards cast from exile
	castBy              map[string]int
	exileCast           map[string]int // casts from exile of the cards whose effect reads wasCastFromExile, foretold or through a may-play permission
	instantAct          int            // actions taken with a spell on the stack or outside a main phase
	first               string
}

// foretellPolicy plays randomly, but takes a foretell action or a foretold
// cast the moment one is offered (three times in four), and any watched card's
// plain cast otherwise, counting the foretell picks.
func foretellPolicy(seed uint64, w *foretellTally) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x666f7265)) // "fore"
	foretellMode, _ := castModeCode("foretell")
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast {
					continue
				}
				if o := c.G.E.G.Obj(c.G.id.obj[a.Src()]); a.Src() < RefStack && o != nil && o.Zone == state.ZExile && wasCastFromExileCards[refCardName(c.G, a.Src())] && prng.IntN(4) != 0 {
					w.exileCast[refCardName(c.G, a.Src())]++
					return a // the arm that reads wasCastFromExile
				}
				if foretellRituals[refCardName(c.G, a.Src())] && a.Mode()&15 == 0 && len(c.G.E.G.Stack) == 0 && prng.IntN(3) != 0 {
					return a // float mana for the actions
				}
				m := a.Mode() & 15
				isAction := m == foretellMode && foretellWatch[refCardName(c.G, a.Src())]
				isCast := m == 15 && foretellWatch[refCardName(c.G, a.Src())]
				if !isAction && !isCast || prng.IntN(4) == 0 {
					continue
				}
				if isAction {
					w.actions++
					if g := c.G.E.G; len(g.Stack) > 0 || !g.Step.IsMain() {
						w.instantAct++
					}
				} else {
					w.casts++
					w.castBy[refCardName(c.G, a.Src())]++
				}
				return a
			}
		}
		return rnd.Pick(c)
	})
}

func runForetell(t *testing.T, seeds int, report bool, mutate func(*kop.Table)) *foretellTally {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("foretell")
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][2][]*cards.Card, len(foretellDecks))
	tbls := make([]*kop.Table, len(foretellDecks))
	for mi, md := range foretellDecks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		if mutate != nil {
			mutate(tbl)
		}
		tbls[mi] = tbl
	}
	w := &foretellTally{castBy: map[string]int{}, exileCast: map[string]int{}}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		mi := int(seed/2) % len(foretellDecks)
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"foretell-a", "foretell-b"}, decks[mi], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[mi], cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, foretellPolicy(seed, w), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		w.games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			w.stalls++
			if report {
				t.Errorf("seed %d: kernel capacity stall %q", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			w.divs++
			msg := strings.TrimSpace(d.What + " " + d.Field + " " + d.Detail)
			if w.first == "" {
				w.first = msg
			}
			if report {
				t.Errorf("seed %d: diverged at decision %d: %s %v\nref %v\nker %v", seed, d.At, msg, d.Diff, d.RefOptions, d.KerOptions)
			}
			if !report || w.divs >= 3 {
				break
			}
		}
	}
	return w
}

func TestForetellLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	w := runForetell(t, 400, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, %d foretell actions (%d at instant timing), %d foretold casts %v, casts from exile of the wasCastFromExile cards %v",
		w.games, w.divs, w.stalls, w.actions, w.instantAct, w.casts, w.castBy, w.exileCast)
	if w.actions < 100 {
		t.Errorf("only %d foretell actions", w.actions)
	}
	for _, n := range []string{"Demon Bolt", "Dual Strike", "Impending Flux", "Dwarven Reinforcements", "Doomskar Titan", "Scorn Effigy",
		"Jarl of the Forsaken", "Rise of the Dread Marn", "Skull Raid"} {
		if w.castBy[n] == 0 {
			t.Errorf("no foretold cast of %s (casts by card: %v)", n, w.castBy)
		}
	}
	for n := range wasCastFromExileCards {
		if w.exileCast[n] < 5 {
			t.Errorf("only %d casts of %s from exile", w.exileCast[n], n)
		}
	}
	if w.casts < 40 {
		t.Errorf("only %d foretold casts", w.casts)
	}
	if w.instantAct < 10 {
		t.Errorf("only %d foretell actions outside a main phase or over a stack", w.instantAct)
	}
}

// TestForetellMutation: relaxing the later-turn rule of the foretold cast,
// or making the action wait for sorcery timing, must each make the lockstep
// diverge (gorge offers the cast only on a later turn and the action at any
// priority of its controller's turn).
func TestForetellMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []struct {
		what string
		flag *bool
	}{{"a foretold card is castable on the turn it was foretold", &dirac.DebugForetellSameTurn}, {"the foretell action waits for sorcery timing", &dirac.DebugForetellSorcery},
		{"a spell cast from exile reads as cast from a hand (Delayed Blast Fireball deals 2, not 5)", &dirac.DebugWasCastFromHandIgnored}} {
		t.Run(m.what, func(t *testing.T) {
			*m.flag = true
			defer func() { *m.flag = false }()
			w := runForetell(t, 200, false, nil)
			if w.divs == 0 {
				t.Fatalf("%s in %d games with no divergence", m.what, w.games)
			}
			t.Logf("mutation diverged in %d of %d games: %s", w.divs, w.games, w.first)
		})
	}
}
