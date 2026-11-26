package kop

// ExtLeaveExile: the exile-instead promise of a move's LeaveBattlefield$
// Exile (Isareth the Awakener, From the Catacombs; gorge registerLeaveExile,
// engine/effects/leavebattlefield.go). The object Obj names, if the move just
// put it onto the battlefield, is exiled instead of going anywhere else when
// it would leave the battlefield; the promise ends with that incarnation
// (gorge's ExileOnMoved sweep). dirac/zone_leaveexile.go.
const ExtLeaveExile ExtOp = 287

func init() { extOpNames[ExtLeaveExile] = "LeaveExile" }
