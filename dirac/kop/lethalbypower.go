package kop

// StatLethalByPower is a Continuous static's LethalDamageByPower$ True
// (Zilortha, Strength Incarnate: "Lethal damage dealt to creatures you
// control is determined by their power rather than their toughness"): for
// each creature matching Filter, every lethal-damage read uses its power in
// place of its toughness (Forge Card.getLethal; gorge
// Engine.LethalDamageThreshold): the CR 704.5g sweep, an attacker's
// lethal-assignment minimum (CR 510.1c) and the CR 120.10 excess. CR 704.5f
// (toughness 0 or less) still reads toughness. Kernel half:
// dirac/static_lethalbypower.go. Explicit ordinal past StatProtYourColors
// (259).
const StatLethalByPower StaticKind = 260
