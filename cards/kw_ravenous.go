// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strconv"

func kwRavenous(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.148: "Ravenous (This creature enters with X +1/+1 counters on
	// it. If X is 5 or more, draw a card when it enters.)" Ravenous rides a
	// mana cost with an {X} component. Forge carries it as a single K: line
	// (no parameters on any corpus carrier) and expands it into the ETB
	// counter put plus the conditional draw, exactly as CardFactoryUtil's
	// Ravenous case does; this expands the same way. The corpus scripts all
	// declare SVar:X:Count$xPaid, but the expansion is self-contained: it
	// mints its own count SVar (Count$xPaid) and keys both the counter put
	// and the draw gate on that, so it cannot depend on the carrier spelling
	// X the way a raw CounterNum$ X would.
	//
	// One ChangesZone self-entry trigger (the Squad/Afterlife shape) whose
	// body puts that many +1/+1 counters and then, as a SubAbility$, draws a
	// card gated on the same count via the shared SVar-condition engine
	// (effects/conditions.go's ConditionCheckSVar$ + ConditionSVarCompare$,
	// the Forge pair): the gate is met only when the count is >= 5. X is the
	// {X} paid for the cast that became this permanent, read by
	// Count$xPaid from the trigger's source (the permanent the spell became,
	// the stack->battlefield move preserving X); a permanent that entered
	// any other way (reanimated, blinked, dropped) reads 0, CR 107.3m. The
	// corpus carriers all have X in their mana cost; a hypothetical
	// param-bearing line keeps the head-only read (all 12 carriers carry the
	// bare head).
	//
	// addKeywordTrigger guards idempotency via the KeywordLine tag, so a
	// second Link() of a cached face cannot double-add the trigger. The SVar
	// names key on i (the keyword index), the etbCounter/Affinity
	// convention, so two Ravenous lines on one face cannot collide.
	if has("T", k) {
		return
	}
	x := "__kwRavenousX" + strconv.Itoa(i)
	draw := "__kwRavenousDraw" + strconv.Itoa(i)
	f.setSVar(x, "Count$xPaid")
	f.setSVar(draw, "DB$ Draw | NumCards$ 1 | ConditionCheckSVar$ "+x+" | ConditionSVarCompare$ GE5")
	f.addKeywordTrigger(head, k,
		"Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | TriggerDescription$ Ravenous",
		"DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ "+x+" | SubAbility$ "+draw, has)
}

func init() { registerKeyword(kwRavenous, "Ravenous") }
