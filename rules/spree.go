package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
)

// Spree (CR 702.171) and Tiered are casting-option keywords: like Kicker or
// Surge they are read directly by rules rather than expanded into a trigger,
// replacement or activated ability, so there is no cards/keywords.go case.
// What they carry is a modal spell ("SP$ Charm") whose per-mode SVar bears a
// ModeCost$ -- the additional cost a chosen mode charges on top of the
// printed mana cost. Registering the heads keeps make report honest about the
// keyword being implemented (the casting-option set kw:Kicker/kw:Surge
// already lives in this package).
func init() {
	effects.RegisterNonAPI("kw:Spree", "kw:Tiered")
}

// modeCost resolves one Charm mode's per-mode additional cost from the SVar
// the mode names (Forge's ModeCost$ on the DB/SP line, e.g. Requisition
// Raid's three `DB$ ... | ModeCost$ 1` modes). ok=false when the mode has no
// ModeCost$ or names an SVar this face does not define. A cost token ParseCost
// cannot model fails closed (the replicateCost/twoPartKickerCosts direction)
// rather than being charged as degraded generic mana.
func modeCost(f *cards.Face, name string) (Cost, bool) {
	if f == nil {
		return Cost{}, false
	}
	sub := cards.ResolveSVar(f.SVars, strings.TrimSpace(name))
	if sub == nil {
		return Cost{}, false
	}
	raw := strings.TrimSpace(sub.Params["ModeCost"])
	if raw == "" {
		return Cost{}, false
	}
	c := ParseCost(raw)
	if len(c.Unknown) > 0 {
		return Cost{}, false
	}
	return c, true
}

// modeCostTotal sums the ModeCost$ of every chosen mode, one charge per
// occurrence: the exact additional cost a Spree/Tiered cast owes above its
// printed cost (CR 702.171b). A repeated mode (CanRepeatModes$) is charged
// once per pick, so the input is the answered ordered list, not a set. A mode
// with no ModeCost$ contributes nothing.
func modeCostTotal(f *cards.Face, names []string) Cost {
	var total Cost
	for _, name := range names {
		if c, ok := modeCost(f, name); ok {
			total = total.Plus(c)
		}
	}
	return total
}
