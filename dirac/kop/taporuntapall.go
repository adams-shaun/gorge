package kop

// ExtTapOrUntapAll (Turnabout; gorge effTapOrUntapAll, Forge
// TapOrUntapAllEffect): the resolving controller elects once, tap or untap
// (a ChGenTapOrUntapAll ask, asked even over an empty sweep), then every
// battlefield permanent matching F is tapped (OpTapAll's walk: the untapped
// ones, one tapping action) or untapped (OpUntapAll's walk: the tapped ones,
// a stun counter removed instead, one untapping action). A ValidTgts$ Player
// scope is F's TargetedPlayerCtrl word. No other field is read.
// dirac/act_taporuntapall.go.
//
// The ordinal is 295, the highest free value below NumExtOps (296 is
// ExtGrantKwAll's, 297 ExtChooseCardEnter's).
const ExtTapOrUntapAll ExtOp = 295

func init() { extOpNames[ExtTapOrUntapAll] = "TapOrUntapAll" }
