package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
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

// mayPlayFlashBoard is the narrow board read mayPlayFlashGranted needs: the
// battlefield statics and the shared may-play gate chain. *Engine satisfies it.
type mayPlayFlashBoard interface {
	activeStatics(mode string) []staticView
	mayPlayStatic(params map[string]string, id state.ObjID, you state.PlayerID, source state.ObjID) (applies, grants, free bool, raise Cost, hasRaise, priced bool)
}

// mayPlayFlashGranted reports whether a printed MayPlay$ static carrying
// MayPlayWithFlash$ True lets p cast o from its current zone at instant speed
// ("you may cast them as though they had flash", Azula; Elsha's top-of-library
// cast). It reads the gates every may-play offer reads (mayPlayStatic:
// Condition$ PlayerTurn, the beneficiary, Affected$/AffectedZone$) and covers
// only a cast made through a static that OPENS o's zone, so the timing
// permission exists exactly where that grant does (CR 601.3, CR 702.8a).
//
// A card in hand is never covered here: the flash of a static that delivers an
// alternative cost (Aluren's free cast, Primal Prayers' {E}) belongs to that
// alternative-cost option alone (altCostView.flash), not to the ordinary cast
// of the same card. A pure read.
func mayPlayFlashGranted(b mayPlayFlashBoard, o *state.Object, p state.PlayerID) bool {
	if o == nil || o.Face() == nil || o.Zone == state.ZHand {
		return false
	}
	for _, st := range mayPlayOwnStatics(o, p) {
		if st.Mode != "Continuous" || !strings.EqualFold(st.ParamStr(cards.PKMayPlayWithFlash), "True") {
			continue
		}
		if _, grants, _, _, _, _ := b.mayPlayStatic(st.Params, o.ID, o.Controller, o.ID); grants {
			return true
		}
	}
	for _, sv := range b.activeStatics("Continuous") {
		if !strings.EqualFold(sv.ParamStr(cards.PKMayPlayWithFlash), "True") || !mayPlayStaticBenefits(p, &sv) {
			continue
		}
		if _, grants, _, _, _, _ := b.mayPlayStatic(sv.Params, o.ID, mayPlayStaticYou(p, &sv), sv.Source); grants {
			return true
		}
	}
	return false
}

// offerMayPlayFlashAlts is the hand walk's timing-failed arm for a may-play
// alternative cost that carries MayPlayWithFlash$ (Aluren, Primal Prayers):
// off sorcery timing the ordinary cast is withheld, and only the alternative-
// cost option the static delivers is offered, at the AltCostIndex the
// sorcery-timing alternative loop would give it (alternativeCosts' order).
func (w *legalWalk) offerMayPlayFlashAlts(p state.PlayerID, id state.ObjID, f *cards.Face, out *[]decision.Option) {
	e := w.e
	for i, alt := range e.alternativeCosts(p, id) {
		if !alt.flash || w.castRestrictedCost(p, id, alt.cost) ||
			!w.offerCastable(p, id, alt.cost, spellScope(""), false) ||
			!e.castTargetsAvailable(p, id, f.SpellAbility()) {
			continue
		}
		*out = append(*out, decision.Option{Index: len(*out), Kind: "cast",
			Label: altCostLabel(f.Name, i), Obj: id, AltCostIndex: i + 1})
	}
}
