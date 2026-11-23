package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// sharedKeywordClause builds the SharedKeywordsZone$ PumpAll lowering's
// presence predicate (lower_sharedpumpall.go): the land or protection-quality
// filter rides PredV as a FilterID (PredWord7's PredN selects the read), and a
// colour mask rides PredV directly.
func (cb *cardBuilder) sharedKeywordClause(kf *kop.Filter, c lClause) error {
	if c.CmpStat == cmpHasProtColor {
		kf.Pred, kf.PredN, kf.PredV = kop.PredWord7, kop.W7HasProtColor, kop.Const(c.CmpN)
		return nil
	}
	if c.CmpStat == cmpHasProtEachColor {
		kf.Pred, kf.PredN = kop.PredWord7, kop.W7HasProtEachColor
		return nil
	}
	if c.CmpFilter == nil {
		return fmt.Errorf("keyword presence word without its filter")
	}
	id, err := cb.filter(c.CmpFilter, kop.ZoneBattlefield)
	if err != nil {
		return err
	}
	kf.Pred, kf.PredN, kf.PredV = kop.PredWord7, kop.W7HasLandwalk, kop.Const(int(id))
	if c.CmpStat == cmpHasProtQuality {
		kf.PredN = kop.W7HasProtQuality
	}
	return nil
}

// word7ZoneRefusal holds the two PredWord7 sub-kinds that cannot be read on a
// battlefield permanent (their word is a card's latest arrival or a printed
// ability set).
func word7ZoneRefusal(c lClause, zone kop.Zone) error {
	if c.CmpN == int(kop.W7NoAbilities) && zone == kop.ZoneBattlefield {
		return refuse("NoAbilities on the battlefield")
	}
	if c.CmpN == int(kop.W7EnteredFromBF) && zone == kop.ZoneBattlefield {
		return refuse("ThisTurnEnteredFrom_Battlefield on the battlefield")
	}
	return nil
}
