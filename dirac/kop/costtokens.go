package kop

// PayBSubDamage: a PayDraw unless-cost whose B.N carries this sentinel is an
// UnlessCost$ DamageYou<X>: paying is the resolving source dealing A damage
// to the payer (Combustion Man), not a draw. A is the folded X; a zero or
// negative X leaves the offer decline-only. C's nibble is full, so the
// sentinel rides the existing Instr.B value.
const PayBSubDamage = 14

// PayBSubMill: a PayDraw unless-cost whose B.N carries this sentinel is an
// UnlessCost$ Mill<A>: paying is the payer milling A cards from the top of
// their own library through the mill payment (CR 701.17b, 118.3; gorge
// payUnlessCost over pay.PayMillCost, Deep Spawn), not a draw. The offer gate
// prices the library (pay.MillCostPayable), so a short library leaves the
// offer decline-only. C's nibble is full, so the sentinel rides the existing
// Instr.B value; 17 is clear of every other PayBSub* member
// (TestKopSentinelFamiliesDistinct).
const PayBSubMill = 17
