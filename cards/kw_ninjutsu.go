// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

// kwNinjutsu expands the printed K:Ninjutsu:<cost> line (CR 702.49) into the
// activated ability the keyword means: "{cost}, Return an unblocked attacker
// you control to hand: Put this card onto the battlefield from your hand
// tapped and attacking."
//
// The keyword is a casting/activation OPTION read off the K: line, but unlike
// Flash/Backlash it has a real body -- a hand-zone activated ability -- so it
// is expanded onto the face exactly as Equip and Cycling are. The body is:
//
//   - ActivationZone$ Hand: the source is the card in hand, so the ordinary
//     activated-ability offer loop (rules/legal.go walks
//     Battlefield/Graveyard/Hand) offers it there with no carve-out.
//   - Cost$ <printed cost> Return<1/Creature.YouCtrl+attacking+unblocked>:
//     the mana cost the K: line carries plus the CR 702.49a "return an
//     unblocked attacker you control to hand" half, modelled through the
//     ordinary Return<N/Spec> cost part (rules/cast.go's returnAsk). The
//     spec's attacking+unblocked predicates (effects/filter.go) are what
//     withhold the activation until an attacker is actually unblocked, so a
//     blocked or non-attacking creature can never pay it.
//   - Defined$ Self | Origin$ Hand | Destination$ Battlefield | Tapped$ True
//     | Attacking$ True: the resolution moves the source card out of hand
//     onto the battlefield tapped and attacking. effects/zone.go's
//     ChangeZone rider emits TokenAttacks against the Ctx's DefendingPlayer,
//     which rules/stack.go binds from the defender captured when the Return
//     cost was paid -- CR 702.49b's "the same player ... the returned
//     creature was attacking".
//   - ActivationPhases$ Declare Blockers,Combat Damage,EndCombat: CR 702.49a
//     allows the activation only from the declare blockers step onward, which
//     the shared activationPhasesOK gate enforces.
//
// A line with no cost (a malformed K:Ninjutsu:) is not expanded: the ability
// would be unpayable, and rules reads nothing off a bare head either.
//
// The colon parameter may carry a trailing rider field: Yuriko, the Tiger's
// Shadow prints `K:Ninjutsu:U B:Commander` (CR 903.8's commander ninjutsu,
// which may also be activated from the command zone). Only the FIRST colon
// field is the cost -- the same split the Equip/Fortify expansion takes
// (cards/kw_equip.go) -- so the rider can never leak into the mana cost. The
// command-zone half is not offered: rules/legal.go's offer walk enumerates
// only battlefield, graveyard and hand, so `ActivationZone$ Command` would
// withhold the whole ability rather than widen it. Yuriko's hand activation
// works; the command-zone activation is recorded in the ticket report.
func kwNinjutsu(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("A", k) {
		return
	}
	cost, _, _ := strings.Cut(strings.TrimSpace(param), ":")
	cost = strings.TrimSpace(cost)
	if cost == "" {
		return
	}
	sa, _ := parseSA("", "AB$ ChangeZone | Defined$ Self | Origin$ Hand | Destination$ Battlefield"+
		" | Tapped$ True | Attacking$ True"+
		" | Cost$ "+cost+" Return<1/Creature.YouCtrl+attacking+unblocked>"+
		" | ActivationZone$ Hand"+
		" | ActivationPhases$ Declare Blockers,Combat Damage,EndCombat"+
		" | Keyword$ Ninjutsu | SpellDescription$ Ninjutsu "+cost)
	if sa != nil {
		sa.Params["KeywordLine"] = k
		f.Abilities = append(f.Abilities, sa)
	}
}

func init() { registerKeyword(kwNinjutsu, "Ninjutsu") }
