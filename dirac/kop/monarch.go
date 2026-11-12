package kop

// The monarch (CR 725): the per-game designation, the op that hands it on
// and the expression that reads it. dirac/act_monarch.go.

// ExtBecomeMonarch (api BecomeMonarch; gorge effects/game.go
// effBecomeMonarch): the player Obj (RefController, a player target slot or
// RefTriggerPlayer) becomes the monarch (CR 725.3: the one holder). A player
// who already is the monarch becomes nothing: no designation moves and no
// "whenever a player becomes the monarch" trigger fires (gorge emits no
// MonarchChange). No other field.
const ExtBecomeMonarch ExtOp = 259

func init() { extOpNames[ExtBecomeMonarch] = "BecomeMonarch" }

// ExMonarch (high expr kind): N 0, Sub[0] if the controller is the monarch,
// else Sub[1] (gorge Count$Monarch.<yes>.<no>, evalCountBodyDottedMorbidOr
// Monarch, and CheckDefinedPlayer$ You.isMonarch); N 1, Sub[0] if an opponent
// of the controller still in the game is the monarch, else Sub[1]
// (CheckDefinedPlayer$ Opponent.isMonarch). Explicit ordinal, the iota tail
// stays untouched.
const ExMonarch ExprKind = 218

// MonarchOpp is ExMonarch's N reading an opponent of the controller.
const MonarchOpp = 1

// TrigNMonarchBeganTurn in Trigger.N of a TrigBecomeMonarch trigger is
// BeginTurn$ You: the trigger fires only if its controller was the monarch as
// the turn began (gorge trigmatch.BecomeMonarchMatches, the TurnStartMonarch
// snapshot folded at every TurnChange).
const TrigNMonarchBeganTurn int16 = 1
