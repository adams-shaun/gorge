package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
)

func init() { Register("ReplaceEffect", effReplaceEffect) }

// effReplaceEffect changes the event currently being replaced. Forge names
// the event field in VarName$ and supplies its replacement value in VarValue$.
// A ReplaceCount$ body -- directly (Torture Pit: VarValue$
// ReplaceCount$DamageAmount/Plus.2) or behind an SVar name (Fiery
// Emancipation: VarValue$ X, SVar:X:ReplaceCount$DamageAmount/Thrice) --
// expresses its new amount in terms of the HELD event's own amount, which
// only the host reading the in-flight event can resolve, so it is handed over
// unresolved. Every other value form is a plain number in the replacement
// source's context, which Num evaluates (an unresolvable expression degrades
// to zero, and the host's ReplaceEvent then leaves the event untouched rather
// than erasing the damage).
func effReplaceEffect(h Host, c *Ctx, sa *cards.SA) {
	name := sa.Params["VarName"]
	if name == "" {
		return
	}
	raw := strings.TrimSpace(sa.Params["VarValue"])
	if c != nil && c.SVars != nil {
		if body, ok := c.SVars[raw]; ok && strings.HasPrefix(body, "ReplaceCount$") {
			raw = body
		}
	}
	if strings.HasPrefix(raw, "ReplaceCount$") {
		h.ReplaceEvent(name, raw, 0)
		return
	}
	h.ReplaceEvent(name, raw, Num(h, c, sa, "VarValue", 0))
}
