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

// The player-hexproof-until-your-next-turn lockstep (CR 702.11c, 611.2b):
// Blossoming Calm gives its caster hexproof until their next turn, which the
// kernel lowers to a next-turn emblem carrying a plain StatPlayerHexproof
// (static_emblemphex.go). Sign in Blood targets a player, so an opponent's
// cast while the emblem stands must offer only its caster, and the caster's
// own cast may still pick themselves. Both seats play the same deck.

var kBlossomingCalmDeck = []cardset.Entry{
	{Name: "Blossoming Calm", Count: 4},
	{Name: "Sign in Blood", Count: 4},
	{Name: "Doom Blade", Count: 4},
	{Name: "Savannah Lions", Count: 4},
	{Name: "Walking Corpse", Count: 4},
	{Name: "Plains", Count: 10},
	{Name: "Swamp", Count: 10},
}

// blossomingCalmCards are the pool's cards whose cast from the hand the scan
// counts.
var blossomingCalmCards = map[string]bool{"Blossoming Calm": true, "Sign in Blood": true, "Doom Blade": true}

// blossomingCalmScan counts, in gorge's event log, each carrier's cast from
// the hand.
func blossomingCalmScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		if ev.Kind != events.PutOnStack {
			continue
		}
		if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ev.From == state.ZHand && blossomingCalmCards[ob.Face().Name] {
			n[ob.Face().Name]++
		}
	}
}

// runBlossomingCalm plays the seeded games; report logs the first divergence
// as a test error (the mutation run expects one and stays quiet).
func runBlossomingCalm(t *testing.T, seeds uint64, report bool) (diverged bool, st map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("blossomingcalm")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range kBlossomingCalmDeck {
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
	tbl, _, _, err := set.Matchup(kBlossomingCalmDeck, kBlossomingCalmDeck)
	if err != nil {
		t.Fatal(err)
	}
	st = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcalm-a", "kcalm-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &oneRingProbe{base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x63616c6d))}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 25})
		CloseSide(k)
		blossomingCalmScan(probe.gs, st)
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			} else {
				t.Logf("seed %d: diverged at decision %d: %s %s", seed, d.At, d.What, d.Field)
			}
			return
		}
	}
	return
}

func TestBlossomingCalmLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	div, st := runBlossomingCalm(t, 256, true)
	t.Logf("256 games, %v", st)
	if div {
		t.Fatalf("diverged")
	}
	for _, name := range []string{"Blossoming Calm", "Sign in Blood"} {
		if st[name] == 0 {
			t.Errorf("%s never cast from the hand", name)
		}
	}
}

// TestBlossomingCalmEmbPlHexMutation: without the kernel's emblem hexproof
// reader an opponent's Sign in Blood may target the player Blossoming Calm
// made hexproof where gorge refuses, so the lockstep must diverge.
func TestBlossomingCalmEmbPlHexMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugEmbPlHexOff = true
	defer func() { dirac.DebugEmbPlHexOff = false }()
	if div, _ := runBlossomingCalm(t, 64, false); !div {
		t.Fatal("DebugEmbPlHexOff: no divergence in 64 seeds")
	}
}
