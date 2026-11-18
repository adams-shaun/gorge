// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

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
	cost := strings.TrimSpace(param)
	if strings.HasPrefix(cost, "PayLife<") {
		cost = "LifeTotalHalfUp"
	}
	f.addKeywordTrigger(head, k,
		"Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | TriggerZones$ Graveyard | ValidCard$ Creature.YouOwn+Other | TriggerDescription$ Recover "+cost,
		"DB$ Recover | Cost$ "+cost, has)
}

func init() { registerKeyword(kwRecover, "Recover") }
