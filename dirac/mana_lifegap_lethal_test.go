package dirac

import "testing"

// The life plan never reaches the payer's life (gorge's lethal guard, CR
// 119.4): Mountain plus one Ogre at 3 life would pay the {2} cast with the
// 3-life activation and die, so the planner has no such plan and the cast is
// not offered. The 4-life control stays offered.
func TestLifeGapAutopayLethalPlanNotOffered(t *testing.T) {
	ctl, stoneCtl, _ := lifeGapAutopayScen(t, 1, 1, 4)
	if !offersCast(ctl.s, stoneCtl) {
		t.Fatal("precondition: Mountain plus one Ogre at 4 life does not offer the {2} cast")
	}
	sc, stone, _ := lifeGapAutopayScen(t, 1, 1, 3)
	if offersCast(sc.s, stone) {
		t.Fatal("the {2} cast is offered at 3 life where the only plan pays 3 life (lethal)")
	}
}
