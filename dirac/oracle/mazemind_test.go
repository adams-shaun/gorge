package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The counter-threshold state-trigger lockstep: Mazemind Tome's
// "When there are four or more page counters on CARDNAME, exile it and you
// gain 4 life" (Mode$ Always, IsPresent$ Card.Self+counters_GE4_PAGE) is a
// CR 603.8 state trigger: the condition is checked when a counter is put on
// the source and again when an instance leaves the stack, and only one
// instance waits at a time. The Tome's own activations put the page counters
// (the Scry and Draw costs), so the fourth activation fires it. gorge
// re-checks at every event; the kernel re-checks on the counter-put event
// and at every resolution's finish, so a counter put while the instance
// waits must not queue a second one, and an instance countered by Nimble
// Obstructionist's cycle-trigger must re-fire while the Tome still meets the
// threshold (Mazemind Tome's ruling; gorge stateTriggerOutstanding).

// kmazemindDecks: the Tome player's activations reach the fourth page
// counter; the responder's Ogres do nothing.
var kmazemindDecks = [2][]cardset.Entry{
	{
		{Name: "Mazemind Tome", Count: 4}, {Name: "Island", Count: 36},
	},
	{
		{Name: "Gray Ogre", Count: 8}, {Name: "Island", Count: 32},
	},
}

// kmazemindRespondDecks: the responder answers the trigger with Spider-Sense,
// which counters a spell or a triggered ability on the stack and leaves its
// source on the battlefield.
var kmazemindRespondDecks = [2][]cardset.Entry{
	kmazemindDecks[0],
	{
		{Name: "Spider-Sense", Count: 4}, {Name: "Island", Count: 36},
	},
}

// runMazemind plays the seeds with the random policy over the state-trigger
// pool and returns the games, the divergences, the Tomes that ended exiled
// (the trigger resolved) and the Tomes that ended in a graveyard (a drawn
// copy discarded to hand size: the pool has no removal).
func runMazemind(t *testing.T, kdecks [2][]cardset.Entry, seeds int, mutate string) (games, divs, exiles, grave int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("statectr")
	if err != nil {
		t.Fatal(err)
	}
	oldLatch, oldRefire := dirac.DebugStateCtrNoLatch, dirac.DebugStateCtrNoRefire
	dirac.DebugStateCtrNoLatch = mutate == "latch"
	dirac.DebugStateCtrNoRefire = mutate == "refire"
	defer func() { dirac.DebugStateCtrNoLatch, dirac.DebugStateCtrNoRefire = oldLatch, oldRefire }()
	var decks [2][]*cards.Card
	for s, d := range kdecks {
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
	tbl, _, _, err := set.Matchup(kdecks[0], kdecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kmazemind-a", "kmazemind-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := RandomPolicy(seed)
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		CloseSide(k)
		games++
		for p := state.PlayerID(0); p < 2; p++ {
			for _, id := range g.E.G.Zone(state.ZExile, p) {
				if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Mazemind Tome" {
					exiles++
				}
			}
			for _, id := range g.E.G.Zone(state.ZGraveyard, p) {
				if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Mazemind Tome" {
					grave++
				}
			}
		}
		if d := res.Div; d != nil {
			divs++
			if mutate == "" {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, exiles, grave
}

func TestMazemindTomeLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, exiles, _ := runMazemind(t, kmazemindDecks, 256, "")
	t.Logf("%d games, %d divergences, Tomes exiled by the trigger %d", games, divs, exiles)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if exiles < 5 {
		t.Errorf("only %d Tomes exiled by the trigger (want at least 5)", exiles)
	}
}

// TestMazemindResponseLockstep is the second case: the trigger is answered
// on the stack. A Spider-Sense counter leaves the Tome in play at threshold,
// which re-fires a fresh instance once the countered one has left the stack;
// both engines do. Must match gorge with no second instance while the first
// one waits.
func TestMazemindResponseLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, exiles, grave := runMazemind(t, kmazemindRespondDecks, 256, "")
	t.Logf("%d games, %d divergences, Tomes exiled %d, Tomes in graveyards %d", games, divs, exiles, grave)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if exiles < 5 {
		t.Errorf("only %d Tomes exiled by the trigger (want at least 5): the response never left one to resolve", exiles)
	}
}

// TestMazemindLatchMutation drops the outstanding scan: a counter put while
// the instance waits queues a second one, so the games must diverge. A green
// mutation means the pool never puts a counter while an instance waits.
func TestMazemindLatchMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runMazemind(t, kmazemindDecks, 256, "latch")
	if divs == 0 {
		t.Fatalf("the latch mutation queued a second instance in %d games with no divergence", games)
	}
	t.Logf("the latch mutation diverged in %d of %d games", divs, games)
}

// TestMazemindRefireMutation drops the re-check at an instance's leave: a
// Tome countered while still at threshold never re-fires, so the games must
// diverge. A green mutation means the pool never counters an instance whose
// source stays on the battlefield -- the response case is not reached.
func TestMazemindRefireMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runMazemind(t, kmazemindRespondDecks, 256, "refire")
	if divs == 0 {
		t.Fatalf("the refire mutation dropped a re-fire in %d games with no divergence", games)
	}
	t.Logf("the refire mutation diverged in %d of %d games", divs, games)
}
