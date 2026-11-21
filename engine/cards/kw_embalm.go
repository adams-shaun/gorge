// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

// cardManaCostToken is Forge's placeholder for "the card's own mana cost" in a
// keyword grant line (Cursecloth Wrappings' Embalm:CardManaCost).
const cardManaCostToken = "CardManaCost"

func kwEmbalm(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("A", k) {
		return
	}
	// CR 702.128 (Embalm) / CR 702.129 (Eternalize): one activated
	// ability the card offers from the graveyard, whose cost is the
	// printed mana cost plus exiling the card itself, and whose effect
	// is "create a token that's a copy of it, except ...". The two are
	// one family: Embalm's token is a white Zombie in addition to its
	// other types; Eternalize's is additionally a 4/4 black Zombie.
	// The body is DB$ CopyPermanent (api CopyPermanent), whose
	// characteristic modifications AddTypes$/SetColor$/SetPower$/
	// SetToughness$ this build applies to the minted copy.
	// ExileFromGrave<1/CARDNAME> is the shared graveyard self-exile
	// cost Encore already uses, settled by the ordinary cast-flow
	// exile stage; the whole keyword parameter is spliced verbatim
	// into Cost$ so an extra cost component (Sinuous Striker's and
	// Sunscourge Champion's "Discard<1/Card>") rides along. RemoveCost$
	// True is the token's "no mana cost" (Forge's spelling); every
	// exception is a copiable value of the token (CR 707.9b).
	// A grant's CardManaCost token (Cursecloth Wrappings, CR 702.128a) is the
	// card's own mana cost. A scratch face with no mana cost (the replay
	// mint of an already-paid activation, which reads only the body) keeps
	// the token verbatim; the offer side refuses that case itself
	// (GrantedKeywordAbilityFor).
	if param == cardManaCostToken && f.ManaCost != "" {
		param = f.ManaCost
	}
	body := "AB$ CopyPermanent | Cost$ " + param + " ExileFromGrave<1/CARDNAME> | ActivationZone$ Graveyard | SorcerySpeed$ True | Defined$ Self | RemoveCost$ True | SetColor$ "
	if head == "Eternalize" {
		body += "Black | AddTypes$ Zombie | SetPower$ 4 | SetToughness$ 4"
	} else {
		body += "White | AddTypes$ Zombie"
	}
	sa, _ := parseSA("", body+" | Keyword$ "+head+" | SpellDescription$ "+head+" "+param)
	if sa != nil {
		sa.Params["KeywordLine"] = k
		f.Abilities = append(f.Abilities, sa)
	}
}

func init() { registerKeyword(kwEmbalm, "Embalm", "Eternalize") }
