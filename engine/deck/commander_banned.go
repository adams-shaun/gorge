package deck

import "github.com/adams-shaun/gorge/engine/cards"

// CommanderBannedNames is the Commander banned list, one front-face card name
// per entry, sorted.
//
// Source: the Commander section of the official Wizards page
// https://magic.wizards.com/en/banned-restricted-list, fetched 2026-11-11 (the
// page shows no update date; its newest linked announcement is
// "commander-banned-and-restricted-february-9-2026").
//
// Refresh: edit this list by hand from that page and bump the fetch date
// above. No automation reads the page.
//
// Not covered (the page names them without listing cards): Conspiracy-type
// cards, the ante cards, the racially-offensive-imagery list, and Lutri
// (banned only as a companion).
var CommanderBannedNames = []string{
	"Ancestral Recall",
	"Balance",
	"Black Lotus",
	"Channel",
	"Chaos Orb",
	"Dockside Extortionist",
	"Emrakul, the Aeons Torn",
	"Erayo, Soratami Ascendant",
	"Falling Star",
	"Fastbond",
	"Flash",
	"Golos, Tireless Pilgrim",
	"Griselbrand",
	"Hullbreacher",
	"Iona, Shield of Emeria",
	"Jeweled Lotus",
	"Karakas",
	"Leovold, Emissary of Trest",
	"Library of Alexandria",
	"Limited Resources",
	"Mana Crypt",
	"Mox Emerald",
	"Mox Jet",
	"Mox Pearl",
	"Mox Ruby",
	"Mox Sapphire",
	"Nadu, Winged Wisdom",
	"Paradox Engine",
	"Primeval Titan",
	"Prophet of Kruphix",
	"Recurring Nightmare",
	"Rofellos, Llanowar Emissary",
	"Shahrazad",
	"Sundering Titan",
	"Sylvan Primordial",
	"Time Vault",
	"Time Walk",
	"Tinker",
	"Tolarian Academy",
	"Trade Secrets",
	"Upheaval",
	"Yawgmoth's Bargain",
}

// commanderBanned is CommanderBannedNames keyed by cards.NormalizeName. It is
// built once and only ever read by key, never ranged.
var commanderBanned = bannedSet(CommanderBannedNames)

func bannedSet(names []string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[cards.NormalizeName(n)] = true
	}
	return m
}

// IsCommanderBanned reports whether c is on the Commander banned list. It
// reads the front face only, like AnyNumberAllowed.
func IsCommanderBanned(c *cards.Card) bool {
	if len(c.Faces) == 0 {
		return false
	}
	return commanderBanned[cards.NormalizeName(c.Faces[0].Name)]
}
