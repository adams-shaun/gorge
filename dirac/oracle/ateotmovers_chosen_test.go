package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The chosen-card Animate AtEOT carriers (dirac/kgen/lower_ateot.go
// oneObjectSubject's one-card ChooseMove/Dig arm): a hand pick of one
// (Sneak Attack, Arms Race, Incandescent Soulstoke) or a Dig of one
// (Impromptu Raid, Killer Instinct) remembers at most one card, which is the
// one object the kernel's OpAtEOT takes. The decks are split so the hand-pick
// and Dig carriers each play beside cheap creatures. Meek Attack, the sixth
// carrier, rides the pick deck: its ChangeType$ totalPT_LE5 predicate reads a
// hand card's P/T (matchCardOneAt's PredTotalPT arm), and Squawkroaster's
// */4 keeps the characteristic-defining power in play.
//
// A Dig whose top card is not a creature moves it to the remainder zone
// without taking it; gorge's digRemember remembers every card the dig moved,
// so the end step still puts the no-op __kwEncoreSacrifice trigger on the
// stack (CR 603.7). The kernel's Dig remembers the remainder too and OpAtEOT
// promises on a remembered object off the battlefield, so the miss is in
// lockstep rather than a tolerated divergence.
var kateotChosenPickDeck = []cardset.Entry{
	{Name: "Sneak Attack", Count: 4}, {Name: "Arms Race", Count: 3},
	{Name: "Incandescent Soulstoke", Count: 3}, {Name: "Meek Attack", Count: 3},
	{Name: "Akoum Hellhound", Count: 5}, {Name: "Gilded Ghoda", Count: 3},
	{Name: "Daggerfang Duo", Count: 3}, {Name: "Deathcap Marionette", Count: 3},
	{Name: "Squawkroaster", Count: 2},
	{Name: "Mountain", Count: 5}, {Name: "Forest", Count: 3}, {Name: "Swamp", Count: 3},
}

var kateotChosenDigDeck = []cardset.Entry{
	{Name: "Impromptu Raid", Count: 4}, {Name: "Killer Instinct", Count: 2},
	{Name: "Mountain", Count: 13}, {Name: "Akoum Hellhound", Count: 8},
	{Name: "Gilded Ghoda", Count: 3}, {Name: "Daggerfang Duo", Count: 3},
	{Name: "Deathcap Marionette", Count: 3}, {Name: "Swamp", Count: 4},
}

// kateotChosenScan counts, from gorge's event log, the next-end-step
// sacrifice promises that fired (a DelayedRegister body __kwEncoreSacrifice
// whose object then left the battlefield during an end step) and the ones
// registered over an object that was never on the battlefield (a Dig that
// missed: the end step's no-op trigger).
func kateotChosenScan(g *GorgeSide) (sacked, missed int) {
	if g == nil {
		return
	}
	reg := map[state.ObjID]string{}
	zone := map[state.ObjID]state.Zone{}
	step := state.Step(0)
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.StepChange:
			step = ev.Step
		case events.MoveZone:
			zone[ev.Obj] = ev.To
			if ev.From != state.ZBattlefield {
				continue
			}
			body, ok := reg[ev.Obj]
			if !ok {
				continue
			}
			delete(reg, ev.Obj)
			if step == state.StepEnd && body == "__kwEncoreSacrifice" && ev.To == state.ZGraveyard {
				sacked++
			}
		case events.DelayedRegister:
			if ev.Counter != "__kwEncoreSacrifice" && ev.Counter != "__kwWarpExile" {
				continue
			}
			reg[ev.Obj] = ev.Counter
			if ev.Counter == "__kwEncoreSacrifice" && zone[ev.Obj] != state.ZBattlefield {
				missed++ // a rest move that never entered: the no-op end-step trigger
			}
		}
	}
	return
}

// kateotChosenRun mirrors kateotRun over deck, strictly: any divergence
// fails, and sacked/missed come from gorge's event log.
func kateotChosenRun(t *testing.T, deck []cardset.Entry, seeds uint64) (games, sacked, missed int, diverged bool, stalls map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("ateotmovers")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range deck {
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
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	stalls = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kateot-c", "kateot-d"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := RandomPolicy(seed)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		closeErr := k.Err()
		CloseSide(k)
		games++
		if closeErr != nil {
			t.Fatalf("seed %d: kernel error: %v", seed, closeErr)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls[res.Stall]++
		}
		s, miss := kateotChosenScan(gs)
		sacked += s
		missed += miss
		if d := res.Div; d != nil {
			diverged = true
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

// TestAteotmoversChosenPickLockstep: the hand-pick carriers beside cheap
// creatures in both seats, strictly in lockstep: the picked card and the
// empty pick (no matching creature in hand, so neither engine registers a
// promise) must agree, and the picked card must be sacrificed at the end step.
func TestAteotmoversChosenPickLockstep(t *testing.T) {
	t.Parallel()
	games, sacked, _, diverged, stalls := kateotChosenRun(t, kateotChosenPickDeck, 64)
	t.Logf("%d games: %d objects sacrificed at the end step; kernel stalls %v", games, sacked, stalls)
	if diverged {
		t.Errorf("the hand-pick AtEOT carriers diverged")
	}
	if sacked < 5 {
		t.Errorf("the end-step rider fired on %d objects, want at least 5", sacked)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}

// TestAteotmoversChosenDigLockstep: the Dig carriers. A Dig that hits takes a
// creature onto the battlefield and both engines promise its end-step
// sacrifice; a Dig that misses (a non-creature top card, the empty-pick case)
// remembers the card it moved and both engines put the no-op trigger on the
// stack. Both classes must lockstep, and the test asserts the miss really was
// reached.
func TestAteotmoversChosenDigLockstep(t *testing.T) {
	t.Parallel()
	games, sacked, missed, diverged, stalls := kateotChosenRun(t, kateotChosenDigDeck, 64)
	t.Logf("%d games: %d objects sacrificed at the end step, %d dig-miss promises; kernel stalls %v",
		games, sacked, missed, stalls)
	if diverged {
		t.Errorf("the dig-card AtEOT carriers diverged")
	}
	if sacked < 5 {
		t.Errorf("the end-step rider fired on %d objects, want at least 5", sacked)
	}
	if missed < 1 {
		t.Errorf("the dig-miss (empty-pick) class was never reached in %d games", games)
	}
	if len(stalls) != 0 {
		t.Errorf("kernel capacity stalls %v", stalls)
	}
}
