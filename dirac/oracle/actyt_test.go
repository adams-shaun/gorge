package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the
// generated actyt pool, whose carrier is Essence Anchor ("{T}: create a 2/2
// Zombie Druid. Activate only during your turn and only if a card left your
// graveyard this turn": PlayerTurn$ True beside CheckSVar$ X,
// Count$LeftGraveyardThisTurn Card.YouOwn). gorge reads the two gates
// independently (activationPhasesOK and sVarGateOK) and requires both; kgen
// lowers the ability's condition to a CondAnd of "your turn" and the count.
// The decks carry Think Twice (flashback, an instant that can be cast from
// the graveyard on the opponent's turn), Raise Dead, Gravedigger, Armored
// Skaab and Opt (cards that leave or fill the graveyard) and burn/bears.
// The test tallies, from gorge's state at each new (turn, step) window with
// an untapped Anchor, the windows where exactly one gate held (the
// activation must be withheld), from gorge's own per-turn zone-entry list.

var kactytDecks = [2][]cardset.Entry{
	{
		{Name: "Essence Anchor", Count: 4}, {Name: "Think Twice", Count: 4},
		{Name: "Raise Dead", Count: 3}, {Name: "Gravedigger", Count: 3},
		{Name: "Armored Skaab", Count: 3}, {Name: "Opt", Count: 3},
		{Name: "Island", Count: 10}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "Mulch", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Forest", Count: 12}, {Name: "Mountain", Count: 12},
	},
}

const kactytAnchor = "Essence Anchor"

// kactytTally counts Anchor activations and the windows where exactly one of
// the two gates held.
type kactytTally struct {
	acts       int // Essence Anchor activations
	noLeft     int // your-turn windows, Anchor untapped, no card left your graveyard
	oppLeft    int // opponent-turn windows, Anchor untapped, a card left your graveyard
	both       int // your-turn windows, Anchor untapped, a card left your graveyard
	evSeen     int
	turn       int
	lastWindow [2]int
}

func (y *kactytTally) reset() { *y = kactytTally{} }

func (y *kactytTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	for ; y.evSeen < len(evs); y.evSeen++ {
		switch ev := evs[y.evSeen]; ev.Kind {
		case events.TurnChange:
			y.turn++
		case events.AbilityPush:
			if firstFaceName(g, ev.Obj) == kactytAnchor {
				y.acts++
			}
		}
	}
	key := [2]int{y.turn, int(g.Step)}
	if key == y.lastWindow {
		return nil
	}
	y.lastWindow = key
	var left, untapped [2]bool
	for _, en := range g.Entered {
		if en.From != state.ZGraveyard {
			continue
		}
		if o := g.Obj(en.Obj); o != nil {
			left[o.Owner] = true
		}
	}
	for p := range untapped {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.Obj(id); o != nil && o.Controller == state.PlayerID(p) && !o.Tapped && firstFaceName(g, id) == kactytAnchor {
				untapped[p] = true
			}
		}
	}
	for p := range untapped {
		if !untapped[p] {
			continue
		}
		mine := g.Active == state.PlayerID(p)
		switch {
		case mine && !left[p]:
			y.noLeft++
		case mine:
			y.both++
		case left[p]:
			y.oppLeft++
		}
	}
	return nil
}

func kactytSetup(t *testing.T) (*cards.Registry, cardset.Set) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("actyt")
	if err != nil {
		t.Fatal(err)
	}
	return reg, set
}

// kactytMutate rewrites every built activated condition that is a CondAnd
// holding a CondYourTurn operand to that operand alone (the count gate
// dropped). It returns the number rewritten.
func kactytMutate(tbl *kop.Table) int {
	n := 0
	for i := range tbl.Cards {
		for j := range tbl.Cards[i].Abilities {
			ab := &tbl.Cards[i].Abilities[j]
			if ab.Kind == kop.AbTriggered || ab.Trig.Cond == 0 {
				continue
			}
			c := tbl.Conds[ab.Trig.Cond]
			if c.Kind != kop.CondAnd {
				continue
			}
			for _, h := range c.Sub {
				if tbl.Conds[h].Kind == kop.CondYourTurn {
					ab.Trig.Cond = h
					n++
					break
				}
			}
		}
	}
	return n
}

// kactytRun plays seeds 1..seeds; mutate rewrites the kernel table's Anchor
// condition first. It returns the first divergence's seed (0 none).
func kactytRun(t *testing.T, reg *cards.Registry, set cardset.Set, seeds uint64, mutate bool, tot *kactytTally) (games int, divSeed uint64, stalls int) {
	t.Helper()
	decks := kcastDeck(t, reg, kactytDecks)
	tbl, _, _, err := set.Matchup(kactytDecks[0], kactytDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate {
		if n := kactytMutate(tbl); n != 1 {
			t.Fatalf("mutated %d Anchor conditions, want 1", n)
		}
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kactyt-a", "kactyt-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var y kactytTally
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: y.check})
		CloseSide(k)
		games++
		tot.acts += y.acts
		tot.noLeft += y.noLeft
		tot.oppLeft += y.oppLeft
		tot.both += y.both
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			if !mutate {
				lo := max(d.At-12, 0)
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nlast actions %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
			}
			return games, seed, stalls
		}
	}
	return games, 0, stalls
}

func TestActytLockstep(t *testing.T) {
	t.Parallel()
	reg, set := kactytSetup(t)
	var tot kactytTally
	games, div, stalls := kactytRun(t, reg, set, 256, false, &tot)
	t.Logf("%d games, diverged seed %d, stalls %d", games, div, stalls)
	t.Logf("Essence Anchor activations %d; windows with the Anchor untapped: your turn with no card left %d, your turn after one left %d, opponent turn after one left %d",
		tot.acts, tot.noLeft, tot.both, tot.oppLeft)
	if div != 0 || stalls != 0 {
		return
	}
	if tot.acts < 20 {
		t.Errorf("%d Anchor activations, want at least 20", tot.acts)
	}
	if tot.noLeft < 10 {
		t.Errorf("%d your-turn windows with nothing left the graveyard, want at least 10", tot.noLeft)
	}
	if tot.oppLeft < 5 {
		t.Errorf("%d opponent-turn windows after a card left the graveyard, want at least 5", tot.oppLeft)
	}
}

// TestActytMutation: replacing the Anchor's built CondAnd with its
// CondYourTurn half (the kernel then offers the token on your turn with
// nothing gone from the graveyard) must make the lockstep diverge.
func TestActytMutation(t *testing.T) {
	reg, set := kactytSetup(t)
	var tot kactytTally
	_, div, _ := kactytRun(t, reg, set, 64, true, &tot)
	if div == 0 {
		t.Fatalf("your-turn-only Anchor condition: no divergence in 64 seeds")
	}
	t.Logf("diverged at seed %d", div)
}
