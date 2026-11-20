package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// embalmGrantKW is the one granted embalm the kernel models: a Pump's
// KW$ Embalm:CardManaCost on a creature card in its owner's graveyard
// (Cursecloth Wrappings; CR 702.128a). The embalm cost is the card's own mana
// cost, which the kernel reads off the embalmed card (dirac kw_embalmgrant.go),
// so the lowered body's mana cost is a placeholder zero. Every other granted
// keyword, Eternalize:CardManaCost included, stays refused by name
// (pumpKeywords).
const embalmGrantKW = "Embalm:CardManaCost"

// grantedEmbalmLowers gates the lowering: off, the granted embalm refuses by
// name like every other granted keyword. gorge prices a granted
// Embalm:CardManaCost at {1} (rules.ParseCost reads the CardManaCost token as
// generic 1; only escapeCost and the mayhem arm substitute the card's mana
// cost) and its offer walk withholds the option the kernel makes, so a pool
// that drew Cursecloth Wrappings would diverge from gorge. The kernel half
// (kw_embalmgrant.go) is landed and tested through the -granted-embalm flag;
// flip this default when gorge's pricing lands (ticket
// gorge-granted-embalm-cardmanacost-pricing), then add the lockstep.
var grantedEmbalmLowers bool

// pumpEmbalmGrant lowers the grant: the body gorge's printed kwEmbalm
// expander mints (one graveyard sorcery-speed AB$ CopyPermanent of Self whose
// cost is the parameter plus exiling the card itself, and whose copy is a
// white Zombie with no mana cost), lowered through the ordinary ability path
// as the granting card's latent ability (grantedKeywordLatent), and
// OpGrantEmbalmOwnCost on the target naming it. The expander is gorge's own,
// so the printed lowerEmbalmKw and this grant lower one body.
func (lc *lowerCtx) pumpEmbalmGrant(obj lRef) error {
	sa := cards.GrantedKeywordAbility("Embalm:0")
	if sa == nil || sa.Kind != "AB" {
		return refuse("keyword %q", embalmGrantKW)
	}
	idx, err := lc.grantedKeywordLatent(sa)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "GrantEmbalmOwnCost", Obj: obj, C: idx})
	return nil
}

// pumpGraveyardKwGrant lowers a graveyard Pump whose whole body is one of the
// two keyword grants the kernel models: Araumi's Encore:CardManaCost
// (ExtGrantEncore) and Cursecloth's embalm. done is false for any other
// keyword, which the ordinary grant path lowers or refuses.
func (lc *lowerCtx) pumpGraveyardKwGrant(ps *params, obj lRef) (done bool, err error) {
	kw, ok := ps.get("KW")
	switch {
	case !ok:
		return false, nil
	case strings.TrimSpace(kw) == "Encore:CardManaCost":
		lc.emit(lInstr{Op: "GrantEncore", Obj: obj})
		return true, nil
	case strings.TrimSpace(kw) == embalmGrantKW:
		if !grantedEmbalmLowers {
			return true, refuse("keyword %q", embalmGrantKW)
		}
		return true, lc.pumpEmbalmGrant(obj)
	}
	return false, nil
}
