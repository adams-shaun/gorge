// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// kwHaunt expands K:Haunt:<SVar> (CR 702.55). The param names the card's own
// "enters or the creature it haunts dies" effect SVar; the keyword supplies the
// rest as up to three triggers:
//
//   - a creature's dies trigger (702.55a) and an instant/sorcery's
//     resolution-to-graveyard trigger (702.55b), each exiling the card and
//     haunting a target creature. The exile is a ChangeZone of the card's new
//     graveyard object with Haunt$ True, which makes effects.emitHaunt fold the
//     link into state.Object.Haunting through events.Haunt.
//   - the haunting card's trigger from exile (702.55c): "when the creature it
//     haunts dies" over ValidCard$ Creature.HauntedBy, running the param SVar.
//   - for a creature, the same param effect as its own enters trigger
//     ("When this creature enters or the creature it haunts dies"); for an
//     instant/sorcery, the same param effect as the spell's own SP$ ability
//     ("Target player discards a card. Haunt ..."): the carrier script prints
//     no A:SP$ line, the keyword's SVar is the spell's effect.
func kwHaunt(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	body, ok := f.SVars[param]
	if !ok {
		return
	}
	const haunt = "DB$ ChangeZone | ValidTgts$ Creature | TgtPrompt$ Choose target creature to haunt | TgtZone$ Battlefield | Defined$ TriggeredNewCardLKICopy | Origin$ Graveyard | Destination$ Exile | Haunt$ True"
	if f.IsCreature() {
		f.addKeywordTrigger(head, k+"#dies", "Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | TriggerDescription$ Haunt (When this creature dies, exile it haunting target creature.)", haunt, has)
		f.addKeywordTrigger(head, k+"#etb", "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | TriggerDescription$ When CARDNAME enters, "+"its haunt effect happens.", body, has)
	} else {
		if len(body) > 3 && body[:3] == "DB$" && !has("A", k) && f.SpellAbility() == nil {
			if sa, _ := parseSA("", "SP$"+body[3:]); sa != nil {
				sa.Params["KeywordLine"] = k
				f.Abilities = append(f.Abilities, sa)
			}
		}
		f.addKeywordTrigger(head, k+"#resolved", "Mode$ ChangesZone | Origin$ Stack | Destination$ Graveyard | ValidCard$ Card.Self | ResolvedOnly$ True | TriggerDescription$ Haunt (When this spell card is put into a graveyard after resolving, exile it haunting target creature.)", haunt, has)
	}
	f.addKeywordTrigger(head, k+"#haunted", "Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature.HauntedBy | TriggerZones$ Exile | TriggerDescription$ When the creature CARDNAME haunts dies, its haunt effect happens.", body, has)
}

func init() { registerKeyword(kwHaunt, "Haunt") }
