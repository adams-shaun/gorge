package rules

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/adams-shaun/gorge/state"
)

// CostPart is one non-mana cost component: Sac<N/Spec> (sacrifice N
// permanents matching Spec) or SubCounter<N/Kind> (remove N counters of
// Kind from the source).
type CostPart struct {
	N    int32
	Spec string
}

// Cost is a parsed cost. X counts how many "X" symbols appeared (almost
// always 0 or 1; WithX folds a chosen value into Generic once per symbol).
// Tap, Sac and SubCounter are non-mana components a cast or activation must
// satisfy separately from mana payment (rules/cast.go's castable and the
// cast-flow stages own that; Pay/CanPay below are mana-only, unchanged from
// before this cost grammar grew non-mana parts).
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
	Colored    state.Mana
	Generic    int32
	X          int
	Tap        bool
	Sac        []CostPart
	SubCounter []CostPart
}

// nonManaCost matches Sac<N/Spec> and SubCounter<N/Kind> tokens. Forge
// appends a human-readable "/description" after the spec and separates OR
// alternatives with ";"; the description may itself contain spaces (e.g.
// "Sac<1/Artifact;Creature/artifact or creature>"), which is why
// splitCostTokens keeps the whole <...> group atomic before nonManaCost ever
// sees it. The captured group only runs up to the first "/", so the trailing
// description is dropped right here; the ";" alternation is folded to ","
// (MatchesSpec's own separator) at the parse site. Ruling FL-54.
var nonManaCost = regexp.MustCompile(`^(Sac|SubCounter)<(\d+)/([^/>]+)(?:/[^>]*)?>$`)

// ParseCost accepts both Forge's space-separated form ("2 U U") and the
// bracketed oracle form ("{2}{U}{U}"). "no cost" and "" are free.
func ParseCost(s string) Cost {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "no cost") {
		return Cost{}
	}
	s = strings.NewReplacer("{", " ", "}", " ").Replace(s)
	var c Cost
	for _, sym := range splitCostTokens(s) {
		switch {
		case sym == "T":
			c.Tap = true
		case sym == "X":
			c.X++
		case len(sym) == 1 && strings.ContainsAny(sym, "WUBRGC"):
			c.Colored[state.ManaIndex(sym[0])]++
		default:
			if m := nonManaCost.FindStringSubmatch(sym); m != nil {
				n, err := strconv.ParseInt(m[2], 10, 64)
				if err != nil || n < 0 || n > int64(math.MaxInt32) {
					// A malformed Sac/SubCounter token degrades the same way
					// an unrecognised mana token does: one generic mana,
					// never a hard parse error.
					c.Generic = addClampedGeneric(c.Generic, 1)
					continue
				}
				// Fold Forge's ";" OR alternation into the "," MatchesSpec
				// already uses, so "Artifact;Creature" matches either.
				spec := strings.ReplaceAll(m[3], ";", ",")
				part := CostPart{N: int32(n), Spec: spec}
				if m[1] == "Sac" {
					c.Sac = append(c.Sac, part)
				} else {
					c.SubCounter = append(c.SubCounter, part)
				}
				continue
			}
			// Try to parse as a numeric token. Negative and out-of-range values
			// fall through to the +1 generic fallback.
			if n, err := strconv.ParseInt(sym, 10, 64); err == nil && n >= 0 && n <= int64(math.MaxInt32) {
				c.Generic = addClampedGeneric(c.Generic, n)
				continue
			}
			// Hybrid ("W/U", "GW", "2B"), Phyrexian ("W/P", "UP", "BP"), and
			// invalid numeric tokens land here.
			c.Generic = addClampedGeneric(c.Generic, 1)
		}
	}
	return c
}

// splitCostTokens splits a cost string on whitespace, but keeps each <...>
// group atomic so Forge's non-mana tokens -- whose trailing "/description"
// can contain spaces -- are not torn apart by a plain Fields split before
// nonManaCost can see them. Ruling FL-54.
func splitCostTokens(s string) []string {
	var out []string
	var cur strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
			cur.WriteRune(r)
		case r == '>':
			if depth > 0 {
				depth--
			}
			cur.WriteRune(r)
		case unicode.IsSpace(r) && depth == 0:
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

// addClampedGeneric adds n to the int32 generic count, saturating at
// math.MaxInt32 on top and refusing to go below zero, so no accumulation of
// numeric tokens (nor a WithX fold) can ever wrap Generic negative. Task 20.
func addClampedGeneric(v int32, n int64) int32 {
	total := int64(v) + n
	if total > math.MaxInt32 {
		return math.MaxInt32
	}
	if total < 0 {
		return 0
	}
	return int32(total)
}

func (c Cost) CMC() int32 { return c.Colored.Total() + c.Generic }

// WithX folds a chosen X value into Generic, once per X symbol the cost
// carried, then clears X: once a value is chosen, {X} is no longer a
// distinct requirement, it is simply that much more generic mana.
func (c Cost) WithX(x int32) Cost {
	c.Generic = addClampedGeneric(c.Generic, int64(c.X)*int64(x))
	c.X = 0
	return c
}

// Plus sums two costs (Kicker's own cost added to the card's printed cost):
// colours and generic add, X counts add, Tap ORs, and each side's
// non-mana parts concatenate.
func (c Cost) Plus(d Cost) Cost {
	for i := range c.Colored {
		c.Colored[i] += d.Colored[i]
	}
	c.Generic += d.Generic
	c.X += d.X
	c.Tap = c.Tap || d.Tap
	if len(d.Sac) > 0 {
		c.Sac = append(append([]CostPart(nil), c.Sac...), d.Sac...)
	}
	if len(d.SubCounter) > 0 {
		c.SubCounter = append(append([]CostPart(nil), c.SubCounter...), d.SubCounter...)
	}
	return c
}

// HasNonMana reports whether paying this cost takes more than mana.
func (c Cost) HasNonMana() bool {
	return c.Tap || len(c.Sac) > 0 || len(c.SubCounter) > 0
}

func (c Cost) CanPay(p state.Mana) bool {
	_, ok := c.Pay(p)
	return ok
}

// Pay spends the cost from a pool and returns what is left. Coloured
// requirements come out first so generic can never strand a colour the cost
// still needs. Mana-only: non-mana parts (Tap/Sac/SubCounter) are the cast
// flow's own job (rules/cast.go), never this function's.
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
