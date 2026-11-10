package kgen

import (
	"strconv"
	"strings"
)

// digNoReveal reads Dig's NoReveal$ (gorge dig_params RevealWin = Reveal$
// True && !NoReveal$ True): True cancels the Reveal$ flag (the window stays
// a private look), False is the default; anything else refuses.
func digNoReveal(ps *params) (bool, error) {
	v, ok := ps.get("NoReveal")
	if !ok {
		return false, nil
	}
	switch v {
	case "True":
		return true, nil
	case "False":
		return false, nil
	}
	return false, refuse("NoReveal %q", v)
}

// digRestTop reads Dig's LibraryPosition2$ (gorge dig_params: an omitted
// or Library DestinationZone2$ defaults it to -1, the bottom). "0" keeps
// the rest on top (kop.DigRestTop) -- exact only while at most one card
// can remain: with two or more still on top gorge poses its
// digTopOrderAsk arrange, which the kernel's DigRestTop does not, so that
// shape refuses. At most one remains when DigNum$ is 1, or when a
// mandatory take of every card but one has no ChangeValid$ to miss.
func digRestTop(ps *params, n, take lValue, dest2 int) (bool, error) {
	v, ok := ps.get("LibraryPosition2")
	if !ok {
		return false, nil
	}
	if dest2 != zLibrary {
		return false, refuse("LibraryPosition2 %q without a library DestinationZone2", v)
	}
	switch v {
	case "-1":
		return false, nil
	case "0":
	default:
		return false, refuse("LibraryPosition2 %q", v)
	}
	if n.Kind != vConst || ps.is("FromBottom", "True") || ps.is("RestRandomOrder", "True") {
		return false, refuse("LibraryPosition2 0 of a non-constant or reordered window")
	}
	_, cv := ps.m["ChangeValid"]
	_, opt := ps.m["Optional"]
	if n.N <= 1 || take.Kind == vConst && take.N >= 1 && !cv && !opt && n.N-take.N <= 1 {
		return true, nil
	}
	return false, refuse("LibraryPosition2 0 with two or more cards left on top (gorge digTopOrderAsk)")
}

// digImprint reads Dig's Imprint$ True (gorge effDig: each nontoken card
// moved by the dig is imprinted on the source, an events.Imprint). On an
// instant or sorcery whose script never reads the imprinted set (no SVar
// names Imprint), the record dies with the spell as it leaves the stack,
// so the parameter has no observable effect and is consumed. Any other
// carrier refuses, bar the permanent shape digImprintPermanent reads; the
// result is true when the Dig needs the imprint marker after it.
func (lc *lowerCtx) digImprint(ps *params, dest int, take lValue) (bool, error) {
	v, ok := ps.get("Imprint")
	if !ok {
		return false, nil
	}
	if strings.TrimSpace(v) != "True" {
		return false, refuse("Dig Imprint$ %q", v)
	}
	spell := false
	for _, t := range lc.face.Types {
		if t == "Instant" || t == "Sorcery" {
			spell = true
		}
	}
	if !spell {
		return true, lc.digImprintPermanent(ps, dest, take) // lower_digimprintset.go
	}
	for _, sv := range lc.face.SVars {
		if strings.Contains(sv, "Imprint") {
			return false, refuse("Dig Imprint$ read by the script")
		}
	}
	return false, nil
}

// pickTotalCMC reads WithTotalCMC$: a cumulative printed-mana-value budget
// over the cards a pick takes (gorge effDig's and effHiddenPick's MaxSum).
// The kernel carries the bound in the pick filter's head
// (kop.Filter.PickSumMax), so it must be a positive constant; an SVar or X
// value refuses by name. The distinct-name walks own their own pick-set
// state, so the parameter refuses beside DifferentNames$.
func pickTotalCMC(ps *params) (lValue, bool, error) {
	v, ok := ps.get("WithTotalCMC")
	if !ok {
		return lValue{}, false, nil
	}
	if _, bad := ps.m["DifferentNames"]; bad {
		return lValue{}, false, refuse("WithTotalCMC$ beside DifferentNames$")
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return lValue{}, false, refuse("WithTotalCMC$ %q is not a positive constant", v)
	}
	return cst(n), true, nil
}

// digDifferentPowers reads Dig's WithDifferentPowers$ True: the take must
// have pairwise different printed powers (gorge digGreedyTake /
// digSetOptionPowers). The flag rides the pick filter's head
// (kop.Filter.PickDistinctPower); the distinct-name walk is not advised of
// it, so it refuses beside DifferentNames$.
func digDifferentPowers(ps *params) (bool, error) {
	v, ok := ps.get("WithDifferentPowers")
	if !ok {
		return false, nil
	}
	if _, bad := ps.m["DifferentNames"]; bad {
		return false, refuse("WithDifferentPowers$ beside DifferentNames$")
	}
	if strings.TrimSpace(v) != "True" {
		return false, refuse("Dig WithDifferentPowers$ %q", v)
	}
	return true, nil
}

// digBoundShape refuses a pick-set bound on the Dig shapes whose kernel arm
// never asks the per-card pickFits: ChangeNum$ All (every matching card is
// taken at once, and its Optional$ election is one yes/no, where gorge drops
// the all-or-nothing election and takes its bounded greedy instead), and a
// Dig over each player or each opponent (the kernel's multi-seat walk takes
// only forced windows, and a bounded take can leave a choice).
func digBoundShape(who lRef, take lValue, hasBudget, distinct bool) error {
	if !hasBudget && !distinct {
		return nil
	}
	name := "WithTotalCMC$"
	if !hasBudget {
		name = "WithDifferentPowers$"
	}
	if take.Kind == vConst && take.N == -1 {
		return refuse("%s beside ChangeNum$ All", name)
	}
	if who.Kind == rEachPlayer || who.Kind == rEachOpp {
		return refuse("%s on an each-player Dig", name)
	}
	return nil
}

// pickConstraintFilter puts the read constraints on a pick's filter head,
// creating the match-all clause a lone bound needs (a Dig with no
// ChangeValid$). A pick with neither constraint keeps its filter.
func pickConstraintFilter(f *lFilter, budget lValue, hasBudget, distinct bool) *lFilter {
	if !hasBudget && !distinct {
		return f
	}
	if f == nil {
		f = &lFilter{Clauses: []lClause{{}}}
	}
	if hasBudget {
		b := budget
		f.PickSumMax = &b
	}
	f.PickDistinctPower = distinct
	return f
}
