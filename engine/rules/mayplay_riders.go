package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// mayPlayAnySeat reports whether a MayPlayPlayer$ value is Player: every
// player benefits (Aluren: "Any player may cast creature spells ...").
func mayPlayAnySeat(v string) bool { return v == "Player" }

// mayPlayActiveSeat reports whether a MayPlayPlayer$ value names the active
// player (Weftwalking's Player.Active; Ian Malcolm / Share the Spoils'
// ActivePlayer): the beneficiary is whoever's turn it is, not the static's
// controller.
func mayPlayActiveSeat(v string) bool { return v == "Player.Active" || v == "ActivePlayer" }

// mayPlaySeatBeneficiary reports whether a MayPlayPlayer$ value names a
// beneficiary other than the static's controller that mayPlayStatic resolves
// itself: the per-card Exiler / CardOwner, every player, or the active player.
func mayPlaySeatBeneficiary(v string) bool {
	return mayPlayPerCardBeneficiary(v) || mayPlayAnySeat(v) || mayPlayActiveSeat(v)
}

// mayPlayWithFlash reports whether a printed MayPlay$ static carrying
// MayPlayWithFlash$ True grants card id to p ("you may cast them as though
// they had flash", Azula; Aluren's "as though they had flash"). It reads the
// same gates every may-play offer reads (mayPlayStatic: Condition$
// PlayerTurn, the beneficiary, Affected$/AffectedZone$), so the timing
// permission exists exactly where the grant does and covers only the cards
// the grant names (CR 601.3, CR 702.8a). A pure read.
func (e *Engine) mayPlayWithFlash(p state.PlayerID, id state.ObjID) bool {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return false
	}
	for _, st := range mayPlayOwnStatics(o, p) {
		if st.Mode != "Continuous" || !strings.EqualFold(st.ParamStr(cards.PKMayPlayWithFlash), "True") {
			continue
		}
		if applies, _, _, _, _, _ := e.mayPlayStatic(st.Params, id, o.Controller, id); applies {
			return true
		}
	}
	for _, sv := range e.activeStatics("Continuous") {
		if !strings.EqualFold(sv.ParamStr(cards.PKMayPlayWithFlash), "True") || !mayPlayStaticBenefits(p, &sv) {
			continue
		}
		if applies, _, _, _, _, _ := e.mayPlayStatic(sv.Params, id, mayPlayStaticYou(p, &sv), sv.Source); applies {
			return true
		}
	}
	return false
}

// spellFlashGranted is the shared needFlash resolution: a CastWithFlash static
// or a printed may-play grant's MayPlayWithFlash$ rider lets p cast id at
// instant speed. cw is the caller's CastWithFlash answer (the engine's or the
// walk's bit-fast one).
func (e *Engine) spellFlashGranted(p state.PlayerID, id state.ObjID, cw bool) bool {
	return cw || e.mayPlayWithFlash(p, id)
}
