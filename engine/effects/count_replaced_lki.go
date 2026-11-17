package effects

import "strings"

// replacedLKIProps maps each property the LKI ref models to whether it is the
// power read (true) or the toughness read (false).
var replacedLKIProps = map[string]bool{"CardPower": true, "CardToughness": false}

// evalReplacedCardLKIOK answers "ReplacedCardLKI$<prop>[/Op]": the last-known
// power or toughness of the object a zone-change replacement is acting on
// (Ravenous Slime's X:ReplacedCardLKI$CardPower). rules' replCtx captures the
// derived P/T when the replacement matches (Ctx.Repl.ReplacedPower), because
// the body's own move has already cleared the object's counters and
// continuous effects by the time the count evaluates. Without a capture the
// live object is read. Only the P/T properties resolve on the LKI ref:
// Kalain's ReplacedCardLKI$CastTotalManaSpent keeps failing closed to 0, and
// the plain ReplacedCard ref is untouched (it resolves through
// refTargets/definedSpec, so Thunderous Velocipede's ReplacedCard$CardManaCost
// still reads the live mana value). Outside a replacement the answer is 0.
func evalReplacedCardLKIOK(h Host, c *Ctx, body string, depth int) (int32, bool) {
	prop, op, hasOp := strings.Cut(body, "/")
	prop = strings.TrimSpace(prop)
	isPower, known := replacedLKIProps[prop]
	if !known {
		return 0, false
	}
	var n int32
	if id := c.Repl.Replaced; id != 0 {
		switch {
		case c.Repl.ReplacedPTValid && isPower:
			n = c.Repl.ReplacedPower
		case c.Repl.ReplacedPTValid:
			n = c.Repl.ReplacedToughness
		case h.Game().Obj(id) == nil:
		case isPower:
			n = h.Power(id)
		default:
			n = h.Toughness(id)
		}
	}
	if hasOp {
		n = applyCountOpOperand(h, c, n, op, depth)
	}
	return n, true
}
