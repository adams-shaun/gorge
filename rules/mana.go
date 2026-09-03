package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/state"
)

// Cost is a parsed mana cost. Hybrid and Phyrexian symbols are folded into
// generic for M1: the decks in scope contain none, and treating them as generic
// is permissive rather than wrong-in-a-way-that-blocks-play.
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
			if n, err := strconv.Atoi(sym); err == nil {
				c.Generic += int32(n)
				continue
			}
			// Hybrid ("W/U") and Phyrexian ("W/P") land here.
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
