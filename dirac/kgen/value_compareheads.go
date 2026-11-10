package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/effects"
)

// two Count$Compare operand heads.

// refValidCount lowers <head>$Valid <spec> over the arm's object ref: gorge
// evalRefProperty Valid (count_ref_property.go) counts the referenced
// objects matching the spec where they now are (kernel countRefMatches over
// RefTarget0..4; a moved target reads its LKI). Targeted and TriggeredCard
// (the triggering object where it now is, gorge's refTargets read) lower;
// Sacrificed is a gorge finding, the rest keep the head$rest error.
func (lc *lowerCtx) refValidCount(head, spec string, ref lRef) (lValue, error) {
	switch head {
	case "Targeted", "TriggeredCard":
		f, err := lc.filter(spec)
		if err != nil {
			return lValue{}, err
		}
		return lValue{Kind: vRefCount, Ref: ref, F: f}, nil
	case "Sacrificed":
		// Card.IsSuspected reads the sacrifice-time LKI designation;
		// Card.<creature type> / Creature.<creature type> read the
		// sacrifice-time derived type line (gorge sacrificedValidCount,
		// effects/count_ref.go). Every other spec keeps the refusal.
		if strings.TrimSpace(spec) == "Card.IsSuspected" {
			f, err := lc.filter(spec)
			if err != nil {
				return lValue{}, err
			}
			return lValue{Kind: vRefCount, Ref: ref, F: f}, nil
		}
		if f, ok := lc.sacrificedTypeFilter(spec); ok {
			// N 1 marks the sacrifice-time type read, so the kernel can tell
			// it from a ConditionDefined$ Sacrificed filter (RefMatches) over
			// the live moved card (dirac/gen.go anySacSub, value_countheads.go).
			return lValue{Kind: vRefCount, Ref: ref, F: f, N: 1}, nil
		}
		return lValue{}, refuse("Sacrificed$Valid %q: only Card.<creature type>, Creature.<creature type> or Card.IsSuspected read the sacrifice-time snapshot", spec)
	}
	return lValue{}, fmt.Errorf("%s$Valid %s", head, spec)
}

// sacrificedTypeFilter parses exactly Card.<creature type> or
// Creature.<creature type>: one clause, no other property, the word a creature
// type of gorge's vocabulary. The kernel reads the LKI snapshot's derived
// creature-type mask (value_countheads.go sacrificedLKICount).
func (lc *lowerCtx) sacrificedTypeFilter(spec string) (*lFilter, bool) {
	head, _, _ := strings.Cut(strings.TrimSpace(spec), ".")
	if head != "Card" && head != "Creature" {
		return nil, false // a bare subtype head ("Human") is not one of the two forms
	}
	f, err := parseFilter(spec, lc.known, lc.p.isTypeWord)
	if err != nil || f.Players != 0 || f.Each || len(f.Clauses) != 1 {
		return nil, false
	}
	c := f.Clauses[0]
	if (c.Types != 0 && c.Types != tCreature) || c.Subtype == "" || !effects.CreatureTypeWords(c.Subtype) {
		return nil, false
	}
	if c.TypesAll != 0 || c.NotTypes != 0 || c.Name != "" || c.NotSubtype != "" || c.Subtype2 != "" ||
		c.Ctrl != whoAny || c.Owner != whoAny || c.Flags != 0 || c.CmpStat != cmpNone ||
		c.CmpOp != 0 || c.CmpN != 0 || c.CmpX != "" || c.CmpV != nil || c.CmpCounter != "" ||
		c.Colors != "" || c.NotColors != "" || c.BasePermanent || c.SpellBase || c.Unblocked {
		return nil, false
	}
	return f, true
}

// replacedCardValue lowers ReplacedCard$<rest>: gorge knownDefinedTargets
// binds ReplacedCard to the replaced object; CardManaCost is its
// state.ManaValue (no stack X: the entrant is not on the stack). The kernel
// reads s.g.replaced inside an ETB replacement's program (ExMVOf RefReplaced).
func (lc *lowerCtx) replacedCardValue(rest string) (lValue, error) {
	if rest != "CardManaCost" || !lc.replEnter {
		return lValue{}, refuse("ReplacedCard$%s outside an ETB replacement", rest)
	}
	return lValue{Kind: vMVOf, Ref: lRef{Kind: rReplaced}}, nil
}

// compareInlineBody resolves a Count$Compare operand that names no SVar:
// gorge resolveCountOperand (count_operand.go) takes the token itself as an
// inline expression. Only a <head>$<rest> token is read.
func compareInlineBody(name string) (string, bool) {
	return name, strings.Contains(name, "$")
}
