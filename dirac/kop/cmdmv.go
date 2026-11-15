package kop

// ExCmdGreatestMV: the greatest mana value among the commanders the reader
// ("you", the expression's controller) owns that are on the battlefield or in
// the command zone, 0 with none or outside a Commander game (gorge
// Count$ValidBattlefield,Command Card.IsCommander+YouOwn$GreatestCardManaCost:
// the two zones' matches folded by objectManaValue; Cloudkill, Majestic
// Genesis). A face-down commander on the battlefield reads 0 (CR 708.2).
// Explicit ordinal in the free range below the high kinds; the iota tail stays
// untouched. dirac/value_cmdmv.go.
const ExCmdGreatestMV ExprKind = 217
