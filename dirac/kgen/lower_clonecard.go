package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The three Clone forms whose template or grant lives off the battlefield
// (gorge effects/clone.go effClone), beside lower_clonecopy.go's six:
//
//	TgtZone$ Graveyard, ValidTgts$ Permanent.YouOwn   the target's printed card      Shifting Woodland
//	Defined$ Exiled, an Exile<1/...> cost             the exiled card, printed        Curie, Emergent Intelligence
//	AddTriggers$ + AddSVars$                          a latent trigger of this card   Curie
//
// The first two copy a card that is not a permanent, so the kernel reads its
// printed card (kop.CloneTmplCard, CloneTmplSac; dirac/copy_clonecard.go) where
// a permanent template is read through its slot. AddTypes$ needs nothing of
// its own: lowerCtx.copyExceptions reads it for every copy.

// cloneGraveTemplate checks template ref tr, a target slot over a graveyard
// card (TgtZone$ Graveyard), and packs its slot into flags. The filter must
// name permanent cards: gorge's pairs copy the card's printed face whatever it
// is, and a spell card there would copy a non-permanent.
func (lc *lowerCtx) cloneGraveTemplate(tr lRef, flags *int) error {
	t := &lc.ab.Targets[tr.N]
	if !flagsEOT(*flags) {
		// a permanent copy of a graveyard card (Likeness Looter) keeps a
		// marker on the printed card for every later re-base: not read here
		return refuse("Clone TgtZone$ Graveyard without Duration$ UntilEndOfTurn")
	}
	if t.F == nil || t.F.Players != 0 || len(t.F.Clauses) == 0 {
		return refuse("Clone template graveyard target names no card")
	}
	for _, c := range t.F.Clauses {
		if c.Types == 0 || c.Types&^tPermanent != 0 {
			return refuse("Clone template graveyard target that is not a permanent card")
		}
	}
	*flags |= kop.CloneTmplCard | tr.N<<kop.CloneTmplShift
	return nil
}

// cloneExiledTemplate lowers Defined$ Exiled: the template is the one card the
// activation's own Exile<1/Spec> cost exiled (gorge's paid-cost "Exiled" list,
// whose printed face is copied, CR 400.7). The kernel's entry records that
// card as its sacrifice snapshot (snapSacPick), so the cost must be exactly one
// picked exile of a nontoken artifact or creature.
func (lc *lowerCtx) cloneExiledTemplate(flags *int) error {
	c := &lc.ab.Cost
	if lc.ab.Kind != akActivated || lc.ab.Latent || !lc.sacrificedIsCost() || !c.SacExile {
		return refuse("Clone Defined$ Exiled without exactly one Exile<> cost")
	}
	if err := cloneTemplateFilter(c.Sac); err != nil {
		return err
	}
	for _, cl := range c.Sac.Clauses {
		if cl.Flags&fNonToken == 0 {
			return refuse("Clone Defined$ Exiled with a cost that may exile a token")
		}
	}
	*flags |= kop.CloneTmplSac
	return nil
}

// cloneTriggerGrant reads AddTriggers$ (with the AddSVars$ naming what the
// trigger's body reads): each named SVar is a trigger line of this face,
// lowered as a latent trigger the copy is granted as one of its own abilities
// (CR 707.9b; cloneEnterGrant, the as-enters form of the same exception). The
// grant is a grant-only override after the exceptions' (kop.CloneGrant), kept
// with the permanent copy marker; an until-end-of-turn copy keeps no grant
// record of its own.
func (lc *lowerCtx) cloneTriggerGrant(ps *params, flags *int, hasExc *bool) (*lOverride, error) {
	if _, ok := ps.get("GainThisAbility"); ok {
		return nil, refuse("Clone AddTriggers$ beside GainThisAbility$")
	}
	if lc.ab == nil || lc.ab.Latent || lc.granted > 0 {
		return nil, refuse("Clone AddTriggers$ outside a printed ability")
	}
	if flagsEOT(*flags) {
		return nil, refuse("Clone AddTriggers$ with Duration$ UntilEndOfTurn")
	}
	if strings.TrimSpace(ps.str("AddTriggers")) == "" {
		return nil, refuse("Clone AddTriggers$ names no trigger")
	}
	o, err := lc.cloneEnterGrant(ps)
	if err != nil || o == nil {
		return nil, err
	}
	*flags |= kop.CloneGrant
	*hasExc = true
	return o, nil
}
