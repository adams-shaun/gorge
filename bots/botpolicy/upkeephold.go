package botpolicy

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The upkeep hold: an opt-in candidate prune the search seats share
// (bots/azmcts Options.HoldUpkeep, bots/searchseat Options.HoldUpkeep), the
// gorge mirror of the search kernel's upkeep-hold rule, so an A/B reads the
// same rule on both engines. An instant-speed cast in the seat's own untap,
// upkeep or draw step with an empty stack has the same lands and the same
// response windows in main 1, and there it has seen the draw and the land
// drop. draft-zero docs/038 measured the rule null on gorge (50.2%,
// 48.4-52.0 over 500 pairs), so it is an arm, off by default.

// PrematureInstantCast reports whether "cast" option o is held by the upkeep
// rule: the seat is the active player (b.MyTurn), b.Step is untap, upkeep or
// draw, the stack is empty, o casts an instant-speed card (b.Cards'
// InstantSpeed, the class cast.go's window features read) from the hand, and
// the card is not exempt.
//
// Never held: a cast whose origin is not the hand (fromHand false: a
// flashback, foretell, plot or until-end-of-turn exile permission, whose
// window may close before main 1); any option that is not a cast
// (activations stay; AbilityWorthTaking is the ability rule); a card whose
// printed name is in exempt, by exact name, scanned in order. Exempt is
// empty by default; a card whose effect reads "this turn" before the draw
// is the kind to name there.
//
// The brief's shape read o's origin off the option; decision.Option carries
// none, so the caller passes it with the stack (HeldUpkeepCast reads both off
// the game).
func (b Board) PrematureInstantCast(o decision.Option, fromHand, stackEmpty bool, exempt []string) bool {
	if o.Kind != "cast" || !fromHand || !stackEmpty || !b.MyTurn {
		return false
	}
	switch b.Step {
	case state.StepUntap, state.StepUpkeep, state.StepDraw:
	default:
		return false
	}
	c := b.Cards.Get(o.Obj)
	if !c.InstantSpeed {
		return false
	}
	for _, n := range exempt {
		if n == c.PrintedName {
			return false
		}
	}
	return true
}

// HeldUpkeepCast is PrematureInstantCast with the origin and the stack read
// off g, the game b was built from: o's object in its hand zone, g's stack
// empty.
func (b Board) HeldUpkeepCast(g *state.Game, o decision.Option, exempt []string) bool {
	if o.Kind != "cast" {
		return false
	}
	obj := g.Obj(o.Obj)
	return b.PrematureInstantCast(o, obj != nil && obj.Zone == state.ZHand, len(g.Stack) == 0, exempt)
}
