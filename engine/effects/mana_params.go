package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects/params"
)

// api:Mana's parameter compiler lives in the leaf package engine/effects/params
// (params/mana.go): the
// payment layer (engine/rules/pay) reads a mana ability's production there without
// importing effects. These are the effects spellings every existing caller
// uses, plus the two Amount$ evaluations that need the resolution's Host.

// ManaParams is one mana ability's production parameters (params.ManaParams).
type ManaParams = params.ManaParams

// ManaOf returns sa's compiled mana parameters (params.ManaOf).
func ManaOf(sa *cards.SA) *ManaParams { return params.ManaOf(sa) }

// ManaKnownKeys is a copy of the api:Mana known-key table, for the census
// check.
func ManaKnownKeys() []string { return params.ManaKnownKeys() }

// ManaAmountNum is effects.Num over p's compiled Amount$ (default def).
func ManaAmountNum(p *ManaParams, h Host, c *Ctx, def int32) int32 {
	return numText(h, c, p.Amount, def)
}

// ManaEachNum is effects.Num over p's compiled Each$ (def when absent).
func ManaEachNum(p *ManaParams, h Host, c *Ctx, def int32) int32 {
	if !p.Each.Present {
		return def
	}
	return numText(h, c, p.Each, def)
}

// ManaAmountResolvedStrict is effects.NumResolvedStrict over p's compiled
// Amount$.
func ManaAmountResolvedStrict(p *ManaParams, h Host, c *Ctx, def int32) (int32, bool) {
	return numResolvedStrictText(h, c, p.Amount, def)
}

// ManaAmountIsZero reports whether p's Amount$ resolves, by the same strict
// evaluator ManaAmountResolvedStrict uses, to exactly zero or less: a
// production known to add nothing, as opposed to one the evaluator can't
// resolve. An inline Count$ body is re-checked with EvalCountOK, which Strict
// does not do itself.
func ManaAmountIsZero(sa *cards.SA, h Host, c *Ctx) bool {
	p := ManaOf(sa)
	if p.AmountTrim == "" {
		return false
	}
	if p.AmountIsLit {
		return p.AmountLit <= 0
	}
	n, ok := ManaAmountResolvedStrict(p, h, c, 1)
	if !ok || n > 0 {
		return false
	}
	if ref := strings.TrimLeft(p.AmountTrim, "+-"); strings.HasPrefix(ref, "Count$") {
		_, evaluated := EvalCountOK(h, c, ref)
		return evaluated
	}
	return true
}
