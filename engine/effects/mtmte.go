// kw:More Than Meets the Eye and kw:Living metal registration markers.
//
// More Than Meets the Eye (CR 702.162a) is an alternative cost to cast a
// double-faced card converted: its behaviour lives in engine/rules (the
// mtmteCast reader in cast_altcost.go, the hand walk's offer in
// legal_walk_hand.go and beginCast's "converted" mode), so there is no effect
// function to register. Living metal (CR 702.161a) is expanded into a layer-4
// type-adding static by engine/cards/kw_livingmetal.go and read by the layer
// walk. This file is the coverage marker effects.Supported() feeds the
// coverage report and the engine/rules/acceptance_test.go ratchet from,
// following the engine/effects/crew.go precedent.

package effects

func init() { RegisterNonAPI("kw:More Than Meets the Eye", "kw:Living metal") }
