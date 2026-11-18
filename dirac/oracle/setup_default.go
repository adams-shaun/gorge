package oracle

// CardIndex is a matchup's table index (Matchup.Table): sixteen bits: four-seat
// pods (singleton Commander decks) hold more than
// 255 distinct cards.
type CardIndex = uint16

// NoCardIndex is the table index of no card (a token, a copy).
const NoCardIndex CardIndex = 0xffff
