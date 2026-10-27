package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// conditionManaSpentMet evaluates ConditionManaSpent$ / ConditionManaNotSpent$
// ("if {G} was spent to cast this spell", CR 601.2h / 608.2c) against the
// per-colour spend the payment recorded on the host (state.Object.
// ManaColorSpent, which survives stack -> battlefield for a trigger that
// resolves from the permanent). A value is a space-separated colour list
// meaning ALL listed colours were spent (Forge's hasAllColors). A source-less
// context or a zero vector means nothing was spent: ManaSpent is false and
// ManaNotSpent true, Forge's null-castSA arms. resolved is false for a token
// naming no colour.
func conditionManaSpentMet(c *Ctx, g *state.Game, spentKey, notSpentKey string) (met, resolved bool) {
	var spent state.Mana
	if o := g.Obj(c.Source); o != nil {
		spent = o.ManaColorSpent
	}
	met = true
	for _, k := range []struct {
		val string
		not bool
	}{{spentKey, false}, {notSpentKey, true}} {
		if k.val == "" {
			continue
		}
		all := true
		for _, tok := range strings.Fields(k.val) {
			l := colourLetter(tok)
			if l == 0 {
				return false, false
			}
			if spent[state.ManaIndex(l)] <= 0 {
				all = false
			}
		}
		if all == k.not {
			met = false
		}
	}
	return met, true
}
