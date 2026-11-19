// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwRecover(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.59: "Recover [cost] (When a creature is put into your graveyard
	// from the battlefield, you may pay [cost]. If you do, return this card
	// from your graveyard to your hand. Otherwise, exile this card.)"
	// Expanded into an ordinary ChangesZone trigger living in the graveyard
	// (the Infesting-Radroach shape: TriggerZones$ Graveyard, matched by the
	// shared zone gate). A creature you OWN dying is the printed "put into
	// your graveyard" oracle -- a card goes to its owner's graveyard, so the
	// ownership qualifier is the destination test; "+Other" keeps a creature
	// carrier's own death from reading as "a creature" (Garza's Assassin's
	// "another creature"). The pay-or-exile election is
	// engine/rules/recover.go's resolution-time flow. The corpus's life
	// spelling (Garza's Assassin's PayLife<X/half your life, rounded up>,
	// priced by its own SVar) folds to the LifeTotalHalfUp token the cost
	// parser accepts, so both spellings price identically.
	//
	// The fold is deliberately CONTENT-BLIND and only correct for the two
	// corpus carriers today: ANY `PayLife<...>` spelling becomes half your
	// life, rounded up, discarding the parameter's actual content. A future
	// carrier with a flat `PayLife<N>` cost would be silently priced as
	// half-your-life. If one appears, replace this fold with a reader that
	// dispatches on the parameter's body (a plain number stays a number,
	// the half-your-life spellings fold to LifeTotalHalfUp) and add its
	// behaviour test; do not widen this blind fold.
	cost := param
	if len(cost) >= 8 && cost[:8] == "PayLife<" {
		cost = "LifeTotalHalfUp"
	}
	f.addKeywordTrigger(head, k,
		"Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | TriggerZones$ Graveyard | ValidCard$ Creature.YouOwn+Other | TriggerDescription$ Recover "+cost,
		"DB$ Recover | Cost$ "+cost, has)
}

func init() { registerKeyword(kwRecover, "Recover") }
