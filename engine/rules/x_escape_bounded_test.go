package rules

import (
	"math"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// xEscapeSpecs covers every way a spec can read a mana value: not at all, each
// comparator on either side of the announced values, parity, and those joined
// by a conjunction, a negation or an alternative, so a match's short circuit
// can hide a comparison at one X and read it at another.
var xEscapeSpecs = []string{
	"Card",
	"Card.Creature",
	"Card.nonCreature",
	"Card.cmcEven",
	"Card.cmcOdd",
	"Card.cmcLE3",
	"Card.cmcLT3",
	"Card.cmcLE6",
	"Card.cmcGE4",
	"Card.cmcGT4",
	"Card.cmcEQ3",
	"Card.cmcEQ5",
	"Card.cmcNE3",
	"Card.cmcNOT5",
	"Card.cmcM20",
	"Card.cmcM21",
	"Card.!cmcLE3",
	"Card.!cmcEven",
	"Card.nonCreature+cmcGE4",
	"Card.Creature+cmcGE4",
	"Card.cmcEven+cmcGE4",
	"Card.cmcOdd+cmcLE5",
	"Card.cmcLE5+!cmcEQ3",
	"Card.Creature,Card.cmcLE2",
	"Card.cmcEven,Card.cmcLE6",
	"Card.cmcOdd,Card.cmcEven",
	"Card.cmcEven+cmcLE4,Card.cmcOdd+cmcLE7",
}

// slowXEscape is the walk xAnnouncementEscapes replaced: every announcement up
// to last, an unpayable one skipped rather than ending the walk.
func slowXEscape(e *Engine, sv staticView, spec string, p state.PlayerID, id state.ObjID, cost Cost, pool *state.Mana, last int32) bool {
	printed := e.faceCompiledCost(e.G.Obj(id).Face()).Cost
	for x := max(cost.XMin, 1); x <= last; x++ {
		if !e.xAnnouncementPayable(p, id, cost, x, pool) {
			continue
		}
		if !e.xAnnouncementMatches(sv, spec, id, printed.WithX(x).CMC(), false, nil) {
			return true
		}
	}
	return false
}

// xEscapeBoard is p0 holding the corpus card rel, a graveyard of gy cards (the
// Delve credit), and p1 controlling Voice of Victory, whose static supplies
// the view each spec is matched under.
func xEscapeBoard(t *testing.T, rel string, gy int) (*Engine, state.ObjID, staticView) {
	t.Helper()
	e := handEngine(t, card(t, corpusCardText(t, rel)))
	id := e.G.Zone(state.ZHand, 0)[0]
	battlePerm(t, e, 1, corpusCardText(t, "v/voice_of_victory.txt"))
	statics := e.activeStatics("CantBeCast")
	if len(statics) == 0 {
		t.Fatal("Voice of Victory registered no CantBeCast static")
	}
	var yard []state.ObjID
	for range gy {
		o := e.G.AddObject(mustCorpusCardT(t, "Island"), 0)
		o.Zone = state.ZGraveyard
		yard = append(yard, o.ID)
	}
	e.G.SetZone(state.ZGraveyard, 0, yard)
	return e, id, statics[0]
}

// TestXEscapesMatchesSlowWalkOnSmallPools holds the early stops to the full
// walk for every X a small pool can pay: each spec, one {X} and two ({X}{X}),
// with and without Delve credit and XMin, against the real pool and an explicit
// one. A colour the pips need going missing is an X-independent refusal.
func TestXEscapesMatchesSlowWalkOnSmallPools(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, rel string
		gy        int
	}{
		{"one X", "m/mind_spring.txt", 0},
		{"one X, Delve", "l/logic_knot.txt", 0},
		{"one X, Delve credit", "l/logic_knot.txt", 3},
		{"two X, Delve credit", "e/empty_the_pits.txt", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e, id, sv := xEscapeBoard(t, tc.rel, tc.gy)
			base := e.faceCompiledCost(e.G.Obj(id).Face()).Cost
			var pools []state.Mana
			for n := range int32(13) {
				var u, mixed state.Mana
				u[state.ManaIndex('U')] = n
				mixed[state.ManaIndex('U')], mixed[state.ManaIndex('B')] = 2, 4
				mixed[state.ManaIndex('C')] = n
				pools = append(pools, u, mixed)
			}
			for _, spec := range xEscapeSpecs {
				for _, xMin := range []int32{0, 2} {
					cost := base
					cost.XMin = xMin
					for _, pool := range pools {
						last := pool.Total() + e.delveCredit(0, id, math.MaxInt32)
						want := slowXEscape(e, sv, spec, 0, id, cost, &pool, last)
						if got := e.xAnnouncementEscapes(sv, spec, 0, id, cost, &pool, false); got != want {
							t.Errorf("%q XMin %d pool %v: priced answer %v, the full walk %v", spec, xMin, pool, got, want)
						}
						e.G.Players[0].Pool = pool
						want = slowXEscape(e, sv, spec, 0, id, cost, nil, last)
						if got := e.xAnnouncementEscapes(sv, spec, 0, id, cost, nil, false); got != want {
							t.Errorf("%q XMin %d floating %v: answer %v, the full walk %v", spec, xMin, pool, got, want)
						}
						e.G.Players[0].Pool = state.Mana{}
					}
				}
			}
		})
	}
}

// TestXEscapesOnUnboundedPoolMatchesWalkOfSmallX: the bound an open source
// prices (potentialUnbounded in every colour) answers what walking the small
// announcements answers, since no spec here compares a number past them, and
// an unpayable non-mana part ends the walk rather than running to the bound.
func TestXEscapesOnUnboundedPoolMatchesWalkOfSmallX(t *testing.T) {
	t.Parallel()
	e, id, sv := xEscapeBoard(t, "m/mind_spring.txt", 0)
	cost := e.faceCompiledCost(e.G.Obj(id).Face()).Cost
	unbounded := state.Mana{}
	for i := range unbounded {
		unbounded[i] = potentialUnbounded
	}
	for _, spec := range xEscapeSpecs {
		want := slowXEscape(e, sv, spec, 0, id, cost, &unbounded, 32)
		if got := e.xAnnouncementEscapes(sv, spec, 0, id, cost, &unbounded, false); got != want {
			t.Errorf("%q: answer %v, walking the small X answers %v", spec, got, want)
		}
	}
	unpayable := cost.Plus(ParseCost("Sac<1/Creature>"))
	if e.xAnnouncementEscapes(sv, "Card.cmcEven", 0, id, unpayable, &unbounded, false) {
		t.Error("an unpayable sacrifice found an escape: no announcement can pay it")
	}
}

// TestXEscapesBoundedOnHugePool: the posed decision's potential walk returns
// promptly over a bound far past any board, the spin kfuzz hung on (Voice of
// Victory and Spectral Denial with an open mana source reached X = 588,502 and
// still climbing, TDM scratch pair 1366). Its offers are the rule's: a
// restriction on every spell binds at every X, and one on even mana values
// has an escape at X = 1. Not parallel: the budget is wall time.
func TestXEscapesBoundedOnHugePool(t *testing.T) {
	float := func(n int32) func(*testing.T, *Engine) {
		return func(t *testing.T, e *Engine) {
			e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: n})
		}
	}
	openSource := func(t *testing.T, e *Engine) {
		onBoardCard(t, e, 0, card(t, corpusCardText(t, "d/dragonstorm_globe.txt")))
	}
	for _, tc := range []struct {
		name, rel, static string
		active            state.PlayerID
		setup             func(*testing.T, *Engine)
		wantTotal         int32
		wantCast          bool
	}{
		{"open source, every spell barred", "s/spectral_denial.txt", "v/voice_of_victory.txt", 1, openSource, math.MaxInt32, false},
		{"600000 floating, every spell barred", "s/spectral_denial.txt", "v/voice_of_victory.txt", 1, float(600000), 600000, false},
		{"open source, even mana values barred", "m/mind_spring.txt", "v/void_winnower.txt", 0, openSource, math.MaxInt32, true},
		{"600000 floating, even mana values barred", "m/mind_spring.txt", "v/void_winnower.txt", 0, float(600000), 600000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := handEngine(t, card(t, corpusCardText(t, tc.rel)))
			id := e.G.Zone(state.ZHand, 0)[0]
			battlePerm(t, e, 1, corpusCardText(t, tc.static))
			tc.setup(t, e)
			e.G.Active = tc.active
			type walked struct {
				mana state.Mana
				opts []decision.Option
			}
			done := make(chan walked, 1)
			go func() {
				e.askPriority(0)
				mana, opts := e.potentialWalkOf(0, false)
				done <- walked{mana, opts}
			}()
			select {
			case w := <-done:
				if got := w.mana.Total(); got != tc.wantTotal {
					t.Fatalf("hypothetical pool totals %d, want %d", got, tc.wantTotal)
				}
				cast := false
				for _, o := range w.opts {
					cast = cast || (o.Kind == "cast" && o.Obj == id)
				}
				if cast != tc.wantCast {
					t.Fatalf("potential walk offers the cast = %v, want %v", cast, tc.wantCast)
				}
			case <-time.After(time.Second):
				t.Fatal("the potential walk did not return within 1s")
			}
		})
	}
}
