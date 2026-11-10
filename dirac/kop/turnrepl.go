package kop

// ExtTurnRepl: an Effect's until-end-of-turn replacement that names a player's
// side of the board rather than one object. C selects which:
//
//	TurnReplExileOpp: "if a creature an opponent controls would die this turn,
//	exile it instead" (Malicious Eclipse's Event$ Moved | ValidLKI$
//	Creature.OppCtrl | Origin$ Battlefield | Destination$ Graveyard). It also
//	catches an opposing creature that enters after the Effect resolved, which
//	the per-slot ExtExileExMark cannot. The kernel keeps one scope record in
//	its exile-instead table (cold exMark); dieZone reads it beside the marks.
//	TurnReplPlusCreatureCtr: "if one or more +1/+1 counters would be put on a
//	creature you control, put that many plus one +1/+1 counters on it instead"
//	(Prairie Dog's Event$ AddCounter | ValidCard$ Creature.YouCtrl |
//	ValidCounterType$ P1P1), a per-player count in the cold buffer beside
//	ExtCounterPlusEOT's genState.plusCtr.
//
// No other field. dirac/repl_turnrepl.go.
//
// The ordinal is 318: 316 is ExtCopyPicked's and 317 ExtDmgMark's.
const ExtTurnRepl ExtOp = 318

// The C values of ExtTurnRepl.
const (
	TurnReplExileOpp = 1 + iota
	TurnReplPlusCreatureCtr
)

func init() { extOpNames[ExtTurnRepl] = "TurnRepl" }
