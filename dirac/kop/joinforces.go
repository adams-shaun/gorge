package kop

// ExtJoinForces (join forces: "each player may pay any amount of mana; X is
// the total paid this way", Collective Voyage, Minds Aglow, Alliance of Arms,
// Shared Trauma, Mana-Charged Dragon): adds B (an amount, the repeated
// player's chosen number, 0 when they chose none) to the resolution's
// join-forces total, which ExJoinForces reads. C = JoinForcesReset instead
// zeroes the total (the chain's trailing StoreSVar$ 0); an add of 0 is not a
// reset. The total lives in the resolution scratch (armRes.jf) and
// starts at 0 with each resolution, so a read is exact only within the chain
// that wrote it; kgen refuses any other read. Saturates at 32767.
// dirac/extop.go.
const ExtJoinForces ExtOp = 301

// JoinForcesReset is ExtJoinForces C's reset flag.
const JoinForcesReset = 1

func init() { extOpNames[ExtJoinForces] = "JoinForces" }
