package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// Num resolves a numeric parameter. A literal is used directly; anything else
// is treated as an SVar name whose body is a Count$ expression. An expression
// this build does not model evaluates to zero rather than to the default, so
// the failure mode is "the card did nothing" rather than "the card did
// something arbitrary".
func Num(h Host, c *Ctx, sa *cards.SA, key string, def int32) int32 {
	if c == nil {
		c = &Ctx{}
	}
	raw, ok := sa.Params[key]
	if !ok {
		return def
	}
	raw = strings.TrimSpace(raw)
	if n, err := strconv.Atoi(raw); err == nil {
		return int32(n)
	}
	if c.SVars != nil {
		if body, ok := c.SVars[raw]; ok {
			return EvalCount(h, c, body)
		}
	}
	if raw == "X" {
		return c.X
	}
	return 0
}

// EvalCount evaluates a "Count$..." expression. The grammar in the corpus is a
// head, an optional space-separated argument, and an optional "/Op" suffix.
func EvalCount(h Host, c *Ctx, expr string) int32 {
	if h == nil || c == nil {
		return 0
	}
	expr = strings.TrimSpace(expr)
	body, ok := strings.CutPrefix(expr, "Count$")
	if !ok {
		if n, err := strconv.Atoi(expr); err == nil {
			return int32(n)
		}
		return 0
	}
	body, op, hasOp := strings.Cut(body, "/")
	n := evalCountBody(h, c, strings.TrimSpace(body))
	if hasOp {
		n = applyCountOp(n, op)
	}
	return n
}

func evalCountBody(h Host, c *Ctx, body string) int32 {
	g := h.Game()
	head, arg, _ := strings.Cut(body, " ")
	arg = strings.TrimSpace(arg)

	switch head {
	case "xPaid":
		return c.X
	case "YourLifeTotal":
		if c.Controller < 0 || int(c.Controller) >= len(g.Players) {
			return 0
		}
		return g.Players[c.Controller].Life
	case "PlayerCountPlayers":
		return int32(g.AliveCount())
	case "PlayerCountOpponents":
		return int32(g.AliveCount() - 1)
	case "RememberedSize":
		return int32(len(c.Remembered))
	case "CardPower":
		if o := g.Obj(c.Source); o != nil && o.Face() != nil {
			return int32(o.Face().Power()) + o.Counter("P1P1")
		}
		return 0
	case "CardToughness":
		if o := g.Obj(c.Source); o != nil && o.Face() != nil {
			return int32(o.Face().Toughness()) + o.Counter("P1P1")
		}
		return 0
	}

	// Valid / ValidZone forms count objects in a zone matching a filter.
	if zone, ok := countZone(head); ok {
		var n int32
		for _, p := range g.AliveFrom(0) {
			for _, id := range g.Zone(zone, p) {
				if MatchesSpecFrom(g, arg, id, c.Controller, c.Source) {
					n++
				}
			}
		}
		return n
	}
	return 0
}

// countZone maps a Count$ head to the zone it scopes over.
func countZone(head string) (state.Zone, bool) {
	switch head {
	case "Valid":
		return state.ZBattlefield, true
	case "ValidHand":
		return state.ZHand, true
	case "ValidGraveyard":
		return state.ZGraveyard, true
	case "ValidLibrary":
		return state.ZLibrary, true
	case "ValidExile":
		return state.ZExile, true
	case "ValidStack":
		return state.ZStack, true
	}
	return 0, false
}

func applyCountOp(n int32, op string) int32 {
	switch {
	case strings.HasPrefix(op, "Plus"):
		if v, err := strconv.Atoi(op[len("Plus"):]); err == nil {
			return n + int32(v)
		}
	case strings.HasPrefix(op, "Minus"):
		if v, err := strconv.Atoi(op[len("Minus"):]); err == nil {
			return n - int32(v)
		}
	case op == "Twice":
		return n * 2
	case op == "HalfDown":
		return n / 2
	case op == "HalfUp":
		return (n + 1) / 2
	case op == "Negative":
		return -n
	}
	return n
}

// SetSVars binds a copy of the SVar table to a context. A nil input leaves
// c.SVars nil, preserving the defensive-copy convention established by
// copyTargets in context.go.
func SetSVars(c *Ctx, sv map[string]string) {
	if sv == nil {
		c.SVars = nil
		return
	}
	copied := make(map[string]string, len(sv))
	for k, v := range sv {
		copied[k] = v
	}
	c.SVars = copied
}
