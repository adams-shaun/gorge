// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwSquad(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.66a: "As an additional cost to cast this spell, you may pay
	// [cost] any number of times. When this creature enters, create that many
	// tokens that are copies of it." The corpus scripts carry only the
	// K:Squad:<cost> line, so the expansion supplies the ETB half.
	//
	// One ChangesZone self-entry trigger (the Afterlife shape) whose effect
	// mints one token copy of the entering creature PER SQUAD PAYMENT. The
	// payment count is the cast-time provenance the cast flow records
	// (rules/cast.go's squadAsk + payCast, the FlagSquadPaid trailing
	// CastInfo), read here through Count$SquadPaid -- the Replicate keyword's
	// Count$ReplicatePaid pattern. Count$SquadPaid resolves against the
	// trigger's SOURCE, which is the permanent the cast spell became, so a
	// cast that paid nothing (the plain cast, or a token copy that was never
	// cast) reads 0 and effCopyPermanent's NumCopies <= 0 arm mints nothing.
	//
	// CopyPermanent mints each copy's battlefield entry as a genuine
	// ChangesZone-matchable MoveZone, so the copies' own entries are observed
	// by every "a creature enters" trigger exactly like an ordinary cast --
	// and each copy carries the printed Squad keyword, so its own entry
	// trigger fires with Count$SquadPaid 0 (it was never cast) and creates no
	// further copies.
	//
	// The colon parameter is the per-payment cost, priced by the cast flow
	// through squadCost (rules/cast.go): a cost ParseCost cannot model is
	// withheld there, so the expansion is inert for such a face rather than
	// paying a degraded cost. addKeywordTrigger guards idempotency via the
	// KeywordLine tag, so a second Link() of a cached face cannot double-add
	// the trigger.
	f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | TriggerDescription$ Squad",
		"DB$ CopyPermanent | Defined$ Self | NumCopies$ Count$SquadPaid", has)
}

func init() { registerKeyword(kwSquad, "Squad") }
