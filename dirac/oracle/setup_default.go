package oracle

// CardIndex is a matchup's table index (Matchup.Table): one byte, which
// every two-seat matchup's distinct cards fit.
type CardIndex = uint8

// NoCardIndex is the table index of no card (a token, a copy).
const NoCardIndex CardIndex = 255
