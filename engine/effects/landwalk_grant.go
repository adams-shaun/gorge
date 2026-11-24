package effects

// Granted landwalk of the host's chosen land type (CR 702.14): Traveler's
// Cloak's `AddKeyword$ Landwalk:ChosenType`. The keyword's land spec names the
// type the HOST (the Aura) chose as it entered, but combat.LandwalkEvades
// judges a DERIVED keyword string on the attacker, whose own ChosenType is
// empty, so the grant is resolved where the static's keyword is built
// (rules/layers_lure.go), the way a granted Protection quality is. A host with
// no recorded choice grants nothing.

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// ResolveGrantedLandwalk rewrites the `Landwalk:ChosenType[:desc]` keywords of
// list to `Landwalk:<host.ChosenType>[:desc]`, and drops them when host chose
// no type; every other keyword passes through unchanged.
func ResolveGrantedLandwalk(list []string, host *state.Object) []string {
	const head = "Landwalk:ChosenType"
	var out []string
	for i, kw := range list {
		rest, ok := strings.CutPrefix(kw, head)
		if !ok || (rest != "" && rest[0] != ':') {
			if out != nil {
				out = append(out, kw)
			}
			continue
		}
		if out == nil {
			out = append(make([]string, 0, len(list)), list[:i]...)
		}
		if host == nil || host.ChosenType == "" {
			continue
		}
		out = append(out, "Landwalk:"+host.ChosenType+rest)
	}
	if out == nil {
		return list
	}
	return out
}
