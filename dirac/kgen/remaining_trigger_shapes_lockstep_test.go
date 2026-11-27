package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The remaining hosted-Commander trigger/static/delayed shapes (ticket
// gap-trigger-shapes-remaining). Each lowered card is played against gorge
// through the misc-shapes runner (miscRun: real corpus fillers, a table built
// through the ordinary tableOf path, gorge leading every decision) and its
// reach counter proves the shape ran. The cards that stay refused by name are
// pinned in TestRemainingTriggerShapesRefused.

// avengeReducedCasts counts the Avenge spells seat 0 resolved with fewer
// lands than its printed {4}{W}{W} needs (the lands it had played by the
// Resolve), so the {2} reduction was the only way to pay.
func avengeReducedCasts(g *state.Game, evs []events.Event) int {
	lands, n := 0, 0
	for _, ev := range evs {
		switch ev.Kind {
		case events.LandPlayed:
			if ev.Player == 0 {
				lands++
			}
		case events.Resolve:
			if o := g.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Avenge" && o.Controller == 0 && lands < 6 {
				n++
			}
		}
	}
	return n
}

// castFromGraveyard counts the casts of the named card from a graveyard (the
// PutOnStack of its object from the graveyard).
func castFromGraveyard(name string) func(*state.Game, []events.Event) int {
	return func(g *state.Game, evs []events.Event) int {
		n := 0
		for _, ev := range evs {
			if ev.Kind != events.PutOnStack || ev.From != state.ZGraveyard {
				continue
			}
			if o := g.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
				n++
			}
		}
		return n
	}
}

func remainingShapes() []miscShape {
	e := func(n string, c int) cardset.Entry { return cardset.Entry{Name: n, Count: c} }
	return []miscShape{
		// B's hasty goblins attack A every turn, so A's Avenge costs {2} less on A's
		// next turn; A has no creature to block and plays only Plains.
		{card: "Avenge", seeds: 40, min: 3, reached: avengeReducedCasts,
			matchups: [][2][]cardset.Entry{{
				{e("Avenge", 8), e("Plains", 32)},
				{e("Raging Goblin", 20), e("Mountain", 20)},
			}}},
		// Haakon cannot be cast from the hand: only the graveyard cast (after
		// Careful Study's discard or an opponent's burn) can put it in play, and
		// the battlefield Haakon lets Black Knight be cast from the graveyard.
		{card: "Haakon, Stromgald Scourge", seeds: 60, min: 3, reached: castFromGraveyard("Haakon, Stromgald Scourge"),
			matchups: haakonMatchups()},
		{card: "Haakon, Stromgald Scourge", seeds: 60, min: 1, reached: castFromGraveyard("Black Knight"),
			matchups: haakonMatchups()},
	}
}

func haakonMatchups() [][2][]cardset.Entry {
	e := func(n string, c int) cardset.Entry { return cardset.Entry{Name: n, Count: c} }
	return [][2][]cardset.Entry{{
		{e("Haakon, Stromgald Scourge", 4), e("Black Knight", 8), e("Careful Study", 4), e("Swamp", 14), e("Island", 10)},
		{e("Shock", 6), e("Hill Giant", 8), e("Grizzly Bears", 8), e("Mountain", 10), e("Forest", 8)},
	}}
}

// TestRemainingTriggerShapesHostedLockstep plays each lowered shape against
// gorge and asserts the shape's own counters.
func TestRemainingTriggerShapesHostedLockstep(t *testing.T) {
	for _, sh := range remainingShapes() {
		t.Run(sh.card, func(t *testing.T) {
			dirac.DebugAtkYouLastYes = 0
			div, reach := miscRun(t, sh)
			if div != 0 {
				t.Fatalf("%d games diverged", div)
			}
			if reach < sh.min {
				t.Fatalf("the pool reached the shape %d times, want at least %d: the lockstep proves nothing", reach, sh.min)
			}
			if sh.card == "Avenge" && dirac.DebugAtkYouLastYes == 0 {
				t.Fatalf("the kernel never counted an attacking opponent")
			}
			t.Logf("%s: %d reaches over %d seeds", sh.card, reach, sh.seeds)
		})
	}
}
