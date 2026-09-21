// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

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
	// Sunscourge Champion's "Discard<1/Card>") rides along. The
	// token's "no mana cost" (Forge's RemoveCost$) is NOT modelled --
	// this engine derives a copy's mana value from its printed card --
	// so RemoveCost$ is deliberately not emitted (an unread param
	// would only rot the parameter census); the divergence is recorded
	// in the task report's Issues section.
	body := "AB$ CopyPermanent | Cost$ " + param + " ExileFromGrave<1/CARDNAME> | ActivationZone$ Graveyard | SorcerySpeed$ True | Defined$ Self | SetColor$ "
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
