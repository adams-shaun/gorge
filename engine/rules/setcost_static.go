package rules

import "github.com/adams-shaun/gorge/engine/effects"

// stat:SetCost is the printed S:Mode$ SetCost static in its one corpus shape,
// Trinisphere's RaiseTo$ True floor ("each spell that would cost less than
// three mana to cast costs three mana to cast"). The cost scan collects it
// beside RaiseCost/ReduceCost (statics_costscan.go scanCostStatics), the
// payment planner and the offer path price it through the same composition
// (statics_costapply.go: applied after every other cost change, a SetCost
// without RaiseTo$ True is not read), and pinned by
// TestTrinisphereSetCostFloor and TestEDHStaticsATrinisphere.
func init() { effects.RegisterNonAPI("stat:SetCost") }
