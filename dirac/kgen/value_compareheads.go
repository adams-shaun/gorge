package kgen

import (
	"fmt"
	"strings"
)

// two Count$Compare operand heads.

// refValidCount lowers <head>$Valid <spec> over the arm's object ref: gorge
// evalRefProperty Valid (count_ref_property.go) counts the referenced
// objects matching the spec where they now are (kernel countRefMatches over
// RefTarget0..4; a moved target reads its LKI). Only Targeted lowers;
// Sacrificed is a gorge finding, the rest keep the head$rest error.
func (lc *lowerCtx) refValidCount(head, spec string, ref lRef) (lValue, error) {
	switch head {
	case "Targeted":
		f, err := lc.filter(spec)
		if err != nil {
			return lValue{}, err
		}
		return lValue{Kind: vRefCount, Ref: ref, F: f}, nil
	case "Sacrificed":
		// only the exact Card.IsSuspected spec is
		// read -- the sacrifice-time LKI designation (gorge
		// sacrificedValidCount, effects/count_ref.go). Every other spec
		// keeps the refusal.
		if strings.TrimSpace(spec) != "Card.IsSuspected" {
			return lValue{}, refuse("Sacrificed$Valid %q: only Card.IsSuspected reads the LKI designation", spec)
		}
		f, err := lc.filter(spec)
		if err != nil {
			return lValue{}, err
		}
		return lValue{Kind: vRefCount, Ref: ref, F: f}, nil
	}
	return lValue{}, fmt.Errorf("%s$Valid %s", head, spec)
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
