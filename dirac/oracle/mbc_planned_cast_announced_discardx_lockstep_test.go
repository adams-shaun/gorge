package oracle

import (
	"testing"
)

// Turbulent Dreams' additional cost is an announced Discard<X/Card> beside no
// printed {X} (Cost$ U U Discard<X/Card/card(s)>, SVar X:Count$xPaid). gorge's
// payment planner declines the shape (rules/pay classify.go
// PlanNonManaAdmissible: part.Announced declines "cost:discard", as it does an
// announced Sac<X>), so under the autopay surface gorge never lists the plain
// cast as a planned play. The kernel's mirror (featview.go
// planDeclinesAddCost) admitted CostDiscard in every form and listed the cast
// from untapped sources -- a priority menu only one engine offered.
func TestMBCTurbulentDreamsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "planned_cast_announced_discardx")
}
