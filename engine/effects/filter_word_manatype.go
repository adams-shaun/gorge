package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// canProduceSameManaTypeContextBase rewrites Forge's context-base spelling
// `<referent>.canProduceSameManaTypeWith+<type>` (Mana Web's
// `TriggeredCard.canProduceSameManaTypeWith+Land`) to the candidate's own
// base with the predicate kept: the trailing type is the candidate's type
// (the brief's "the base is the trailing type"), and the predicate compares
// the candidate with SpecContext.TriggerCard. The rewrite is name-specific,
// exactly like sameNameContextBase: any other head or token order is left to
// the ordinary grammar and fails closed as an unknown head.
func canProduceSameManaTypeContextBase(base, rest string) (string, string, bool) {
	if base != "TriggeredCard" {
		return "", "", false
	}
	ps := strings.Split(rest, "+")
	n := len(ps)
	if n < 2 || ps[n-2] != "canProduceSameManaTypeWith" || ps[n-1] == "" {
		return "", "", false
	}
	return ps[n-1], strings.Join(ps[:n-1], "+"), true
}

// producibleTypesShare reports whether two objects' printed mana abilities
// could produce at least one common type of mana (Mana Web's "could produce
// any type of mana that land could produce"). producibleSymbols is the one
// producible-type census the reflected mana abilities already read, so a
// blank/Any production, a Combo list and a plain rune list including {C} all
// agree with the rest of the engine. An object that produces nothing shares
// no type, fail closed.
func producibleTypesShare(a, b *state.Object) bool {
	if a == nil || b == nil {
		return false
	}
	as, bs := producibleSymbols(a), producibleSymbols(b)
	if as == "" || bs == "" {
		return false
	}
	for i := 0; i < len(as); i++ {
		if strings.IndexByte(bs, as[i]) >= 0 {
			return true
		}
	}
	return false
}
