package kgen

import "fmt"

// The monarch's controller words (CR 725; gorge engine/effects
// trigger_referents.go controlReferentPlayers). Two spellings lower, both
// read on the battlefield only (kop.PredMonarchCtrl, dirac/filter_monarch.go):
//   - ControlledBy Player.isMonarch (Azure Fleet Admiral's CantBlockBy
//     ValidBlocker$): the candidate's controller is the monarch; with no
//     monarch the referent is unbound and nothing matches.
//   - ControlledBy EnchantedController.!isMonarch (Fall from Favor's Untap
//     replacement): the candidate's controller is the enchanted permanent's
//     controller and that player is not the monarch. Read beside EnchantedBy,
//     the candidate is the enchanted permanent, so the two controllers are the
//     one seat and the word is "its controller is not the monarch"; without
//     EnchantedBy the clause refuses (monarchCtrlCheck).
const (
	monarchCtrlIs     = 0 // PredMonarchCtrl PredN: controlled by the monarch
	monarchCtrlIsNot  = 1 // PredMonarchCtrl PredN: controlled by a player who is not the monarch
	monarchCtrlWord   = "ControlledBy Player.isMonarch"
	monarchEnchNotWrd = "ControlledBy EnchantedController.!isMonarch"
)

// monarchCtrlProp reads p as one of the two words; ok is false for any other.
func (c *lClause) monarchCtrlProp(p string) (ok bool, err error) {
	switch p {
	case monarchCtrlWord:
		return true, c.setCmp(cmpMonarchCtrl, 0, monarchCtrlIs, "", "")
	case monarchEnchNotWrd:
		return true, c.setCmp(cmpMonarchCtrl, 0, monarchCtrlIsNot, "", "")
	}
	return false, nil
}

// monarchCtrlCheck refuses the enchanted-controller spelling outside a clause
// that also names EnchantedBy: only there is the enchanted controller the
// candidate's own.
func (c *lClause) monarchCtrlCheck() error {
	if c.CmpStat == cmpMonarchCtrl && c.CmpN == monarchCtrlIsNot && c.Flags&fEnchantedBy == 0 {
		return fmt.Errorf("%s without EnchantedBy", monarchEnchNotWrd)
	}
	return nil
}
