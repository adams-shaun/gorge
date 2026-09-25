// kw:Crew's registration marker.
//
// The keyword's parser lives in cards/kw_crew.go: it expands K:Crew into an
// AB$ Animate whose tapXType<Any/Creature.Other+withTotalPowerGE<N>> cost
// carries the set-level "total power N or greater" floor. That floor is
// implemented in the SHARED tap-cost machinery -- rules/mana.go's
// stripGroupPowerFloor, rules/cast.go's offer gate and tap election, and
// decision's MinSum/Value wire contract -- machinery Mossbridge Troll's
// cost rides too, so there is no crew-specific effect function to register.
// This file is the coverage marker effects.Supported() feeds the coverage
// report and the rules/acceptance_test.go ratchet from, following the
// effects/attach.go precedent of registering keyword primitives whose
// behaviour is implemented rules-side.
package effects

func init() { RegisterNonAPI("kw:Crew") }
