package kop

// CumulativeScaled is ExtCumulative's B for a cumulative upkeep whose cost is
// coloured or hybrid mana (Phobian Phantasm's {B}, Yavimaya Ants' {G}{G},
// Arctic Nishoba's {G/W}; CR 702.24a). The UnlessPaid behind it carries the
// cost of ONE age counter (generic A, packed pips B, hybrid pips in F, as a
// trigger's own PayMana), and the kernel multiplies all three by the age
// counters now on the source when it poses the ask (dirac/trig_cumulativepay.go).
// A generic-only cost keeps the plain form (A = age counters times the
// generic amount, B = 0).
const CumulativeScaled = 2
