package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// effectLibTopPlay reads an Effect static that lets its controller look at
// and play the top card of their library for the rest of the turn (The
// Belligerent: lands and spells; Gwenom, Remorseless: cards, a spell paid
// for with life equal to its mana value in place of its mana cost). gorge's
// effEffect registers two continuous effects from the one line: a look-only
// grant (mayLookAtGrantFromLine) and the play grant, priced through
// altCostParse when MayPlayAltManaCost$ rides along. Only the exact library-top
// spelling reads this way:
//
//	Affected$ Card.TopLibrary+YouCtrl (or +YouOwn) | AffectedZone$ Library |
//	MayLookAt$ You (or True) | MayPlay$ True [| MayPlayAltManaCost$ PayLife<ConvertedManaCost>]
//
// The look is information only: no event, decision or legality reader at the
// pin (Engine.MayLookAtLibraryTop and the seat view), the same projection the
// printed view-only look lowers to (continuous in lower_trigger.go), so it
// lowers to nothing and the play rides the ordinary library MayPlayFrom
// static, which offers the current top card only. MayLookAt$ Player reveals
// the card to every seat and stays refused, as does every other rider.
//
// ok reports the shape; rest is the static without the look and the
// alternative cost, payLife that the alternative cost was named.
func effectLibTopPlay(s cards.Static) (rest cards.Static, payLife, ok bool) {
	p := s.Params
	if s.Mode != "Continuous" || strings.TrimSpace(p["MayPlay"]) != "True" || strings.TrimSpace(p["AffectedZone"]) != "Library" {
		return s, false, false
	}
	if aff := strings.TrimSpace(p["Affected"]); aff != "Card.TopLibrary+YouCtrl" && aff != "Card.TopLibrary+YouOwn" {
		return s, false, false
	}
	if ml := strings.TrimSpace(p["MayLookAt"]); ml != "You" && ml != "True" {
		return s, false, false
	}
	cp := make(map[string]string, len(p))
	for k, v := range p {
		switch k {
		case "Mode", "Affected", "AffectedZone", "MayPlay", "Description":
			cp[k] = v
		case "MayLookAt":
		case "MayPlayAltManaCost":
			if strings.TrimSpace(v) != "PayLife<ConvertedManaCost>" {
				return s, false, false
			}
			payLife = true
		default:
			return s, false, false
		}
	}
	s.Params = cp
	return s, payLife, true
}
