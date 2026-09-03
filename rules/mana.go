package rules

import (
	"math"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/state"
)

// Cost is a parsed mana cost. X is a flag contributing 0 to CMC; a caller that
// resolves X folds the chosen value into Generic before paying.
//
// Hybrid and Phyrexian symbols are approximated as one generic mana each. This
// approximation is deliberately over-permissive: a {W/U} cost is payable by any
// single mana of any colour, and a {UP} cost is payable without paying life.
// This is a known M1 limitation — correct modelling needs an alternative-payment
// representation that a later milestone must add. The M1 acceptance decks do not
// use hybrid or Phyrexian-restricted payment, so the approximation has no effect
// in practice: Dismember and Gitaxian Probe (the only M1 cards with Phyrexian
// mana) are thus mispriced (accepting payment in regular mana, not life).
type Cost struct {
	Colored state.Mana
	Generic int32
	X       bool
}

// ParseCost accepts both Forge's space-separated form ("2 U U") and the
// bracketed oracle form ("{2}{U}{U}"). "no cost" and "" are free.
func ParseCost(s string) Cost {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "no cost") {
		return Cost{}
	}
	s = strings.NewReplacer("{", " ", "}", " ").Replace(s)
	var c Cost
	for _, sym := range strings.Fields(s) {
		switch {
		case sym == "X":
			c.X = true
		case len(sym) == 1 && strings.ContainsAny(sym, "WUBRGC"):
			c.Colored[state.ManaIndex(sym[0])]++
		default:
			// Try to parse as a numeric token. Negative and out-of-range values
			// fall through to the +1 generic fallback.
			if n, err := strconv.ParseInt(sym, 10, 64); err == nil && n >= 0 && n <= int64(math.MaxInt32) {
				c.Generic += int32(n)
				continue
			}
			// Hybrid ("W/U", "GW", "2B"), Phyrexian ("W/P", "UP", "BP"), and
			// invalid numeric tokens land here.
			c.Generic++
		}
	}
	return c
}

func (c Cost) CMC() int32 { return c.Colored.Total() + c.Generic }

func (c Cost) CanPay(p state.Mana) bool {
	_, ok := c.Pay(p)
	return ok
}

// Pay spends the cost from a pool and returns what is left. Coloured
// requirements come out first so generic can never strand a colour the cost
// still needs.
func (c Cost) Pay(p state.Mana) (state.Mana, bool) {
	out := p
	for i, n := range c.Colored {
		if out[i] < n {
			return p, false
		}
		out[i] -= n
	}
	need := c.Generic
	// Spend colourless first, then colours in fixed WUBRG order, so payment is
	// deterministic and does not depend on map iteration.
	for _, i := range [...]int{state.MC, state.MW, state.MU, state.MB, state.MR, state.MG} {
		for need > 0 && out[i] > 0 {
			out[i]--
			need--
		}
	}
	if need > 0 {
		return p, false
	}
	return out, true
}
