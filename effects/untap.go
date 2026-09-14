package effects

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("Untap", effUntap) }

// effUntap implements "AB$ Untap" / "DB$ Untap": the listed objects untap.
// Targets act exactly as for Tap (effTap, combatfx.go): Forge's rule is that
// an ability that names targets acts on them and one that names none acts on
// its source, which Defined's empty-Defined arm already resolves -- so an
// untap activation with ValidTgts$ gets its targets from Ctx.Targets and an
// Untap sub with Defined$ Remembered/ReplacedCard acts on that set, each of
// the ticket's real cards through one of those two arms (Basalt Monolith's
// {3}: Untap this artifact is source-less and source-directed; Fabled
// Passage's and Baloth Prime's DB$ Untap chain off Remembered / no Defined).
//
// TryUntap applies CR 122.1d's stun replacement to every untap event. It is
// shared by ability effects and the turn-based untap step: a stun counter is
// removed instead of untapping the permanent. Keeping the replacement at this
// single event proposal point prevents the two untap sites from drifting.
func TryUntap(h Host, id state.ObjID) {
	o := h.Game().Obj(id)
	if o == nil || o.Zone != state.ZBattlefield || !o.Tapped {
		return
	}
	if o.Counter("STUN") > 0 {
		h.Emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "STUN", Amount: -1})
		return
	}
	h.Emit(events.Event{Kind: events.Untap, Obj: id})
}

// untapBattlefieldCondition implements Untap's Fabled Passage class: a
// ConditionPresent$ with no ConditionDefined$ counts battlefield objects.
// It stays local rather than widening Resolve's condition grammar for every
// unrelated API. Unknown predicates fail closed for this local supported form.
func untapBattlefieldCondition(h Host, c *Ctx, sa *cards.SA) bool {
	spec, ok := sa.Params["ConditionPresent"]
	if !ok || sa.Params["ConditionDefined"] != "" {
		return true
	}
	if len(UnknownPredicates(spec)) > 0 {
		return false
	}
	n := 0
	g := h.Game()
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o != nil && MatchesObjectCtx(g, spec, o, c.SpecContext(c.Controller)) {
				n++
			}
		}
	}
	cmp := sa.Params["ConditionCompare"]
	if cmp == "" {
		return n > 0
	}
	op, want, ok := parseConditionCompare(cmp)
	if !ok {
		return false
	}
	switch op {
	case "EQ":
		return n == want
	case "NE":
		return n != want
	case "LT":
		return n < want
	case "LE":
		return n <= want
	case "GT":
		return n > want
	case "GE":
		return n >= want
	}
	return false
}

// A permanent that is not on the battlefield, or already untapped, is skipped:
// untapping an untapped permanent is a no-op (CR 701.27a), and an object that
// left the battlefield mid-resolution must not be touched.
func effUntap(h Host, c *Ctx, sa *cards.SA) {
	if !untapBattlefieldCondition(h, c, sa) {
		return
	}
	for _, t := range Defined(h, c, sa) {
		if !t.IsPlayer {
			TryUntap(h, t.Obj)
		}
	}
}
