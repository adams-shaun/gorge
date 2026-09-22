// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strconv"

func kwRavenous(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.148: "Ravenous (This creature enters with X +1/+1 counters on
	// it. If X is 5 or more, draw a card when it enters.)" Ravenous rides a
	// mana cost with an {X} component. Forge carries it as a single K: line
	// (no parameters on any corpus carrier) and expands it into two pieces,
	// which this mirrors:
	//
	//   - the counters are an ENTERS-WITH replacement (R:Event$ Moved,
	//     Destination$ Battlefield, ReplaceWith$ PutCounter with ETB$ True),
	//     not a triggered ability. This is load-bearing, not stylistic: a
	//     Ravenous creature with base toughness 0 -- Zoanthrope is exactly
	//     that (PT:0/0) -- would die to the CR 704.5f zero-toughness
	//     state-based action before an ETB trigger could resolve, so a
	//     trigger-based counter put loses the carrier outright. The
	//     replacement places the counters as the permanent enters, so the
	//     SBA sees the counters. This is the same shape kw:etbCounter
	//     (cards/kw_etbcounter.go) uses.
	//
	//   - the X>=5 draw is a ChangesZone self-entry trigger whose Draw body
	//     is gated on the same count through the shared SVar-condition
	//     engine (effects/conditions.go's ConditionCheckSVar$ +
	//     ConditionSVarCompare$, the Forge pair). The oracle says "draw a
	//     card when it enters", so a trigger is the right shape here.
	//
	// X is the {X} the cast paid, read by Count$xPaid from the moving object
	// (the replacement's Ctx.X) and, for the draw trigger, from the trigger's
	// source -- the permanent the spell became, the stack->battlefield move
	// preserving X. A permanent that entered any other way (reanimated,
	// blinked, dropped) reads 0, CR 107.3m.
	//
	// The corpus scripts all declare SVar:X:Count$xPaid, but the expansion is
	// self-contained: it mints its own count SVar (Count$xPaid) and keys both
	// halves on that, so it cannot depend on the carrier spelling X the way a
	// raw CounterNum$ X would.
	//
	// Idempotency: the two halves are guarded independently through the same
	// has() the other expanders use -- has("R", k) reads f.Repls for the
	// counters, has("T", k) reads f.Triggers for the draw, and both carry the
	// exact keyword line k as their KeywordLine tag -- so a second Link() of a
	// cached face (cards/registry.go re-links decoded faces so a newly added
	// expansion reaches an existing cache) cannot double-add either. The SVar
	// names key on i (the keyword index), the etbCounter/Affinity convention,
	// so two Ravenous lines on one face cannot collide.
	x := "__kwRavenousX" + strconv.Itoa(i)
	if !has("R", k) {
		put := "__kwRavenousPut" + strconv.Itoa(i)
		f.setSVar(put, "DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ "+x+" | ETB$ True")
		p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self" +
			" | ReplacementResult$ Updated | ReplaceWith$ " + put + " | Keyword$ Ravenous")
		p["KeywordLine"] = k
		f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
	}
	if !has("T", k) {
		draw := "__kwRavenousDraw" + strconv.Itoa(i)
		f.setSVar(draw, "DB$ Draw | NumCards$ 1 | ConditionCheckSVar$ "+x+" | ConditionSVarCompare$ GE5")
		p := parseParams("Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self" +
			" | Execute$ " + draw + " | Keyword$ Ravenous | TriggerDescription$ Ravenous")
		p["KeywordLine"] = k
		f.Triggers = append(f.Triggers, Trigger{Mode: p["Mode"], Params: p})
	}
	// The count SVar is shared by both halves, so it is set unconditionally:
	// a cached face that already carries one half must still resolve the
	// other's reference to it.
	f.setSVar(x, "Count$xPaid")
}

func init() { registerKeyword(kwRavenous, "Ravenous") }
