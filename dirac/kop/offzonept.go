package kop

// StatOffZonePT is The Muckslinger Gang's "Creature spells you control and
// creature cards you own in any zone other than the battlefield or the stack
// get +A/+B" (S:Mode$ Continuous | AffectedZone$ All, the one Affected$
// spelling dirac/kgen lower_offzonept.go accepts). The kernel models no
// off-battlefield characteristic change. While the static is in force, a read
// of an affected card's power or toughness off the battlefield stalls
// unmodelled (dirac/static_offzonept.go), so a game never plays the printed
// value where the CR 611.3 boost applies. A battlefield permanent is never
// affected. Explicit ordinal: the StaticKind iota tail stays untouched.
const StatOffZonePT StaticKind = 258
