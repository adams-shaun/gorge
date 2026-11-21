package oracle

import (
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestExchangeLifeLockstep: api:ExchangeLife's one-target arm (gorge
// engine/effects/life.go effExchangeLife) in lockstep over the exchangelife
// fixture pool: Magus of the Mirror and Mirror Universe exchange life totals
// with the opponent in the controller's upkeep. Soul Warden and Ajani's
// Pridemate observe the gain side's life-gain triggers, Rampaging Ferocidon
// swallows the gain. Both payment surfaces run (every fourth seed uses
// gorge's autopay surface). The finish hook counts the exchanges off gorge's
// finished event log (a Mirror Universe sacrificed to its own ability; two adjacent
// opposite LifeChange events on two players) and the test fails when the pool never reaches one, so it
// cannot pass vacuously. dirac is two-seat, so there is no pod arm.
var kexlDecks = [2][]cardset.Entry{
	{
		{Name: "Magus of the Mirror", Count: 4}, {Name: "Mirror Universe", Count: 3}, {Name: "Soul Warden", Count: 2}, {Name: "Ajani's Pridemate", Count: 2},
		{Name: "Rampaging Ferocidon", Count: 1}, {Name: "Memnite", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Hill Giant", Count: 2}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Swamp", Count: 8}, {Name: "Plains", Count: 4},
	},
	{
		{Name: "Magus of the Mirror", Count: 3}, {Name: "Mirror Universe", Count: 3}, {Name: "Soul Warden", Count: 2}, {Name: "Ajani's Pridemate", Count: 2},
		{Name: "Memnite", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 2},
	},
}

func kexlSeeds() uint64 {
	if v, e := strconv.Atoi(os.Getenv("EXCHANGE_LIFE_SEEDS")); e == nil && v > 0 {
		return uint64(v)
	}
	return 96
}

func TestExchangeLifeLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exchangelife")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, kexlDecks)
	tbl, _, _, err := set.Matchup(kexlDecks[0], kexlDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := kexlSeeds()
	exchanges, swaps := 0, 0
	finish := func(g *GorgeSide) {
		evs := g.E.L.Events
		for i := range evs {
			e := &evs[i]
			if e.Kind != events.MoveZone || e.From != state.ZBattlefield || e.To != state.ZGraveyard {
				continue
			}
			o := g.E.G.Obj(e.Obj)
			if o == nil || o.Card == nil || len(o.Card.Faces) == 0 {
				continue
			}
			if o.Card.Faces[0].Name == "Mirror Universe" {
				exchanges++ // the pool has no artifact removal: it leaves only as the ability's sacrifice cost
			}
		}
		for i := 0; i+1 < len(evs); i++ {
			a, b := &evs[i], &evs[i+1]
			if a.Kind == events.LifeChange && b.Kind == events.LifeChange && a.Player != b.Player &&
				a.Amount != 0 && a.Amount == -b.Amount {
				swaps++ // the exchange's two opposite LifeChange events
				i++
			}
		}
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		m, err := NewMatchup(reg, [2]string{"kexl-a", "kexl-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 30, Finish: finish})
		CloseSide(k)
		if s := res.Stall; len(s) >= len(KernelStallPrefix) && s[:len(KernelStallPrefix)] == KernelStallPrefix {
			t.Fatalf("seed %d: kernel stall %s", seed, s)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d seeds: %d Mirror Universe sacrifices, %d opposite life-change pairs", seeds, exchanges, swaps)
	if exchanges < 3 || swaps < 3 {
		t.Fatalf("%d exchange abilities and %d life-change pairs over %d seeds: the pool does not exercise api:ExchangeLife", exchanges, swaps, seeds)
	}
}

// TestExchangeLifeMutationDiverges is the pool's mutation check:
// DebugExchangeLifeOneSide applies only the controller's half, so a game that
// exchanges must diverge from gorge.
func TestExchangeLifeMutationDiverges(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exchangelife")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, kexlDecks)
	tbl, _, _, err := set.Matchup(kexlDecks[0], kexlDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugExchangeLifeOneSide = true
	defer func() { dirac.DebugExchangeLifeOneSide = false }()
	for seed := uint64(1); seed <= kexlSeeds(); seed++ {
		m, err := NewMatchup(reg, [2]string{"kexl-a", "kexl-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{MaxDecisions: 2000, MaxTurns: 30})
		CloseSide(k)
		if res.Div != nil {
			return
		}
	}
	t.Fatal("DebugExchangeLifeOneSide never diverged: the pool does not observe the exchange")
}
