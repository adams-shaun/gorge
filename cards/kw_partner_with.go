// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

// kwPartnerWith expands CR 702.128's partner-with keyword (K:Partner
// with:<name>[:<short-name>], 52 corpus files at the pin). Unlike the bare
// K:Partner line -- which is deck CONSTRUCTION only and is read by
// deck.IsPartnerPair, never by a trigger -- the partner-with keyword prints
// real rules text in addition to the designation it shares with K:Partner:
//
//	"Partner with [name] (When this creature enters, target player may put
//	 [name] into their hand from their library, then shuffle.)"
//
// The Oracle line above is reminder text around a real ETB triggered ability,
// and no carrier's script prints that ability separately (measured: 0 of the
// 52 K:Partner with lines carry a matching T:/SVar pair), so the expansion
// supplies it. Forge spells it as an ordinary ETB trigger whose effect is a
// hidden-library search:
//
//	T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield |
//	 ValidCard$ Card.Self | TriggerZones$ Battlefield | Execute$ __kw... |
//	 TriggerDescription$ Partner with <name>
//	<SVar> DB$ ChangeZone | Origin$ Library | Destination$ Hand |
//	 ChangeType$ Card.named<name> | ChangeTypeDesc$ <name> |
//	 DefinedPlayer$ Targeted | Optional$ True | ValidTgts$ Player
//
// The param is "<full name>" or "<full name>:<short name>" (Sylvia
// Brightspear:Sylvia, Frodo, Adventurous Hobbit:Frodo): the search must use
// the FULL printed card name -- the short form is a deck-hint alias, not a
// name a library card carries -- so everything from the first colon is
// dropped. A name may contain a raw comma ("Kamber, the Plunderer") or an
// ampersand ("Bebop, Skull & Crossbones"); the named<...> filter keeps a raw
// comma in the argument when what follows is not a filter alternative, and
// "&" is not a filter separator, so both parse as the printed name.
//
// DefinedPlayer$ Targeted makes the searching player the player the trigger
// targeted (CR 702.128's "target player"), and Chooser$ Targeted makes that
// same targeted player the one who ANSWERS the private search ask — without
// it, effSearchLibrary defaults the decision seat to the trigger controller,
// who would read the opponent's library. ValidTgts$ Player sits on this same
// effect SA because that is the SA rules asks targets for at trigger push
// (the Kitesail Freebooter shape). The quality is STATED
// (Card.named<name>), so effSearchLibrary offers a fail-to-find Min of 0 --
// the "may" -- and then shuffles by default; Optional$ True is carried
// verbatim from Forge's script shape even though the search path's Min 0,
// not the param, is what realises the election here.
func kwPartnerWith(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	name := strings.TrimSpace(param)
	if j := strings.IndexByte(name, ':'); j >= 0 {
		name = strings.TrimSpace(name[:j])
	}
	if name == "" {
		// A bare K:Partner with with no name is not a corpus shape; leave it
		// unexpanded rather than mint a search for the empty name (which
		// sharesName would never match), the fail-closed direction.
		return
	}
	f.addKeywordTrigger(head, k,
		"Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | TriggerZones$ Battlefield | TriggerDescription$ Partner with "+name,
		"DB$ ChangeZone | Origin$ Library | Destination$ Hand | ChangeType$ Card.named"+name+
			" | ChangeTypeDesc$ "+name+" | DefinedPlayer$ Targeted | Chooser$ Targeted | Optional$ True | ValidTgts$ Player | TgtPrompt$ Select target player", has)
}

func init() { registerKeyword(kwPartnerWith, "Partner with") }
