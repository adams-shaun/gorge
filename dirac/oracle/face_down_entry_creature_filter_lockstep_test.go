package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A face-down ENTRY is a creature entry (CR 708.2, 701.58a): a manifested or
// cloaked card enters as a 2/2 creature with no other types, so a Moved
// replacement's `Creature.faceDown+YouCtrl` ValidCard$ matches a cloaked
// NONCREATURE too (CR 614.12: the replacement applies to the object as it
// would enter). gorge's type predicate read the origin-zone card's printed
// types at match time -- the face-down marker is not folded until the move it
// intercepts applies -- so Veiled Ascension's "Face-down creatures you control
// enter with a flying counter on them" put nothing on a cloaked Plains. The
// recording is evloop tail-61 pair 20's game (decision 313: the flying counter
// projection, gorge 0 / kernel 1), where the Ascension cloaks a Plains.
// kernel DebugFaceDownFilterPrinted (dirac/turn_gen.go) restores the printed
// read for the mutation test.

// TestFaceDownEntryCreatureFilterReprosLockstep replays the recorded game:
// after the fix both engines put the flying counter on the face-down Plains
// and the game replays clean.
func TestFaceDownEntryCreatureFilterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "face_down_entry_creature_filter")
}

// TestFaceDownEntryCreatureFilterMutation makes the kernel match a face-down
// entrant by its printed card: the Plains stops being a creature for the
// filter, the replacement stops applying, and the recorded game must diverge
// again at the counter projection.
func TestFaceDownEntryCreatureFilterMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "face_down_entry_creature_filter.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugFaceDownFilterPrinted
	dirac.DebugFaceDownFilterPrinted = true
	defer func() { dirac.DebugFaceDownFilterPrinted = old }()
	res := replayDecodedRes(t, synthReg(t), globSet(t, r.Pool, r), r)
	if res.Div == nil {
		t.Fatalf("mutation matched a face-down entrant by its printed card in %d actions with no divergence", len(r.Actions))
	}
	t.Logf("mutation divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}

// facedownEntryDeck is Veiled Ascension beside noncreature cards (a sorcery,
// an instant, an artifact) and Plains: every cloak its upkeep trigger makes
// is of a card that is not a creature.
func facedownEntryDeck() []string {
	return deckOf(4, "Veiled Ascension",
		deckOf(4, "Grisly Spectacle",
			deckOf(4, "Eyeblight's Ending",
				deckOf(4, "Orazca Relic", deckOf(24, "Plains")...)...)...)...)
}

// facedownEntryTally reads gorge's log for the mechanism: a cloak of a
// noncreature card (the entry the filter must admit) and the flying counter
// such an entry receives from Veiled Ascension's replacement. counters are
// cumulative over the pod's seeds; last is per game.
type facedownEntryTally struct {
	last      int
	cloaked   map[state.ObjID]bool
	cloaks    int
	countered int
}

func (ft *facedownEntryTally) resetFirst() {
	ft.last = 0
	// each seed is its own game: an object id names a different object there,
	// so the cloak map must not carry across
	ft.cloaked = map[state.ObjID]bool{}
}

func (ft *facedownEntryTally) check(g *GorgeSide, _ Side) *Divergence {
	if ft.cloaked == nil {
		ft.cloaked = map[state.ObjID]bool{}
	}
	evs := g.E.L.Events
	for ; ft.last < len(evs); ft.last++ {
		e := &evs[ft.last]
		switch e.Kind {
		case events.MoveZone:
			if e.To != state.ZBattlefield || !strings.Contains(e.Counter, "entered_cloaked") {
				continue
			}
			o := g.E.G.Obj(e.Obj)
			if o == nil || o.Face() == nil {
				continue
			}
			if o.Face().IsCreature() {
				continue // the creature case TestETBReplacementFilterVeiledAscension pins
			}
			ft.cloaks++
			ft.cloaked[e.Obj] = true
		case events.CounterChange:
			if !strings.EqualFold(e.Counter, "Flying") || !ft.cloaked[e.Obj] {
				continue
			}
			ft.countered++
		}
	}
	return nil
}

// TestFaceDownEntryCreatureFilterFourSeat plays a pod whose seats each cast
// Veiled Ascension and cloak noncreature cards: 20 seeds, no divergence, and
// the mechanism reached -- at least one cloaked noncreature entered and
// received the flying counter from the replacement, so the test cannot pass
// with the entry filter never exercised.
func TestFaceDownEntryCreatureFilterFourSeat(t *testing.T) {
	corpusDirOrSkip(t)
	deck := facedownEntryDeck()
	tally := &facedownEntryTally{}
	play := fourSeatSetPlay(t, cardsettest.Synth(t, "tail-61"), [4][]string{deck, deck, deck, deck},
		nil, func(g *GorgeSide, _ Side) *Divergence { return tally.check(g, nil) }, nil,
		func(seed uint64) Policy { return combatPolicy(seed, nil) })
	const seeds = 20
	for seed := uint64(1); seed <= seeds; seed++ {
		tally.resetFirst()
		res := play(seed)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s last %s", seed, res.Div.At,
				res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	t.Logf("FourSeat: %d noncreature cloaks, %d of them granted the flying counter", tally.cloaks, tally.countered)
	if tally.cloaks == 0 {
		t.Fatal("no seat cloaked a noncreature card in 20 seeds: the entry filter was never exercised")
	}
	if tally.countered == 0 {
		t.Fatal("no cloaked noncreature entered with the flying counter: the face-down entry filter never matched")
	}
}
