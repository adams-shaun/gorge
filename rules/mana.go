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

// ManaPair is one two-colour hybrid symbol: both A and B are WUBRG letters,
// and either one spells the pip (CR 107.4e).
type ManaPair struct{ A, B byte }

// Cost is a parsed cost. X counts how many "X" symbols appeared (almost
// always 0 or 1; WithX folds a chosen value into Generic once per symbol).
// Tap, Sac and SubCounter are non-mana components a cast or activation must
// satisfy separately from mana payment (rules/cast.go's castable and the
// cast-flow stages own that; Pay/CanPay below are mana-only, unchanged from
// before this cost grammar grew non-mana parts).
//
// Hybrid and Phyrexian symbols are no longer flattened to generic. A hybrid
// pip (GW) is recorded in Hybrid as the pair of colours it accepts; a
// Phyrexian pip (UP) is recorded in Phyrexian as its colour (CR 107.4f: pay
// that colour OR two life). Both are “announcement” costs: which half of a
// hybrid and whether a Phyrexian pip is paid with life is a player choice at
// cast time (CR 601.2b), not something a parser decides. Because a Phyrexian
// pip may be paid with life, the mana-only Pay/CanPay below cannot fully own
// it; rule/cast.go's payment stage resolves the announced choice and spends
// against both the pool and the payer's life (see Cost.payable).
type Cost struct {
	Colored    state.Mana
	Generic    int32
	X          int
	Hybrid     []ManaPair
	Phyrexian  []byte
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
		case isHybrid(sym):
			c.Hybrid = append(c.Hybrid, hybridPair(sym))
		case isPhyrexian(sym):
			c.Phyrexian = append(c.Phyrexian, phyrexianColor(sym))
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
			// An unrecognised symbol (including a malformed hybrid/Phyrexian
			// token) degrades to one generic mana, never a hard parse error.
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

// isHybrid reports whether sym is a two-colour hybrid pip: either the
// slash form ("W/U") or the concatenated form ("GW", "WB"). A second
// character of 'P' is Phyrexian, not hybrid, and is handled by
// isPhyrexian. Both letters must be distinct WUBRG colours (a doubled
// letter, "WW", is not a hybrid — it is a script typo and degrades to
// generic).
func isHybrid(sym string) bool {
	var a, b byte
	if len(sym) == 3 && sym[1] == '/' {
		a, b = sym[0], sym[2]
	} else if len(sym) == 2 && sym[1] != 'P' {
		a, b = sym[0], sym[1]
	} else {
		return false
	}
	return a != b && strings.ContainsRune("WUBRG", rune(a)) && strings.ContainsRune("WUBRG", rune(b))
}

// hybridPair normalises a hybrid symbol to its two colours as a ManaPair.
// sym is guaranteed a hybrid by isHybrid.
func hybridPair(sym string) ManaPair {
	if len(sym) == 3 && sym[1] == '/' {
		return ManaPair{A: sym[0], B: sym[2]}
	}
	return ManaPair{A: sym[0], B: sym[1]}
}

// isPhyrexian reports whether sym is a Phyrexian pip: a WUBRG colour
// followed by 'P', either slash ("W/P") or concatenated ("UP", "BP").
// CR 107.4f: pay that colour OR two life.
func isPhyrexian(sym string) bool {
	if len(sym) == 3 && sym[1] == '/' {
		return strings.ContainsRune("WUBRG", rune(sym[0])) && sym[2] == 'P'
	}
	if len(sym) != 2 {
		return false
	}
	return sym[1] == 'P' && strings.ContainsRune("WUBRG", rune(sym[0]))
}

// phyrexianColor returns the colour letter of a Phyrexian pip. sym is
// guaranteed a Phyrexian pip by isPhyrexian.
func phyrexianColor(sym string) byte {
	return sym[0]
}

func (c Cost) CMC() int32 {
	return c.Colored.Total() + c.Generic + int32(len(c.Hybrid)) + int32(len(c.Phyrexian))
}

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
	if len(d.Hybrid) > 0 {
		c.Hybrid = append(append([]ManaPair(nil), c.Hybrid...), d.Hybrid...)
	}
	if len(d.Phyrexian) > 0 {
		c.Phyrexian = append(append([]byte(nil), c.Phyrexian...), d.Phyrexian...)
	}
	if len(d.Sac) > 0 {
		c.Sac = append(append([]CostPart(nil), c.Sac...), d.Sac...)
	}
	if len(d.SubCounter) > 0 {
		c.SubCounter = append(append([]CostPart(nil), c.SubCounter...), d.SubCounter...)
	}
	return c
}

// commanderTaxFor composes base with the CR 903.8 commander tax: an
// additional generic {2} -- on top of everything else -- for each previous
// time id has been cast from the command zone this game. It is the ONE place
// a command-zone cast's extra cost is composed, and BOTH sides of the
// two-sided gate apply it to the same base: the legality offer (the
// command-zone walk in rules/legal.go calls commanderTaxFor(base) before
// castable) and the payment (rules/cast.go's beginCast applies
// commanderTaxFor to the cost it resolves). Because both derive the taxed
// cost from this same function over
// the same board, an offered command-zone cast and the cost it charges are
// structurally incapable of disagreeing -- the no-progress spin
// stalledCastLimit bounds is exactly what that disagreement used to cause.
//
// base is expected to be the already-reduced output of adjustedCost (kicker/
// surge/flashback recast it afterwards, but a card in the command zone is
// only ever cast here by its plain cost). Applying the tax to base means cost
// reductions land BEFORE it and never spill onto it: an additional cost is
// outside a plain "cost to cast" reduction, exactly how Kicker's own
// additional {N} composes in beginCast. Colored requirements are untouched.
//
// The Commander-format gate is explicit, not incidental: outside the format
// -- even when a card sits in the (usually empty) command zone -- base passes
// through unchanged. A card that is not its owner's commander, or a commander
// no longer in the command zone, is likewise untaxed.
func (e *Engine) commanderTaxFor(p state.PlayerID, id state.ObjID, base Cost) Cost {
	if e.format != FormatCommander {
		return base
	}
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZCommand {
		return base
	}
	for k, cid := range e.G.Players[p].Commanders {
		if cid != id {
			continue
		}
		if n := e.G.Players[p].CmdCasts[k]; n > 0 {
			base.Generic += 2 * int32(n)
		}
		return base
	}
	return base
}

// HasNonMana reports whether paying this cost takes more than mana.
func (c Cost) HasNonMana() bool {
	return c.Tap || len(c.Sac) > 0 || len(c.SubCounter) > 0
}

// Priceable reports whether the mana-only payment path (Pay, and therefore
// payMana) can actually charge every part of this cost. Pay charges only
// Colored and Generic: an {X} component that has not been folded into
// Generic by WithX is not a drain the pool can be asked to satisfy -- a cost
// "X" parses as {Generic:0, X:1}, which Pay would satisfy from an EMPTY
// pool -- and a non-mana part (Tap/Sac/SubCounter) is likewise invisible to
// Pay, which handles mana only. Such a cost is unpriceable by the payment
// API and must be DECLINED, never silently priced at zero. This is the
// predicate I-5 routes through: every component ParseCost collapses into a
// shape Pay cannot charge -- an SVar-sourced X, a cast-time-chosen X, a
// Sac/SubCounter/Tap part -- travels through the same predicate rather than
// a `if cost == "X"` special case.
//
// This is deliberately NOT consulted by Cost.Pay itself. The cast flow folds
// a chosen X via WithX and settles its non-mana parts itself
// (rules/cast.go), so Pay INEVITABLY sees an unfolded X or a non-mana part
// as a normal intermediate there; rejecting those inside Pay would break
// ordinary casting. Priceable is the "is this a chargeable mana-only cost"
// question the mid-resolution unless-pay answer must ask before trusting the
// pool.
func (c Cost) Priceable() bool {
	return c.X == 0 && !c.HasNonMana() && len(c.Hybrid) == 0 && len(c.Phyrexian) == 0
}

// pip is one coloured-or-flexible demand inside a cost's mana part: the set
// of acceptable colours, and whether the pip may alternatively be paid with
// two life (a Phyrexian pip). n is always 1 for the pips this engine builds
// from a cost; the field exists so a caller that expands a multi-count
// coloured requirement can reuse the same struct.
type pip struct {
	colors [2]byte
	n      int
	lifeOK bool
}

// costPips expands a cost's Colored, Hybrid and Phyrexian parts into a flat
// pip list, in that order (exact colours first, then hybrids, then
// Phyrexian). A coloured pip accepts exactly its own colour; a hybrid accepts
// either of its pair; a Phyrexian pip accepts its colour or two life.
func (c Cost) costPips() []pip {
	var out []pip
	// The coloured slots including the colourless one: a plain {C} pip is a
	// strict colourless requirement generic must not satisfy by stealing the
	// pool's only colourless, so it is reserved like any coloured pip.
	for _, letter := range []byte{'W', 'U', 'B', 'R', 'G', 'C'} {
		for n := c.Colored[state.ManaIndex(letter)]; n > 0; n-- {
			out = append(out, pip{colors: [2]byte{letter, letter}, n: 1})
		}
	}
	for _, pair := range c.Hybrid {
		out = append(out, pip{colors: [2]byte{pair.A, pair.B}, n: 1})
	}
	for _, letter := range c.Phyrexian {
		out = append(out, pip{colors: [2]byte{letter, letter}, n: 1, lifeOK: true})
	}
	return out
}

// resolveMana finds A concrete payment of the cost's mana part from pool and
// the payer's life, preferring to spend coloured pool mana over life for a
// Phyrexian pip and preferring the first colour of a hybrid pair, so the
// assignment is deterministic. It returns the pool with the coloured pips
// spent, the life spent (each Phyrexian pip paid by life costs 2), and
// whether the whole cost is payable (generic left satisfied from the
// remainder). The generic requirement is paid last from whatever the pips
// left, so coloured mana is never spent on generic while a pip still needs
// it.
func (c Cost) resolveMana(pool state.Mana, life int32) (state.Mana, int32, bool) {
	pips := c.costPips()
	rem := pool
	lifeSpent := int32(0)
	var rec func(i int) bool
	rec = func(i int) bool {
		if i == len(pips) {
			return rem.Total() >= c.Generic
		}
		p := pips[i]
		// Try each acceptable colour, in the order given. For a hybrid this
		// prefers A over B; for a single-colour pip A==B so it is just once.
		for _, col := range p.colors {
			di := state.ManaIndex(col)
			if rem[di] > 0 {
				rem[di]--
				if rec(i + 1) {
					return true
				}
				rem[di]++
			}
		}
		if p.lifeOK && life >= 2 {
			life -= 2
			lifeSpent += 2
			if rec(i + 1) {
				return true
			}
			lifeSpent -= 2
			life += 2
		}
		return false
	}
	if !rec(0) {
		return pool, 0, false
	}
	// The search found a pip assignment that leaves enough total mana; deduct
	// the generic requirement from that remainder, preferring colourless then
	// colours in fixed WUBRG order so payment is deterministic. Generic can
	// be paid by any leftover mana, so a total >= Generic always suffices.
	need := c.Generic
	for _, i := range [...]int{state.MC, state.MW, state.MU, state.MB, state.MR, state.MG} {
		for need > 0 && rem[i] > 0 {
			rem[i]--
			need--
		}
	}
	return rem, lifeSpent, true
}

// payable reports whether the cost's mana part can be paid by pool and the
// payer's current life (a Phyrexian pip may be paid with two life). This is
// the offering gate's feasibility question, and the real answer to "is there
// ANY way this cost can be paid right now" -- the same resolveMana the
// payment stage uses, so an offered cost and the cost it charges can never
// disagree.
func (c Cost) payable(pool state.Mana, life int32) bool {
	_, _, ok := c.resolveMana(pool, life)
	return ok
}

func (c Cost) CanPay(p state.Mana) bool {
	// Pool-only feasibility, no life offered: a hybrid must be paid by one of
	// its colours in the pool, a Phyrexian pip by its colour. This is the
	// pure pricing question the corpus invariants ask, and it never treats a
	// hybrid as generic nor lets colourless `pay` it.
	_, _, ok := c.resolveMana(p, 0)
	return ok
}

// Pay spends the cost from a pool and returns what is left. Coloured
// requirements come out first so generic can never strand a colour the cost
// still needs; hybrid pips take one of their pair and Phyrexian pips their
// colour (pool-only -- the cast flow's payMana handles the life half and
// passes a fully-resolved cost here). Mana-only: non-mana parts
// (Tap/Sac/SubCounter) are the cast flow's own job (rules/cast.go), never
// this function's.
func (c Cost) Pay(p state.Mana) (state.Mana, bool) {
	// Pool-only: no life is offered, so a Phyrexian pip is paid by its colour
	// (the cast flow's payMana handles the life half and passes a fully
	// resolved cost here). resolveMana already reserves the coloured pips and
	// deducts generic, so the returned pool is fully spent.
	out, _, ok := c.resolveMana(p, 0)
	return out, ok
}
