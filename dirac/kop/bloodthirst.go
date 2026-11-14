package kop

// ExDmgOppsTaken: the total damage the controller's opponents still in the
// game were dealt this turn (gorge Count$DamageOppsTakenThisTurn, the sum of
// Host.DamageTakenThisTurn over the living opponents: every player-targeted
// Damage event since the turn began, infect damage included, prevented
// damage not). Bloodthirst's reader (CR 702.54a): a fixed N gates on it
// above 0, Bloodthirst X enters with that many counters. Explicit ordinal in
// the free low range; the iota tail stays untouched. Kernel kw_bloodthirst.go.
const ExDmgOppsTaken ExprKind = 66
