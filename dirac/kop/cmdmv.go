package kop

// ExCmdGreatestMV: the greatest mana value among the commanders the reader
// ("you", the expression's controller) owns that are on the battlefield or in
// the command zone, 0 with none or outside a Commander game (gorge
// Count$ValidBattlefield,Command Card.IsCommander+YouOwn$GreatestCardManaCost:
// the two zones' matches folded by objectManaValue; Cloudkill, Majestic
// Genesis). A face-down commander on the battlefield reads 0 (CR 708.2).
// Expr.N CmdMVAllZones widens the fold to every zone (gorge Count$ValidAll:
// the hand, library, graveyard, exile and stack too, a spell's {X} counted,
// CR 202.3e; Tangleweave Armor). Explicit ordinal in the free range below the
// high kinds; the iota tail stays untouched. dirac/value_cmdmv.go.
const ExCmdGreatestMV ExprKind = 217

// CmdMVAllZones is ExCmdGreatestMV's Expr.N for the every-zone fold.
const CmdMVAllZones = 1
