// RegisterNonAPI for the corpus's bare keyword line printed by Chorus of the
// Conclave (the exact-sentence registration pattern of kw_untap_choice.go): the
// whole sentence is the head, so the supported set is keyed on the sentence
// itself and a corpus spelling that drifts fails closed. The behaviour lives
// in engine/rules/chorus_cost.go (the amount ask) and pay.AppendAnyAmountGrant
// (the entry counters); engine/rules's keyword_registration_test.go names the
// behaviour test that proves it.

package effects

func init() {
	RegisterNonAPI("kw:As an additional cost to cast creature spells, you may pay any amount of mana. If you do, that creature enters with that many additional +1/+1 counters on it.")
}
