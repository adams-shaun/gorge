package kgen

import "fmt"

// counterKindsValue lowers Count$DifferentCounterKinds_<spec>: the distinct
// counter kinds among the battlefield objects matching spec (gorge
// evalCountBodyObjHeads). gorge reads three selectors; only the
// creatures-you-control one lowers, the other two refuse by name until a
// card needs them, and any other spelling refuses as gorge does.
func (lc *lowerCtx) counterKindsValue(spec string) (lValue, error) {
	switch spec {
	case "Creature.YouCtrl":
		f, err := lc.filter(spec)
		if err != nil {
			return lValue{}, err
		}
		return lValue{Kind: vCounterKinds, F: f}, nil
	case "Card.Self", "Permanent.YouCtrl":
		return lValue{}, refuse("Count$DifferentCounterKinds_%s (selector not lowered)", spec)
	}
	return lValue{}, fmt.Errorf("Count$DifferentCounterKinds_%s", spec)
}
