// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// kwSoulshift expands Soulshift N (CR 702.46a): "When this permanent is put
// into a graveyard from the battlefield, you may return target Spirit card
// with mana value N or less from your graveyard to your hand." One
// ChangesZone death trigger (the Afterlife shape) whose body is a
// graveyard-to-hand ChangeZone. No TgtZone$ is needed: a single concrete
// Origin$ Graveyard implies the target census zone
// (rules.originImpliedTargetZone), the Wrenn and Six shape. The param is a
// bare number on every corpus line at the pin (measured: values 1-8), so it
// is spliced in verbatim as the target's cmcLE bound.
func kwSoulshift(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | OptionalDecider$ You | TriggerDescription$ Soulshift "+param,
		"DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | ValidTgts$ Card.Spirit+YouOwn+cmcLE"+param+" | TgtPrompt$ Select target Spirit card with mana value "+param+" or less", has)
}

func init() { registerKeyword(kwSoulshift, "Soulshift") }
