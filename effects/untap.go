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
// A permanent that is not on the battlefield, or already untapped, is skipped:
// untapping an untapped permanent is a no-op (CR 701.27a), and an object that
// left the battlefield mid-resolution must not be touched. Stun counters are
// NOT consulted here: CR 701.27b's "remove a stun counter instead" is an
// untap-step replacement this build does not model (see AGENTS.md), and an
// ability untap never routes through it.
func effUntap(h Host, c *Ctx, sa *cards.SA) {
	for _, t := range Defined(h, c, sa) {
		if t.IsPlayer {
			continue
		}
		o := h.Game().Obj(t.Obj)
		if o == nil || o.Zone != state.ZBattlefield || !o.Tapped {
			continue
		}
		h.Emit(events.Event{Kind: events.Untap, Obj: o.ID})
	}
}
