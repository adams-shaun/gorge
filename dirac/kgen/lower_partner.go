package kgen

// partnerTexts are the partner—[text] abilities CR 702.124i names, as the
// corpus spells them after K:Partner:.
var partnerTexts = map[string]bool{
	"Character select": true, "Father & Son": true, "Friends forever": true, "Survivors": true,
}

// deckOnlyPartner reports whether a printed K:Partner line's parameter is
// partner (none) or a partner—[text] ability (CR 702.124h/i). Both modify
// deck construction only and function before the game begins (CR 702.124a):
// nothing in play reads them (gorge registers kw:Partner play-free, and a
// with<Keyword> filter word admits no Partner), so the line lowers to
// nothing. Which two cards may be a seat's commanders is deck.IsPartnerPair,
// the one home gorge's commander config and the dirac adapter's validate both
// call; the kernel seats the pair from Config.Commanders (zone_cmdpartner.go).
// Any other parameter refuses by name.
func deckOnlyPartner(param string) bool {
	return param == "" || partnerTexts[param]
}
