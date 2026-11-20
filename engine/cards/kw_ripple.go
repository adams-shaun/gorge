// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// kwRipple expands K:Ripple (CR 702.60a) into the triggered ability the
// keyword prints: "When you cast this spell, you may reveal the top N cards
// of your library. If you reveal cards this way, you may cast any of those
// cards with the same name as this spell without paying their mana costs.
// Put all revealed cards not cast this way on the bottom of your library in
// any order."
//
//   - the trigger is the ordinary SpellCast self-trigger the other
//     cast-time keywords use (Storm: Mode$ SpellCast | ValidCard$ Card.Self
//     | TriggerZones$ Stack), with OptionalDecider$ You carrying the whole
//     ability's "may" (CR 603.5): a declined may places the ability but
//     resolves nothing, so the library is untouched either way;
//   - the body is DB$ Ripple | Num$ <N> (engine/effects/ripple.go), which
//     reveals min(N, library), offers each same-name card for a free cast
//     and bottoms the rest in an order the controller chooses;
//   - a spell cast by the ripple is itself a spell with ripple, so its own
//     printed (or granted) instance triggers separately (CR 702.60b) --
//     both routes fire on the ordinary PutOnStack walk.
//
// The parameter rides the minted body verbatim (the corpus prints exactly
// K:Ripple:<n>; measured at the corpus pin): a blank or unpriceable tail
// degrades at RESOLUTION, where Num's grammar answers 0 for an
// unresolvable count and the DB$ Ripple body no-ops (the documented Num
// convention), never a guess.
func kwRipple(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	f.addKeywordTrigger(head, k,
		"Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | OptionalDecider$ You | TriggerDescription$ Ripple",
		"DB$ Ripple | Num$ "+param+" | TriggerDescription$ Ripple", has)
}

func init() { registerKeyword(kwRipple, "Ripple") }
