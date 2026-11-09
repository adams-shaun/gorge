package kop

// StatCantPreventDamage: while the static functions (a battlefield static
// with Cond2, or an emblem's static: the Effect form, "damage can't be
// prevented this turn") no damage is prevented (CR 615.12): every
// prevention shield and prevention replacement (protection's CR 702.16e
// prevention, a shield counter, the K:Prevent sentences, granted prevention
// keywords, colour and scope prevention records) matches nothing. gorge
// cantPreventDamage with no ValidSource$, IsCombat$ or Affected$ (kgen
// refuses them). An explicit ordinal keeps the iota tail of StaticKind
// untouched. dirac/static_cantprevent.go.
const StatCantPreventDamage StaticKind = 232
