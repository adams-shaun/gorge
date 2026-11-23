package oracle

// The CR 616.1 entry-order ask of a plain OpReturn move (Y'shtola Rhul's
// "exile target creature you control, then return it to the battlefield under
// its owner's control"). A blinked Balamb Garden, SeeD Academy (enters tapped)
// beside The Wandering Minstrel ("lands you control enter untapped") is a
// competition gorge orders by asking the affected player; the plain return
// route had no park hook and stalled (StallEntryOrder).

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestEntryOrderPlainReturnReprosLockstep replays the recorded game: the
// kernel poses the order ask on the blink's return and replays clean.
func TestEntryOrderPlainReturnReprosLockstep(t *testing.T) {
	stdReproNamed(t, "entry_order_plain_return")
}

// TestEntryOrderPlainReturnMutation restores the route without the park hook
// (dirac.DebugEntryOrderNoReturnPark) and requires the recorded game to stall
// or diverge again.
func TestEntryOrderPlainReturnMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "entry_order_plain_return.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugEntryOrderNoReturnPark = true
	defer func() { dirac.DebugEntryOrderNoReturnPark = false }()
	res := replayDecodedRes(t, synthReg(t), globSet(t, r.Pool, r), r)
	if res.Div == nil && !strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("mutation produced no divergence and no kernel stall over %d recorded actions", len(r.Actions))
	}
	t.Logf("mutation: div=%v stall=%q", res.Div != nil, res.Stall)
}

// blinkLandTally counts, from gorge's log, a Balamb Garden returned from exile
// (a blink) while The Wandering Minstrel is on its controller's battlefield:
// the entry competition the park orders.
type blinkLandTally struct {
	last    int
	reached int
}

func (p *blinkLandTally) walk(ref *GorgeSide) {
	g := ref.E.G
	evs := ref.E.L.Events
	for ; p.last < len(evs); p.last++ {
		e := &evs[p.last]
		if e.Kind != events.MoveZone || e.From != state.ZExile || e.To != state.ZBattlefield {
			continue
		}
		ob := g.Obj(e.Obj)
		if ob == nil || ob.Face() == nil || ob.Face().Name != "Balamb Garden, SeeD Academy" {
			continue
		}
		for _, id := range g.Zone(state.ZBattlefield, ob.Controller) {
			if m := g.Obj(id); m != nil && m.Face() != nil && m.Face().Name == "The Wandering Minstrel" {
				p.reached++
				break
			}
		}
	}
}

// yshtolaBlinkChooser aims Y'shtola's trigger at a Balamb Garden (a Vehicle
// once transformed and crewed) and keeps the transform and crew activations
// coming.
func yshtolaBlinkChooser() func(*PickCtx) (Canon, bool) {
	return func(c *PickCtx) (Canon, bool) {
		if c.G == nil {
			return 0, false
		}
		d := c.G.Pending()
		if d == nil || d.Kind != decision.KTarget || !strings.Contains(d.Prompt, "for Y'shtola Rhul") {
			return 0, false
		}
		for _, o := range c.D.Options {
			if o.Kind() != ActTarget {
				continue
			}
			opt, ok := c.G.GorgeOption(o)
			if !ok || opt.Obj == 0 {
				continue
			}
			if ob := c.G.E.G.Obj(opt.Obj); ob != nil && ob.Face() != nil && strings.HasPrefix(ob.Face().Name, "Balamb Garden") {
				return o, true
			}
		}
		return 0, false
	}
}

// TestEntryOrderPlainReturnFourSeat blinks enters-tapped lands beside the
// untap body on a four-seat pod. No seed may diverge or stall, and the
// mechanism must be reached at least once.
func TestEntryOrderPlainReturnFourSeat(t *testing.T) {
	corpusDirOrSkip(t)
	set := cardsettest.Synth(t, "std-fin")
	deck := deckOf(6, "Y'shtola Rhul",
		deckOf(4, "The Wandering Minstrel",
			deckOf(10, "Balamb Garden, SeeD Academy",
				deckOf(8, "Forest", deckOf(12, "Island")...)...)...)...)
	var tally *blinkLandTally
	base := fourSeatSetPlay(t, set, [4][]string{deck, deck, deck, deck}, nil,
		func(g *GorgeSide, _ Side) *Divergence {
			if tally != nil {
				tally.walk(g)
			}
			return nil
		},
		func(g *GorgeSide) {
			if tally != nil {
				tally.walk(g)
			}
		},
		func(seed uint64) Policy {
			return developmentPolicy(seed, yshtolaBlinkChooser())
		})
	var reached int
	play := func(seed uint64) Result {
		tally = &blinkLandTally{}
		res := base(seed)
		reached += tally.reached
		return res
	}
	const seeds = 20
	fourSeatAtkClean(t, play, seeds)
	t.Logf("blinked enters-tapped lands beside the untap body: %d across %d seeds", reached, seeds)
	if reached == 0 {
		t.Fatalf("the mechanism was never reached in %d seeds: no Balamb Garden was blinked back beside The Wandering Minstrel", seeds)
	}
}
