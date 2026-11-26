package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The cycling alternative cost (CR 118.9; kop.StatCyclingAlt): New
// Perspectives' "As long as you have seven or more cards in hand, you may pay
// {0} rather than pay cycling costs". gorge offers the activator a second
// option for each of their cycling and typecycling abilities (AltCostIndex 1,
// activate_static_altcost.go abilityAlternateCostFor), priced at the static's
// Cost$, the discard alone.
//
// The kernel prices an activation off its ability's Cost at every stage, so
// the alternative is the ability's twin (kop.Ability.CycAltOf, kgen
// build_cyclingalt.go): the same body with the discard as its only cost,
// offered by priorityHandAct beside the printed one exactly while a static
// of the hand owner's holds. Nothing is stamped on the card or in State: the
// static is read live, as kw_cyclinggrant.go reads a granted cycling.
//
// Not modelled, as in gorge: a cycling granted by a static (priorityCyclingGrant)
// is offered at its own cost only.

// cyclingAltLive reports whether a StatCyclingAlt static p controls is in
// force: its Cond2 (the hand-count gate) holds, read against its controller
// as gorge's checkSVarHolds reads it. The static is the battlefield
// permanent's (ValidPlayer$ You): only its controller's activations take it.
//
//go:noinline
func (s *State) cyclingAltLive(k *prep, p uint8) bool {
	live := false
	s.eachStaticTS(k, sclsOffKwG, func(st statSrc) bool {
		if st.a.Kind != kop.AbStatic || st.a.Static.Kind != kop.StatCyclingAlt || st.ctrl != p {
			return true
		}
		live = s.statCond(k, st)
		return !live
	})
	return live
}

// cyclingTwinOffered reports whether the hand ability a is offered at all: a
// printed ability always is, a twin only while p's cycling alternative cost is
// live (memo: 0 unknown, 1 live, 2 not, per priorityHandAct walk).
func (s *State) cyclingTwinOffered(k *prep, p uint8, a *kop.Ability, memo *uint8) bool {
	if a.CycAltOf == 0 {
		return true
	}
	if *memo == 0 {
		*memo = 2
		if s.cyclingAltLive(k, p) {
			*memo = 1
		}
	}
	return *memo == 1
}
