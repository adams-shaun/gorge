package searchseat

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func twoCardDeck(t *testing.T, reg *cards.Registry, land, spell string) []*cards.Card {
	t.Helper()
	l, ok := reg.Lookup(land)
	if !ok {
		t.Fatalf("%q not in the corpus", land)
	}
	s, ok := reg.Lookup(spell)
	if !ok {
		t.Fatalf("%q not in the corpus", spell)
	}
	deck := make([]*cards.Card, 0, 60)
	for i := 0; i < 30; i++ {
		deck = append(deck, l, s)
	}
	return deck
}

// playWithFeeds plays bot games over seeds, handing visit every decision with
// the deciding seat's feed captured through it.
func playWithFeeds(t *testing.T, decks [][]*cards.Card, seeds int, visit func(e *rules.Engine, f *Feed, seed uint64, step int)) {
	t.Helper()
	names := []string{"a", "b"}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		e := rules.New(rules.Config{Seed: seed, Names: names, Decks: decks})
		e.Advance()
		rngs := searchprobe.BotRandoms(seed, 2)
		board := botpolicy.NewBoard(2)
		feeds := [2]*Feed{NewFeed(0), NewFeed(1)}
		for step := 0; !e.G.Over && step < 2000; step++ {
			d := e.Pending()
			if d == nil {
				break
			}
			for i, f := range feeds {
				if _, ok := f.Observe(e); !ok {
					t.Fatalf("seed %d: seat %d's feed stopped: %s", seed, i, f.StopReason())
				}
			}
			visit(e, feeds[d.Player], seed, step)
			in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
			if err := feeds[d.Player].RecordAnswer(d, in); err != nil {
				t.Fatal(err)
			}
			if err := e.Submit(in); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// A library top the seat can see is part of what it observes, so an honest
// root keeps that card on top: Courser of Kruphix reveals its controller's top
// to both seats (CR 401.5), Vizier of the Menagerie shows it to its controller
// alone.
func TestHonestRootKeepsAVisibleLibraryTop(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	decks := [][]*cards.Card{
		twoCardDeck(t, reg, "Forest", "Courser of Kruphix"),
		twoCardDeck(t, reg, "Forest", "Vizier of the Menagerie"),
	}
	setup := searchprobe.PublicGame{Names: []string{"a", "b"}, Decks: decks}
	revealed, private := 0, 0
	playWithFeeds(t, decks, 6, func(e *rules.Engine, f *Feed, seed uint64, step int) {
		actor := e.Pending().Player
		for pi := range e.G.Players {
			p := state.PlayerID(pi)
			lib := e.G.Zone(state.ZLibrary, p)
			public, own := e.LibraryTopRevealed(p), p == actor && e.MayLookAtLibraryTop(p)
			if len(lib) == 0 || (!public && !own) {
				continue
			}
			if public {
				revealed++
			} else {
				private++
			}
			root, why := HonestRoot(setup, f, e, [2]uint64{seed, uint64(step)})
			if root == nil {
				t.Fatalf("seed %d step %d: seat %d sees player %d's top: %s", seed, step, actor, p, why)
			}
			if got := root.G.Zone(state.ZLibrary, p)[0]; got != lib[0] {
				t.Fatalf("seed %d step %d: the root's top of player %d is object %d, the seat sees %d", seed, step, p, got, lib[0])
			}
		}
	})
	if revealed == 0 || private == 0 {
		t.Fatalf("precondition: %d revealed and %d private tops seen", revealed, private)
	}
}

// The monarch's end-step draw names a hidden library card as its source, so an
// honest root keeps that card where it is and the seats get their answer with
// the trigger on the stack.
func TestHonestRootWithAStackSourceInALibrary(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	decks := [][]*cards.Card{
		twoCardDeck(t, reg, "Plains", "Palace Jailer"),
		twoCardDeck(t, reg, "Plains", "Palace Jailer"),
	}
	setup := searchprobe.PublicGame{Names: []string{"a", "b"}, Decks: decks}
	seen := 0
	playWithFeeds(t, decks, 6, func(e *rules.Engine, f *Feed, seed uint64, step int) {
		for i := range e.G.Objs {
			o := &e.G.Objs[i]
			if o.Zone != state.ZStack {
				continue
			}
			src := e.G.Obj(o.Source)
			if src == nil || src.Zone != state.ZLibrary {
				continue
			}
			seen++
			if root, why := HonestRoot(setup, f, e, [2]uint64{seed, uint64(step)}); root == nil {
				t.Fatalf("seed %d step %d: %s", seed, step, why)
			}
			break
		}
	})
	if seen == 0 {
		t.Fatal("precondition: no stack entry with a source in a library")
	}
}
