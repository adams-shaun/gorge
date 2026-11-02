// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwOffspring(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.175a: "You may pay an additional [cost] as you cast this spell"
	// and "When this permanent enters, if its offspring cost was paid, create
	// a token that's a copy of it, except it's 1/1." The "if" is an
	// intervening if (CR 603.4): a plain cast puts nothing on the stack.
	// The corpus scripts carry only the K:Offspring:<cost> line, so the
	// expansion supplies the ETB half -- the Squad expansion's shape
	// (engine/cards/kw_squad.go), with three deliberate differences:
	//
	//   - Offspring is optional and paid AT MOST ONCE, so the cast flow
	//     offers a "cast (offspring)" option that pays the base cost PLUS
	//     this additional cost (never a count ask), and the provenance is a
	//     bool: Count$OffspringPaid reads 1 when the cost was paid and 0 for
	//     the plain cast (or a token copy that was never cast).
	//   - The copy is a 1/1, not a full-size copy: the oracle says "create a
	//     1/1 token copy of it", so SetPower$/SetToughness$ force base
	//     power/toughness 1 as a real layer-7 characteristic modification on
	//     the mint (the CopyPermanent SetPower$/SetToughness$ family).
	//   - The token copy is NOT a cast spell, so it carries no
	//     Count$OffspringPaid provenance of its own; the copy enters with the
	//     same printed K:Offspring line, but its own entry reads 0 and does
	//     not trigger (the Squad recursion guard).
	//
	// CopyPermanent mints each copy's battlefield entry as a genuine
	// ChangesZone-matchable MoveZone, so the copy's entry is observed by
	// every "a creature enters" trigger exactly like an ordinary cast.
	//
	// The colon parameter is the additional cost, priced by the cast flow
	// through offspringCost (engine/rules/cast.go): a cost ParseCost cannot model is
	// withheld there, so the expansion is inert for such a face rather than
	// paying a degraded cost. addKeywordTrigger guards idempotency via the
	// KeywordLine tag, so a second Link() of a cached face cannot double-add
	// the trigger.
	f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | CheckSVar$ Count$OffspringPaid | TriggerDescription$ Offspring",
		"DB$ CopyPermanent | Defined$ Self | NumCopies$ Count$OffspringPaid | SetPower$ 1 | SetToughness$ 1", has)
}

func init() { registerKeyword(kwOffspring, "Offspring") }
