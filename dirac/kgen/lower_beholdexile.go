package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/effects"
)

// raiseSelfBehold lowers the Lorwyn Champion cycle's own additional cost,
// `S:Mode$ RaiseCost | ValidCard$ Card.Self | Activator$ You | Type$ Spell |
// Cost$ BeholdExile<1/Type> | EffectZone$ All` ("as an additional cost to
// cast this spell, behold a <Type> and exile it", CR 701.4a, 601.2b/f-h).
// gorge folds the static's non-mana Cost$ into the pending cast's own cost
// (foldRaiseExtra): the beheld object is exiled with the cast spell as the
// source, the link the Champion's leaves trigger reads (Defined$ ExiledWith).
// The lowering is card data, not a static: CardDef.BeholdN / BeholdF /
// BeholdExile (kernel act_beholdexile.go). handled is true when the
// static is that shape (st is then to be dropped); every other non-mana
// Cost$ refuses by its cost head, a RaiseCost without Amount$ or Cost$
// keeps raiseCost's "empty amount".
func (lc *lowerCtx) raiseSelfBehold(sp *params, st *lStatic) (bool, error) {
	cost, ok := sp.m["Cost"]
	if !ok || sp.m["Amount"] != "" {
		return false, nil
	}
	cost = strings.TrimSpace(cost)
	if _, life := raiseLifeN(cost); life {
		return false, nil // a life raise: raiseCost lowers it (lower_raiselife.go)
	}
	sp.consume("Cost")
	rest, found := strings.CutPrefix(cost, "BeholdExile<")
	if !found {
		return true, refuse("RaiseCost Cost$ %q", cost)
	}
	n, typ, ok := strings.Cut(strings.TrimSuffix(rest, ">"), "/")
	if cnt, err := strconv.Atoi(n); !strings.HasSuffix(rest, ">") || !ok || err != nil || cnt != 1 || !effects.CreatureTypeWords(typ) {
		return true, refuse("RaiseCost Cost$ %q: not a behold of one creature type", cost)
	}
	if sp.str("ValidCard") != "Card.Self" || sp.str("Activator") != "You" || sp.str("Type") != "Spell" || sp.str("EffectZone") != "All" {
		return true, refuse("RaiseCost Cost$ %q: not the card's own spell cost", cost)
	}
	if _, has := sp.m["Condition"]; has {
		return true, refuse("RaiseCost Cost$ %q with a Condition$", cost)
	}
	raises := 0
	for _, s := range lc.face.Statics {
		if s.Mode == "RaiseCost" {
			raises++
		}
	}
	o := lc.out
	if raises != 1 || o.BeholdN != 0 || o.SpellCost != nil || o.AltAddCost != nil || o.OptCost != nil {
		return true, refuse("RaiseCost Cost$ %q beside another raise or an additional cost", cost)
	}
	f, err := lc.filter(typ)
	if err != nil {
		return true, err
	}
	o.BeholdN, o.BeholdF, o.BeholdExile = 1, f, true
	st.Kind = "" // card data only: lowerStatic drops a kind-less static
	return true, nil
}
