package kgen

// api:Play from the command zone (CR 903.8; Geode Golem's "you may cast your
// commander from the command zone without paying its mana cost"): Valid$
// Card.IsCommander+YouOwn, ValidZone$ Command, no ValidSA$, no Amount$.
//
// gorge's effPlay walks only the resolving controller's command zone and
// keeps the cards Valid$ matches; the kernel's command-zone population
// (OpCastFree with the Command zone, cast_freecast.go) is that controller's
// own commanders, which are exactly the cards IsCommander+YouOwn admits
// there, so the filter lowers to none. An absent ValidSA$ filters nothing
// in gorge, and a commander is never a land, so OpCastFree's land skip
// drops nothing either. The free cast still pays the commander tax: the
// kernel's buildCostGenPre adds it in its sfFree branch (legal_gen.go), as
// gorge's beginCast captures it for every mode.

// playCommandZone reports whether ps is the command-zone commander Play the
// kernel's Command population reads exactly.
func playCommandZone(ps *params) bool {
	_, hasSA := ps.m["ValidSA"]
	_, hasAmount := ps.m["Amount"]
	_, hasVT := ps.m["ValidTgts"]
	return ps.m["ValidZone"] == "Command" && ps.m["Valid"] == "Card.IsCommander+YouOwn" &&
		!hasSA && !hasAmount && !hasVT && ps.m["Defined"] == ""
}
