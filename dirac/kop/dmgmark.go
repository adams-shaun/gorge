package kop

// ExtDmgMark (Lightning, Army of One's Stagger): "until your next turn, if a
// source would deal damage to that player or a permanent that player
// controls, it deals double that damage instead". gorge registers an Effect
// DamageDone replacement (Duration$ UntilYourNextTurn, ValidTarget$
// Player.IsRemembered,Permanent.ControlledBy Player.IsRemembered, any source)
// over the player the trigger captured (RememberObjects$ TriggeredTarget).
// Obj names that player. The kernel keeps one bit per (resolving controller,
// marked player) pair (genState.dmk) and doubles each damage event to the
// marked player or a permanent they control once per set pair; the controller's
// pairs end at the cleanup before their next turn begins (the boundary of
// every UntilYourNextTurn effect). dirac/repl_dmgmark.go.
//
// The ordinal is 317: 316 is ExtCopyPicked's.
const ExtDmgMark ExtOp = 317

func init() { extOpNames[ExtDmgMark] = "DmgMark" }
