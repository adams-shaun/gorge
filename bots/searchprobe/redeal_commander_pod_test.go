package searchprobe

import (
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// podRoot is a 4-seat Commander game stopped at one of the actor's own
// decisions, with everything a redeal of that decision reads.
type podRoot struct {
	setup     PublicGame
	h         History
	known     KnownCards
	engine    *rules.Engine
	collector *Collector
}

// playPodRoot plays the five-precon 4-seat Commander acceptance game with the
// shared policy answering every seat, capturing through the actor's collector
// at every decision, and stops at the actor's first decision at which stop
// holds.
// The game is a pure function of the seed, so each actor's run is the same
// game stopped at a different decision.
func playPodRoot(t *testing.T, actor state.PlayerID, stop func(e *rules.Engine) bool) podRoot {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	names, decks, commanders, err := testutil.CommanderAcceptanceDecks(reg)
	if err != nil {
		t.Fatal(err)
	}
	cfg := rules.CommanderAcceptanceConfig(reg, names, decks, commanders)
	e := rules.New(cfg)
	e.Advance()
	p := podRoot{
		setup:     PublicGame{Names: cfg.Names, Decks: cfg.Decks, Tokens: cfg.Tokens, StartingLife: cfg.StartingLife},
		h:         History{Actor: actor, Answers: make(map[int][]Action)},
		engine:    e,
		collector: NewCollector(actor),
	}
	r := rand.New(rand.NewPCG(4, 8))
	pos := 0
	for i := 0; i < 200000; i++ {
		d := e.Pending()
		if d == nil || e.G.Over {
			break
		}
		f, err := p.collector.Capture(e, e.L.Events[pos:])
		if err != nil {
			t.Fatal(err)
		}
		pos = len(e.L.Events)
		p.h.Frames = append(p.h.Frames, f)
		if d.Player == actor && len(e.G.Stack) == 0 && stop(e) {
			p.known, err = ProjectKnownCards(p.h)
			if err != nil {
				t.Fatal(err)
			}
			return p
		}
		in := botpolicy.Decide(botpolicy.BoardFromGame(e.G, e, d.Player), d, r)
		if d.Player == actor {
			if p.h.Answers[len(p.h.Frames)-1], err = p.collector.Actions(d, in); err != nil {
				t.Fatal(err)
			}
		}
		if err := e.Submit(in); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatalf("seat %d never reached the wanted position (game over %v, turn %d)", actor, e.G.Over, e.G.Turn)
	return p
}

// zoneIDsNames is a zone's card names in zone order.
func zoneIDsNames(e *rules.Engine, zone state.Zone, p state.PlayerID) []string {
	return zoneNames(e, zone, p)
}

// sortedNames is a zone's card names, sorted, for zones whose order is not
// a fact (the hand).
func sortedNames(e *rules.Engine, zone state.Zone, p state.PlayerID) []string {
	out := zoneNames(e, zone, p)
	slices.Sort(out)
	return out
}

// TestRedealFourSeatCommanderHidesOnlyHidden: at a decision of a 4-seat
// Commander game, for each of the four seats as the observer, the honest
// redeal of every one of its three opponents moves hidden cards only. The
// commanders stay in the command zone, each seat's library and hand keep their
// sizes, the observer's own hidden zones and every public zone are unchanged,
// and the opponents' hidden zones really do change (so the test is not
// comparing a world with itself).
func TestRedealFourSeatCommanderHidesOnlyHidden(t *testing.T) {
	if testing.Short() {
		t.Skip("plays four 4-seat Commander games")
	}
	const seats = 4
	publicZones := []state.Zone{state.ZBattlefield, state.ZGraveyard, state.ZExile, state.ZStack, state.ZCommand}
	for actor := state.PlayerID(0); actor < seats; actor++ {
		root := playPodRoot(t, actor, func(e *rules.Engine) bool { return e.G.Turn >= 3 })
		live := root.engine
		if got := len(live.G.Players); got != seats {
			t.Fatalf("actor %d: %d players, want %d", actor, got, seats)
		}
		inCommand := 0
		for p := state.PlayerID(0); p < seats; p++ {
			inCommand += len(live.G.Zone(state.ZCommand, p))
		}
		if inCommand == 0 {
			t.Fatalf("actor %d: no commander is in a command zone at the root; the commander assertions below would be vacuous", actor)
		}
		redealer, reason := NewRedealer(root.setup, root.h, root.known, RedealBase{Engine: live, Observer: root.collector})
		if reason != "" {
			t.Fatalf("actor %d: redealer refused: %s", actor, reason)
		}
		moved := make([]bool, seats)
		for w := uint64(0); w < 3; w++ {
			world, reason := redealer.Deal([2]uint64{w + 1, 7*w + 11}, nil)
			if reason != "" {
				t.Fatalf("actor %d world %d: deal refused: %s", actor, w, reason)
			}
			for p := state.PlayerID(0); p < seats; p++ {
				if got, want := len(world.G.Zone(state.ZHand, p)), len(live.G.Zone(state.ZHand, p)); got != want {
					t.Errorf("actor %d world %d seat %d: hand %d cards, want %d", actor, w, p, got, want)
				}
				if got, want := len(world.G.Zone(state.ZLibrary, p)), len(live.G.Zone(state.ZLibrary, p)); got != want {
					t.Errorf("actor %d world %d seat %d: library %d cards, want %d", actor, w, p, got, want)
				}
				for _, z := range publicZones {
					if !reflect.DeepEqual(zoneIDsNames(world, z, p), zoneIDsNames(live, z, p)) {
						t.Errorf("actor %d world %d seat %d: zone %v changed: %v, want %v", actor, w, p, z,
							zoneIDsNames(world, z, p), zoneIDsNames(live, z, p))
					}
				}
				// A commander the live game keeps in the command zone is
				// nowhere in the world's hidden zones of its owner.
				for _, id := range live.G.Zone(state.ZCommand, p) {
					name := live.G.Obj(id).Card.Faces[0].Name
					for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
						if slices.Contains(zoneNames(world, z, p), name) {
							t.Errorf("actor %d world %d seat %d: commander %q was dealt into %v", actor, w, p, name, z)
						}
					}
				}
				hiddenSame := reflect.DeepEqual(zoneNames(world, state.ZLibrary, p), zoneNames(live, state.ZLibrary, p)) &&
					reflect.DeepEqual(sortedNames(world, state.ZHand, p), sortedNames(live, state.ZHand, p))
				if p == actor {
					// The observer knows its own hand and which cards its
					// library holds, not their order.
					for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
						if !reflect.DeepEqual(sortedNames(world, z, p), sortedNames(live, z, p)) {
							t.Errorf("actor %d world %d: the observer's own %v changed", actor, w, z)
						}
					}
					continue
				}
				if !hiddenSame {
					moved[p] = true
				}
			}
		}
		for p := state.PlayerID(0); p < seats; p++ {
			if p != actor && !moved[p] {
				t.Errorf("actor %d: opponent seat %d's hidden cards were never redealt in any world; the test compares a world with itself", actor, p)
			}
		}
	}
}

// TestRedealSkipsDepartedSeats: once a seat has left the game (CR 800.4a
// ceased every card it owned), the redeal of the position still builds and
// deals. The departed seat has no hidden pool, so its deck list is not held
// against the cards left in play; refusing the root there would drop the
// searching seat to its wrapped bot for the rest of the game.
func TestRedealSkipsDepartedSeats(t *testing.T) {
	if testing.Short() {
		t.Skip("plays a 4-seat Commander game")
	}
	root := playPodRoot(t, 0, func(e *rules.Engine) bool {
		if e.G.Players[0].Lost {
			return false
		}
		for i := range e.G.Players {
			if e.G.Players[i].Lost {
				return true
			}
		}
		return false
	})
	live := root.engine
	departed := -1
	for i := range live.G.Players {
		if live.G.Players[i].Lost {
			departed = i
		}
	}
	// Precondition: the departed seat's cards really are gone from its
	// zones, so the case is the one that used to be refused.
	if departed < 0 || len(live.G.Zone(state.ZLibrary, state.PlayerID(departed)))+len(live.G.Zone(state.ZHand, state.PlayerID(departed))) != 0 {
		t.Fatalf("seat %d: not a departed seat with empty hidden zones", departed)
	}
	redealer, reason := NewRedealer(root.setup, root.h, root.known, RedealBase{Engine: live, Observer: root.collector})
	if reason != "" {
		t.Fatalf("redealer refused a position with seat %d departed: %s", departed, reason)
	}
	world, reason := redealer.Deal([2]uint64{3, 5}, nil)
	if reason != "" {
		t.Fatalf("deal refused: %s", reason)
	}
	for p := state.PlayerID(0); int(p) < len(live.G.Players); p++ {
		if got, want := len(world.G.Zone(state.ZHand, p)), len(live.G.Zone(state.ZHand, p)); got != want {
			t.Errorf("seat %d: hand %d cards, want %d", p, got, want)
		}
		if got, want := len(world.G.Zone(state.ZLibrary, p)), len(live.G.Zone(state.ZLibrary, p)); got != want {
			t.Errorf("seat %d: library %d cards, want %d", p, got, want)
		}
	}
}
