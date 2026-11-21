package kgen

import (
	"regexp"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Meld (CR 701.42, 712.4; gorge engine/effects/meld.go, state/meld.go).
//
// The result card (AlternateMode:Meld with a second face) lowers as kop.FaceMeld:
// its Faces[1] is the melded permanent's face and its def carries the sum of the
// two front faces' mana values (CR 712.4c, 202.3c: gorge state.ManaValue). The
// pair's other half has one face and lowers as a single-faced card (meldHalf).
// api Meld lowers to OpExt ExtMeld (dirac/face_meld.go): F is the partner's
// named filter (the pair's other card, on the battlefield, under the melder's
// control: effMeld's partner scan). Only the result card's own Meld ability is
// accepted, and Tapped$/Attacking$ (Mishra, Claimed by Gix) refuse until
// part 2.

// meldCommanderRefusal is the refusal a Meld result card gets in a Commander
// pool: dirac does not mirror gorge's CR 903.3 handling of a melded commander
// yet (part 2), so the card is refused by name and nothing diverges.
const (
	meldCommanderRefusal = "meld commander (CR 903.3) — part 2"
	meldAPI              = "Meld" // the IR API word, compared once
)

// commanderPools are the pools whose games are Commander pods.
var commanderPools = map[string]bool{"cmd5": true, "edh": true, "mbc": true}

var (
	meldPrimaryRe   = regexp.MustCompile(`Primary\$\s*([^|]+?)\s*(?:\||$)`)
	meldSecondaryRe = regexp.MustCompile(`Secondary\$\s*([^|]+?)\s*(?:\||$)`)
)

// meldNames is the Primary$ and Secondary$ of c's front face's Meld
// instruction, read from its abilities and its SVars (a trigger's Execute$).
func meldNames(c *cards.Card) (primary, secondary string) {
	if c == nil || len(c.Faces) == 0 || c.Faces[0] == nil {
		return "", ""
	}
	f := c.Faces[0]
	for _, sa := range f.Abilities {
		if sa != nil && sa.API == meldAPI {
			return strings.TrimSpace(sa.Params["Primary"]), strings.TrimSpace(sa.Params["Secondary"])
		}
	}
	for _, v := range f.SVars {
		if !strings.Contains(v, "$ Meld") {
			continue
		}
		if m := meldPrimaryRe.FindStringSubmatch(v); m != nil {
			if n := meldSecondaryRe.FindStringSubmatch(v); n != nil {
				return m[1], n[1]
			}
		}
	}
	return "", ""
}

// meldPartnerName is the name of the card c melds with: the other of the
// Meld instruction's two names.
func meldPartnerName(c *cards.Card) (string, bool) {
	pr, sec := meldNames(c)
	if pr == "" || sec == "" || c.Faces[0] == nil {
		return "", false
	}
	switch c.Faces[0].Name {
	case pr:
		return sec, true
	case sec:
		return pr, true
	}
	return "", false
}

// meldAccepted reports whether the Meld result card c lowers: it names its
// partner, the partner is a Meld card that lowers as a single face, and the
// pool is not a Commander pool.
func (lc *lowerCtx) meldAccepted(c *cards.Card) bool {
	if lc.out.Token {
		lc.refuseNode("card", "Meld layout on a token script")
		return false
	}
	if lc.p.commander {
		lc.refuseNode("card", "%s", meldCommanderRefusal)
		return false
	}
	pn, ok := meldPartnerName(c)
	if !ok {
		lc.refuseNode("card", "Meld result card without a Meld instruction naming its partner")
		return false
	}
	if pc := lc.p.byName[pn]; pc != nil && !meldHalf(pc) {
		lc.refuseNode("card", "Meld partner %q is not a single-faced meld half", pn)
		return false
	}
	return true
}

// meldResultCost sets the melded face's mana cost to the pair's mana-value
// sum, generic only so the face's colours stay its own (CR 712.4c).
func (lc *lowerCtx) meldResultCost(c *cards.Card) {
	pn, ok := meldPartnerName(c)
	pc := lc.p.byName[pn]
	if !ok || pc == nil || len(pc.Faces) == 0 || pc.Faces[0] == nil {
		return // the partner is not in the pool: the pair never melds
	}
	n := int(c.Faces[0].ManaValue()) + int(pc.Faces[0].ManaValue())
	lc.out.Cost, lc.out.CostEmpty = manaCost{Generic: n}, false
}

// meld is api Meld: the source (a result card) exiles itself and its
// partner and returns them melded.
func (lc *lowerCtx) meld(ps *params) error {
	if lc.out.FaceMode != kop.FaceMeld || lc.out.Face != 0 {
		return refuse("Meld outside a Meld result card's front face")
	}
	pn, ok := meldPartnerName(lc.card)
	if !ok {
		return refuse("Meld without Primary$/Secondary$ naming the source")
	}
	name := ps.str("Name")
	if len(lc.card.Faces) < 2 || lc.card.Faces[1] == nil || lc.card.Faces[1].Name != name {
		return refuse("Meld Name$ %q is not the result face", name)
	}
	ps.consume("Primary", "Secondary", "SecondaryType")
	if ps.str("Tapped") != "" || ps.str("Attacking") != "" {
		return refuse("Meld with Tapped$/Attacking$ (tapped and attacking melded entry; part 2)")
	}
	f, err := lc.filter("Permanent.YouCtrl+named" + strings.NewReplacer(",", ";", " ", "_").Replace(pn))
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Meld", Obj: lRef{Kind: rSelf}, F: f})
	return nil
}

// meldPresent2 admits a Meld result card's second presence clause, "a creature
// (land) named <partner> you control and own" (gorge presentClauseHolds reads
// IsPresent2$ like IsPresent$: the battlefield, GE1 by default).
func meldPresent2(lc *lowerCtx, ip2 string) bool {
	return lc.out.FaceMode == kop.FaceMeld && strings.Contains(ip2, "+named")
}

// meldPresent2OK reports whether an IsPresent2$ expression is one lowerTriggered
// admits: the Case Solved shapes, or (on a meld face) the partner's named
// presence. Kept out of lowerTriggered so its branch expression stays as it
// was (codeshape).
func meldPresent2OK(lc *lowerCtx, ip2 string) bool {
	if meldPresent2(lc, ip2) {
		return true
	}
	p := strings.TrimSpace(ip2)
	return p == "Card.Self+!IsSolved" || p == "Card.Self+IsSolved"
}

// meldSVarCompareDefault supplies Forge's GE1 default compare for a meld
// face's CheckSVar gate (gorge CheckSVarCompare; Urza, Hanweir Battlements).
func meldSVarCompareDefault(lc *lowerCtx, cmp string, ok bool) (string, bool) {
	if ok || lc.out.FaceMode != kop.FaceMeld {
		return cmp, ok
	}
	return "GE1", true
}
