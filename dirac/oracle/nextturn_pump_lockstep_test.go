package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The rest of this change's until-your-next-turn lowerings, decision by
// decision against gorge (CR 611.2a): the cards the onering pool does not
// play. A-The One Ring and The Stasis Coffin grant their controller
// protection from everything until their next turn, through the same emblem
// The One Ring's grant lowers to. Blue Dragon, Mu Yanling, Celestial Wind and
// Wasp, Shrinking Savior give creatures -N/-0, and Liliana, the Last Hope and
// Hag of Inner Weakness give -2/-1, until their controller's next turn. Both
// seats play this deck, so every carrier is cast by both engines.
// Coral Merfolk and Typhoid Rats are the X/1s a -2/-1 kills and the
// deathtouch a -N/-0 silences, and Sign in Blood targets a player the
// protection must refuse.

var kNextTurnDeck = []cardset.Entry{
	{Name: "A-The One Ring", Count: 2},
	{Name: "The Stasis Coffin", Count: 2},
	{Name: "Blue Dragon", Count: 2},
	{Name: "Mu Yanling, Celestial Wind", Count: 2},
	{Name: "Wasp, Shrinking Savior", Count: 3},
	{Name: "Liliana, the Last Hope", Count: 2},
	{Name: "Hag of Inner Weakness", Count: 3},
	{Name: "Coral Merfolk", Count: 2},
	{Name: "Typhoid Rats", Count: 2},
	{Name: "Walking Corpse", Count: 1},
	{Name: "Sign in Blood", Count: 2},
	{Name: "Island", Count: 9},
	{Name: "Swamp", Count: 8},
}

// nextTurnCards are the carriers whose cast from the hand the scan counts,
// whose activations it counts as "act" and whose triggers as "trig".
var nextTurnCards = map[string]bool{
	"A-The One Ring": true, "The Stasis Coffin": true, "Blue Dragon": true,
	"Mu Yanling, Celestial Wind": true, "Wasp, Shrinking Savior": true,
	"Liliana, the Last Hope": true, "Hag of Inner Weakness": true, "Sign in Blood": true,
}

// nextTurnScan counts, in gorge's event log, each carrier's cast from the
// hand, each activation and trigger its permanents put on the stack, and
// every burden counter placed.
func nextTurnScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.PutOnStack:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ev.From == state.ZHand && nextTurnCards[ob.Face().Name] {
				n[ob.Face().Name]++
			}
		case events.AbilityPush:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && nextTurnCards[ob.Face().Name] {
				n[ob.Face().Name+" act"]++
			}
		case events.TriggerPush:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && nextTurnCards[ob.Face().Name] {
				n[ob.Face().Name+" trig"]++
			}
		case events.CounterChange:
			if ev.Counter == "BURDEN" {
				n["burden"]++
			}
		}
	}
}

// runNextTurn plays the seeded games with the onering probe. report logs the
// first divergence as a test error; the mutation runs expect one and stay
// quiet.
func runNextTurn(t *testing.T, seeds uint64, report bool) (diverged bool, st map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("nextturn")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kNextTurnDeck {
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
	tbl, _, _, err := set.Matchup(kNextTurnDeck, kNextTurnDeck)
	if err != nil {
		t.Fatal(err)
	}
	st = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"knextturn-a", "knextturn-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &oneRingProbe{base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x6e787474))}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 25})
		CloseSide(k)
		nextTurnScan(probe.gs, st)
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

func TestNextTurnPumpLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	div, st := runNextTurn(t, 256, true)
	t.Logf("256 games, %v", st)
	if div {
		t.Fatalf("diverged")
	}
	for name, want := range map[string]int{
		"A-The One Ring":                 32,
		"A-The One Ring act":             64,
		"A-The One Ring trig":            64,
		"The Stasis Coffin":              32,
		"The Stasis Coffin act":          32,
		"Blue Dragon":                    16,
		"Blue Dragon trig":               16,
		"Mu Yanling, Celestial Wind":     16,
		"Mu Yanling, Celestial Wind act": 32,
		"Wasp, Shrinking Savior":         64,
		"Wasp, Shrinking Savior trig":    64,
		"Liliana, the Last Hope":         32,
		"Liliana, the Last Hope act":     64,
		"Hag of Inner Weakness":          64,
		"Hag of Inner Weakness trig":     128,
		"Sign in Blood":                  32,
		"burden":                         64,
	} {
		if st[name] < want {
			t.Errorf("%s %d, want at least %d", name, st[name], want)
		}
	}
}

// TestNextTurnNTEOTMutation: the -N/-0 and -2/-1 records ending at end of
// turn instead of at their controller's next turn must make the lockstep
// diverge.
func TestNextTurnNTEOTMutation(t *testing.T) {
	dirac.DebugPumpNTEndsEOT = true
	defer func() { dirac.DebugPumpNTEndsEOT = false }()
	if div, _ := runNextTurn(t, 64, false); !div {
		t.Fatal("DebugPumpNTEndsEOT: no divergence in 64 seeds")
	}
}

// TestNextTurnPlProtAllMutation: without the kernel's protection static, A-The
// One Ring's and The Stasis Coffin's grants let through the damage and the
// targets gorge refuses, so the lockstep must diverge.
func TestNextTurnPlProtAllMutation(t *testing.T) {
	dirac.DebugPlProtAllOff = true
	defer func() { dirac.DebugPlProtAllOff = false }()
	if div, _ := runNextTurn(t, 64, false); !div {
		t.Fatal("DebugPlProtAllOff: no divergence in 64 seeds")
	}
}
