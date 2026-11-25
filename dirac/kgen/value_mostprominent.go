package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func init() { exprKinds[vMostProminentType] = kop.ExMostProminentCreatureType }

// mostProminentValue lowers Count$MostProminentCreatureType <spec> (gorge
// mostProminentCreatureType, count_cov3.go): the size of the largest group of
// battlefield objects matching the spec that share a creature type, a
// Changeling in every group. The kernel's ExMostProminentCreatureType answers
// it over the spec's filter (dirac/value_mostprominent.go).
// A bare head reads "Creature", as gorge does.
func (lc *lowerCtx) mostProminentValue(spec string) (lValue, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		spec = "Creature"
	}
	if strings.ContainsAny(spec, "$,") {
		return lValue{}, refuse("Count$MostProminentCreatureType %q", spec)
	}
	f, err := lc.filter(spec)
	if err != nil {
		return lValue{}, err
	}
	return lValue{Kind: vMostProminentType, F: f}, nil
}
