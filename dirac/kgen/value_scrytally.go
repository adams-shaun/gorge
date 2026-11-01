package kgen

// Count$YouScryThisTurn / Count$YouSurveilThisTurn lower to
// vScriedThisTurn N 0 / N 1 (countValue, value.go) and ExScriedThisTurn.

// scryTallyOK accepts a bare vScriedThisTurn leaf or a sum of them (the
// carriers' X = Count$YouScryThisTurn/Plus.Y): the controller's own per-turn
// tallies, nothing of the source, so a printed graveyard ability's
// CheckSVar$ X (Proctor of Potential) lowers (graveActCondOK).
func scryTallyOK(v lValue) bool {
	if v.Mul != 0 || v.Div != 0 || v.Add != 0 || v.Neg {
		return false
	}
	switch v.Kind {
	case vScriedThisTurn:
		return true
	case vSum:
		for _, s := range v.Sub {
			if !scryTallyOK(s) {
				return false
			}
		}
		return len(v.Sub) != 0
	}
	return false
}
