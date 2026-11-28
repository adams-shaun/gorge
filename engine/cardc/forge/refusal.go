package forge

import (
	"fmt"
	"strings"
)

// RefusalKind says why a reader declined a token.
type RefusalKind uint8

const (
	RefuseNone RefusalKind = iota
	// RefuseEmpty is an empty filter or value.
	RefuseEmpty
	// RefuseEachForm is a filter that starts with "EACH ".
	RefuseEachForm
	// RefuseUnknownWord is a base that is no known base or type word.
	RefuseUnknownWord
	// RefuseBaseAbility is a base naming abilities, not objects.
	RefuseBaseAbility
	// RefuseBaseReferent is a base naming a context object (the target, a
	// remembered card, the source).
	RefuseBaseReferent
	// RefuseUnknownPredicate is a predicate outside the table.
	RefuseUnknownPredicate
	// RefuseUnknownKeyword is a keyword outside ir's keyword list.
	RefuseUnknownKeyword
	// RefuseUnknownCounter is a counter kind outside ir's counter list.
	RefuseUnknownCounter
	// RefuseBoundNamesSVar is a numeric bound that is not an integer.
	RefuseBoundNamesSVar
	// RefuseNeedsReference is a predicate naming a context object or player
	// the table cannot reference.
	RefuseNeedsReference
	// RefuseNestedFilter is a token holding an inner filter.
	RefuseNestedFilter
	// RefuseUnknownHead is a value head outside the head table.
	RefuseUnknownHead
	// RefuseUnknownOperator is a value operator outside the operator table.
	RefuseUnknownOperator
	// RefuseNeedsCondition is a value that branches on a game condition.
	RefuseNeedsCondition
	// RefuseSVarCycle is an SVar chain that revisits a name or runs past
	// maxSVarDepth names.
	RefuseSVarCycle
	// RefuseSVarMissing is a name the card's SVar table does not hold.
	RefuseSVarMissing
	// RefuseNotANumber is a value text, or an SVar body, that is no numeric
	// form (an ability, a trigger, a filter).
	RefuseNotANumber
	// RefuseWrongSubject is a reference that can only denote the other
	// subject (a player where an object is wanted, or the reverse).
	RefuseWrongSubject
	// RefuseUnknownReferent is a reference word outside the atom and
	// projection tables.
	RefuseUnknownReferent
	// RefuseNeedsAbilityRef is a reference naming an ability or a spell
	// ability on the stack (the triggered spell ability, a parent ability).
	RefuseNeedsAbilityRef
	// RefuseValidForm is a "Valid <filter>" reference whose inner filter
	// LowerFilter refuses; the atom is the inner refusal's "kind:atom".
	RefuseValidForm
	// RefuseNeedsMemoryKind is a store word with no ir.Memory value.
	RefuseNeedsMemoryKind
	refusalKindCount
)

// firstValueRefusal and lastValueRefusal bound the kinds only LowerValue
// gives; firstRefRefusal and lastRefRefusal bound those only LowerRef gives.
// The kinds before firstValueRefusal are LowerFilter's.
const (
	firstValueRefusal = RefuseUnknownHead
	lastValueRefusal  = RefuseNotANumber
	firstRefRefusal   = RefuseWrongSubject
	lastRefRefusal    = RefuseNeedsMemoryKind
)

var refusalKindNames = [...]string{
	RefuseNone:             "none",
	RefuseEmpty:            "empty",
	RefuseEachForm:         "each-form",
	RefuseUnknownWord:      "unknown-word",
	RefuseBaseAbility:      "base-ability",
	RefuseBaseReferent:     "base-referent",
	RefuseUnknownPredicate: "unknown-predicate",
	RefuseUnknownKeyword:   "unknown-keyword",
	RefuseUnknownCounter:   "unknown-counter",
	RefuseBoundNamesSVar:   "bound-names-svar",
	RefuseNeedsReference:   "needs-reference",
	RefuseNestedFilter:     "nested-filter",
	RefuseUnknownHead:      "unknown-head",
	RefuseUnknownOperator:  "unknown-operator",
	RefuseNeedsCondition:   "needs-condition",
	RefuseSVarCycle:        "svar-cycle",
	RefuseSVarMissing:      "svar-missing",
	RefuseNotANumber:       "not-a-number",
	RefuseWrongSubject:     "wrong-subject",
	RefuseUnknownReferent:  "unknown-referent",
	RefuseNeedsAbilityRef:  "needs-ability-ref",
	RefuseValidForm:        "valid-form",
	RefuseNeedsMemoryKind:  "needs-memory-kind",
}

// String returns the lower-kebab name of k.
func (k RefusalKind) String() string {
	if k < refusalKindCount {
		return refusalKindNames[k]
	}
	return fmt.Sprintf("RefusalKind(%d)", uint8(k))
}

// ParseRefusalKind is the inverse of String.
func ParseRefusalKind(s string) (RefusalKind, bool) {
	for k := RefuseNone + 1; k < refusalKindCount; k++ {
		if refusalKindNames[k] == s {
			return k, true
		}
	}
	return RefuseNone, false
}

// Refusal names the first token a reader could not read.
type Refusal struct {
	Kind RefusalKind
	// Atom is the base or predicate token refused, each digit run folded to "N".
	Atom string
}

// Error is "<kind>: <atom>".
func (r *Refusal) Error() string { return r.Kind.String() + ": " + r.Atom }

// refuse builds a Refusal for tok, folding its digit runs.
func refuse(k RefusalKind, tok string) *Refusal { return &Refusal{Kind: k, Atom: foldDigits(tok)} }

// foldDigits replaces each run of ASCII digits in s with "N".
func foldDigits(s string) string {
	var b strings.Builder
	inRun := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			if !inRun {
				b.WriteByte('N')
			}
			inRun = true
			continue
		}
		inRun = false
		b.WriteByte(c)
	}
	return b.String()
}
