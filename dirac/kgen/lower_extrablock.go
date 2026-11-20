package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// extrablock: "can block an additional creature" (CanBlockAmount$ N) and
// "can block any number of creatures" (CanBlockAny$ True) on a Continuous
// static (CR 509.1a). gorge reads both (rules/combat/extrablock.go
// BlockCapacity). dirac lowers two shapes of amount 1: the creature's own
// S:Mode$ Continuous | AffectedDefined$ Self | CanBlockAmount$ 1 (Ghastbark
// Twins), CardDef.ExtraBlocks, and the grant S:Mode$ Continuous | Affected$
// <creatures> | CanBlockAmount$ 1 (Brave the Sands), kop.StatExtraBlock over
// the affected filter. Either makes a blocker of at most two attackers whose
// controller divides its combat damage between them (CR 510.1d,
// combat_extrablock.go); a blocker that two such sources reach (capacity
// three) stops the kernel at StallExtraBlockCap.
//
// CanBlockAny$ True lowers the same two shapes with unbounded capacity
// (kop.ExtraBlocksAny: Wall of Glare; the grant's Static.A, plus the
// creature an Aura enchants, Entangler): a blocker of every attacker it can
// block, its damage divided among all of them (combat_blockany.go). A
// Pump's CanBlockAny$ (Give No Ground, Valor Made Real) is not lowered:
// gorge's Pump does not read it (BlockCapacity reads only a Continuous
// static), so the cards would play differently. Every other shape (another
// amount, a Condition$, an extra key) is refused by name.

// extraBlockStatic lowers the self CanBlockAmount$ 1 static. It reports false
// (and no error) for any other shape, which extraBlockRefusal then refuses.
func (lc *lowerCtx) extraBlockStatic(s cards.Static) (bool, error) {
	ps := newParams(s.Params)
	ps.consume("Mode", "Description")
	// the card parser reads AffectedDefined$ Self as Affected$ Card.Self
	key, amount := "CanBlockAmount", 1
	_, hasAny := ps.get("CanBlockAny")
	_, hasAmount := ps.get("CanBlockAmount")
	switch {
	case ps.is("CanBlockAmount", "1") && !hasAny:
	case ps.is("CanBlockAny", "True") && !hasAmount:
		key, amount = "CanBlockAny", int(kop.ExtraBlocksAny)
	default:
		return false, nil
	}
	if !ps.is("Affected", "Card.Self") && !ps.is("AffectedDefined", "Self") {
		return lc.extraBlockGrant(ps, key, amount)
	}
	ps.consume("Affected", "AffectedDefined", key)
	if len(ps.unread()) > 0 {
		return false, nil
	}
	lc.out.ExtraBlocks = amount
	return true, nil
}

// extraBlockRefusal refuses a static carrying either extra-block key.
func extraBlockRefusal(sp *params) error {
	for _, key := range []string{"CanBlockAmount", "CanBlockAny"} {
		if v, ok := sp.get(key); ok {
			return refuse("%s$ %q: a blocker of several attackers (CR 509.1a, 510.1d) outside the lowered amount-1 and any-number shapes is unmodelled in dirac", key, v)
		}
	}
	return nil
}

// extraBlockGrant lowers the Affected$ grant of one extra block: only the
// Affected$ filter (over the static's controller) and CanBlockAmount$ 1 may
// be present, since gorge BlockCapacity reads the static's gate and an
// AffectedDefined$ scope too.
func (lc *lowerCtx) extraBlockGrant(ps *params, key string, amount int) (bool, error) {
	v, ok := ps.get("Affected")
	if _, scoped := ps.m["AffectedDefined"]; !ok || scoped {
		return false, nil
	}
	ps.consume("Affected", key)
	if len(ps.unread()) > 0 {
		return false, nil
	}
	f, err := lc.filter(v)
	if err != nil {
		return false, err
	}
	if f == nil || len(f.Clauses) == 0 || f.Players != 0 {
		return false, nil
	}
	for _, c := range f.Clauses {
		// only the lockstep-verified shape lowers: creatures the static's
		// controller controls, no attachment, counter or other qualifier
		// (Brave the Sands, High Ground); Echo Circlet's equipped creature,
		// Foriysian Totem's self filter and Cenn's Tactician's counter gate
		// stay refused until a lockstep game reaches them
		if blockAnyEnchanted(&c, amount) {
			continue // Entangler: the enchanted creature
		}
		if c.Flags != 0 || c.CmpStat != cmpNone || c.Ctrl != whoYou || c.Types != tCreature || c.Subtype != "" || c.NotTypes != 0 || c.Colors != "" || c.NotColors != "" {
			return false, nil
		}
	}
	lc.out.Statics = append(lc.out.Statics, lStatic{Kind: "ExtraBlock", F: f, BlockAmount: amount})
	return true, nil
}

// blockAnyEnchanted: the clause is "Creature.EnchantedBy" of a CanBlockAny$
// grant (Entangler's AffectedDefined$ Enchanted | Affected$ Creature, which
// the card parser folds into it): the creature the Aura enchants, whoever
// controls it, nothing else qualifying it.
func blockAnyEnchanted(c *lClause, amount int) bool {
	return amount == int(kop.ExtraBlocksAny) && c.Flags == fEnchantedBy && c.Ctrl == whoAny && c.Owner == whoAny &&
		c.CmpStat == cmpNone && c.Types == tCreature && c.Subtype == "" && c.NotTypes == 0 && c.Colors == "" && c.NotColors == ""
}
