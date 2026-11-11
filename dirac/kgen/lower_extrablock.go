package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// extrablock: "can block an additional creature" (CanBlockAmount$ N) and
// "can block any number of creatures" (CanBlockAny$ True) on a Continuous
// static (CR 509.1a). gorge reads both (rules/combat/extrablock.go
// BlockCapacity). dirac lowers exactly one shape, the creature's own
// S:Mode$ Continuous | AffectedDefined$ Self | CanBlockAmount$ 1 (Ghastbark
// Twins): CardDef.ExtraBlocks, a blocker of at most two attackers whose
// controller divides its combat damage between them (CR 510.1d,
// combat_extrablock.go). A Self static of amount 1 bounds every blocker's
// capacity at two, because no other lowered card raises it. Every other
// shape (CanBlockAny$, another amount, an Affected$ grant, a Condition$, an
// extra key) is refused by name.

// extraBlockStatic lowers the self CanBlockAmount$ 1 static. It reports false
// (and no error) for any other shape, which extraBlockRefusal then refuses.
func (lc *lowerCtx) extraBlockStatic(s cards.Static) (bool, error) {
	ps := newParams(s.Params)
	ps.consume("Mode", "Description")
	// the card parser reads AffectedDefined$ Self as Affected$ Card.Self
	if !ps.is("Affected", "Card.Self") && !ps.is("AffectedDefined", "Self") || !ps.is("CanBlockAmount", "1") {
		return false, nil
	}
	ps.consume("Affected", "AffectedDefined", "CanBlockAmount")
	if len(ps.unread()) > 0 {
		return false, nil
	}
	lc.out.ExtraBlocks = 1
	return true, nil
}

// extraBlockRefusal refuses a static carrying either extra-block key.
func extraBlockRefusal(sp *params) error {
	for _, key := range []string{"CanBlockAmount", "CanBlockAny"} {
		if v, ok := sp.get(key); ok {
			return refuse("%s$ %q: a blocker of several attackers (CR 509.1a, 510.1d) outside the self CanBlockAmount$ 1 shape is unmodelled in dirac", key, v)
		}
	}
	return nil
}
