package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// thisTurnCastReferent is the `sharesNameWith ThisTurnCast` spelling (Twinning
// Glass's `Card.nonLand+sharesNameWith ThisTurnCast`).
const thisTurnCastReferent = "sharesNameWith ThisTurnCast"

// thisTurnCastFor binds the per-turn cast ledger for a spec that reads the
// ThisTurnCast referent, nil for every other spec. The read is unfiltered
// ("a spell that was cast this turn": any caster), the same cast walk the
// Count$ThisTurnCast_<spec> head reads through HostCastLedger.
func thisTurnCastFor(h Host, c *Ctx, spec string) []state.ObjID {
	if !strings.Contains(spec, thisTurnCastReferent) {
		return nil
	}
	return h.EachSpellCastThisTurnMatching(c.Controller, "Card", 0, c.TriggerContext)
}
