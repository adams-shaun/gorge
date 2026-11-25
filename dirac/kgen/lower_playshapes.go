package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac"
)

// The Play shapes past the free, optional, ValidSA$-filtered one (lower_zone.go play).

// playLandCreatureTypes are the creature types a land card carries in the
// corpus (measured over every Types: line naming Land: the Land Creature
// Forest Dryad, Island Fish and Mountain Goblin). A population headed by any
// other creature type holds no land card.
var playLandCreatureTypes = map[string]bool{"Dryad": true, "Fish": true, "Goblin": true}

// playDefaultValidSA is the ValidSA$ an absent one reads as. gorge's effPlay
// filters nothing without a ValidSA$, so a land in the population is offered
// (and played as a land); OpCastFree always skips lands, so the absent
// ValidSA$ reads as "Spell" only over a population no land card can match:
// every alternative of the ValidTgts$ (else Valid$) filter is headed Instant,
// Sorcery or Battle, is headed by a creature type no land carries (Horde of
// Notions' Elemental), or says nonLand. Any other population keeps the
// refusal.
func playDefaultValidSA(ps *params) (string, bool) {
	if _, ok := ps.m["ValidSA"]; ok {
		return ps.str("ValidSA"), true
	}
	pop, ok := ps.m["ValidTgts"]
	if !ok {
		pop, ok = ps.m["Valid"]
	}
	if !ok || strings.TrimSpace(pop) == "" {
		return "", false
	}
	for _, alt := range strings.Split(pop, ",") {
		if !playAltNonland(strings.TrimSpace(alt)) {
			return "", false
		}
	}
	return "Spell", true
}

// playAltNonland reports a filter alternative that no land card matches.
func playAltNonland(alt string) bool {
	head, props, _ := strings.Cut(alt, ".")
	for _, p := range strings.Split(props, "+") {
		if p == "nonLand" {
			return true
		}
	}
	switch head {
	case "Instant", "Sorcery", "Battle":
		return true
	}
	return dirac.IsCreatureTypeWord(head) && !playLandCreatureTypes[head]
}

// playMandatoryOK refuses a mandatory Play (no Optional$ True: gorge's ask
// has Min 1, or every candidate under Amount$ All) on a shape the kernel's
// no-Done offer does not cover: the Hideaway play (lower_hideaway.go) and the
// copy, repeat and up-to casts. A paid play lowers as mandatory: the kernel's
// freeMandatory bit drops the offer's Done, and a picked card whose printed
// cost cannot be paid is begun and reversed exactly as an optional one is
// (CR 733.1, the freeQ drain).
func playMandatoryOK(mandatory, hideaway, copyCard, repeat, upTo bool) error {
	if mandatory && (hideaway || copyCard || repeat || upTo) {
		return refuse("Play without Optional on a Hideaway, copy, repeat or up-to cast")
	}
	return nil
}

// playBaseCS is the CastFree flag word's head: Mandatory drops the offer's
// Done (opCastFree's C bit 11).
func playBaseCS(mandatory bool) string {
	if mandatory {
		return "Mandatory"
	}
	return "Optional"
}

// playLandShape reports the Play without ValidSA$ over a land-capable
// population the kernel plays: an optional play of the remembered cards in exile
// (Valid$ Card.IsRemembered, ValidZone$ Exile), paid with Amount$ All (Dungeon of
// the Mad Mage's Runestone Caverns, the Dig's two exiled cards) or free with
// Amount$ 1 (Vault 112: Sadistic Simulation's "you may play one of those cards
// without paying its mana cost"). OpCastFree's Lands flag offers its lands as
// land plays (dirac/cast_freeland.go), which a free play makes the same way: a
// land is never cast, so WithoutManaCost$ changes nothing about it.
func playLandShape(ps *params, paid, mandatory, copyCard, repeat, upTo, forget, replace bool, amount string) bool {
	if _, ok := ps.m["ValidSA"]; ok {
		return false
	}
	_, hasT := ps.m["ValidTgts"]
	return (paid && amount == "All" || !paid && amount == "1") && !mandatory && !copyCard && !repeat && !upTo && !forget && !replace && !hasT &&
		ps.m["Defined"] == "" && ps.m["ValidZone"] == "Exile" && ps.m["Valid"] == "Card.IsRemembered"
}

// playLandsFlag is the CastFree flag a land-capable population adds (opCastFree's
// C bit 12: the picked lands are played, dirac/cast_freeland.go).
func playLandsFlag(landPlay bool) string {
	if landPlay {
		return "|Lands"
	}
	return ""
}

// playRememberedZone is the zone a Play of Defined$ Remembered reads its cards
// in: the hand straight after a Draw that remembered its cards (Mad Wizard's
// Lair's free cast of one of three drawn cards), else exile.
func (lc *lowerCtx) playRememberedZone() int {
	if n := len(lc.ab.Prog); n > 0 && lc.ab.Prog[n-1].Op == "Draw" && lc.ab.Prog[n-1].CS == "Remember" {
		return zHand
	}
	return zExile
}
