package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func init() { exprKinds[vMostCardName] = kop.ExMostCardName }

// mostCardNameValue lowers Count$MostCardName <spec> (gorge count_ref.go
// countMostCardName, Forge's Count$MostCardName): the greatest number of
// battlefield objects matching the spec that share one face name, the oracle's
// "N or more <things> with the same name as one another" (Endless Atlas,
// Mechanized Production, Chrome Replicator, Sceptre of Eternal Glory). The
// kernel's ExMostCardName answers it over the spec's filter, every seat's
// battlefield with the spec's controller-relative words read against the
// evaluating controller (dirac/value_mostcardname.go), as gorge's zone-aware
// spec matcher does. An aggregate or comma alternative stays refused: the
// corpus's four carriers name one plain battlefield spec each.
func (lc *lowerCtx) mostCardNameValue(spec string) (lValue, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" || strings.ContainsAny(spec, "$,") {
		return lValue{}, refuse("Count$MostCardName %q", spec)
	}
	f, err := lc.filter(spec)
	if err != nil {
		return lValue{}, err
	}
	return lValue{Kind: vMostCardName, F: f}, nil
}
