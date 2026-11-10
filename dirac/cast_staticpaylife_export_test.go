package dirac

import "testing"

// PayLifeAltScenario is a position with a pay-life library static in force
// `emblems` times and the card on top (for the canon package's tests).
func PayLifeAltScenario(t *testing.T, emblems uint8) (*State, Obj) {
	sc, o := payLifeTwoScenario(t, emblems)
	return sc.s, o
}
