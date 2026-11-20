package kgen

import (
	"fmt"
	"math/bits"
	"strings"

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

// sharedKeywordProp parses the SharedKeywordsZone$ PumpAll lowering's internal
// filter words (lower_sharedpumpall.go). A landwalk or a spec-shaped protection
// quality stores its spec text; lowerCtx.filter compiles it to CmpFilter.
func (c *lClause) sharedKeywordProp(p string) (bool, error) {
	spec, ok := strings.CutPrefix(p, sharedLandwalkWord)
	if !ok {
		if spec, ok = strings.CutPrefix(p, sharedProtWord); !ok {
			return false, nil
		}
		switch {
		case spec == "everything":
			return true, c.setCmp(cmpHasKw, 0, bits.TrailingZeros32(uint32(kop.KwProtectionAll)), "", "")
		case spec == "each color":
			return true, c.setCmp(cmpHasProtEachColor, 0, 0, "", "")
		}
		if col, ok := protColourLetters[spec]; ok {
			return true, c.setCmp(cmpHasProtColor, 0, int(colorMask(col)), "", "")
		}
		if err := c.setCmp(cmpHasProtQuality, 0, 0, "", ""); err != nil {
			return true, err
		}
		c.CmpSpec = spec
		return true, nil
	}
	if err := c.setCmp(cmpHasLandwalk, 0, 0, "", ""); err != nil {
		return true, err
	}
	c.CmpSpec = spec
	return true, nil
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

// simpleFlagProp holds the two one-flag property words (unblocked and
// hasXCost) so the property chain stays within its codeshape ceiling.
func (c *lClause) simpleFlagProp(p string) (bool, error) {
	switch p {
	case "unblocked":
		c.Unblocked = true
		return true, nil
	case "hasXCost":
		return true, c.setCmp(cmpHasX, 0, 0, "", "")
	}
	return false, nil
}

// prepareFilterClause resolves one parsed clause's late-bound parts: the
// SharedKeywordsZone$ PumpAll presence word's spec filter, the source-name
// marker, and an SVar-named bound. It is a named helper so the property
// chain's complexity stays within its codeshape ceiling.
func (lc *lowerCtx) prepareFilterClause(c *lClause, expr string) error {
	if c.CmpSpec != "" {
		switch c.CmpStat {
		case cmpHasLandwalk:
			lf, err := lc.landwalkFilter("Landwalk:"+c.CmpSpec, c.CmpSpec)
			if err != nil {
				return err
			}
			c.CmpFilter = lf
		case cmpHasProtQuality:
			qf, err := lc.qualityFilter(c.CmpSpec)
			if err != nil {
				return err
			}
			c.CmpFilter = qf
		}
		c.CmpSpec = ""
	}
	if c.Flags&fNamedAsSource != 0 {
		// every named<X> is a literal clause (gorge wordNamed ->
		// sharesName reads the NAME, not the source; CR 201.2):
		// build.go resolves Name against the pool, Pred = PredNamed.
		// The source's own printed name is no exception: a source-relative
		// read follows the source's current name (a layer-3 rename) and
		// matches nothing once the source has left the battlefield (Hare
		// Apparent killed in response to its own trigger), where gorge's
		// literal name still matches.
		c.Flags &^= fNamedAsSource
	}
	if c.CmpX != "" {
		v, err := lc.value(c.CmpX)
		if err != nil {
			return err
		}
		if v.Kind == vX && lc.ab != nil && lc.ab.Cost.LoyaltyX {
			// a mana-value bound by a planeswalker's [-X] (Tezzeret the
			// Seeker, Sorin, Vengeful Bloodlord): no lockstep reads it yet
			return refuse("filter %q: bound %s reads a loyalty [-X]", expr, c.CmpX)
		}
		c.CmpV = &v
	}
	return nil
}
