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
//	TurnReplUncounter, TurnReplUncounterCreature: "spells you control can't be
//	countered this turn" (Veil of Summer's Event$ Counter | ValidSA$
//	Spell.YouCtrl | Layer$ CantHappen) and "creature spells you cast this turn
//	can't be countered" (Domri, Anarch of Bolas's ValidSA$
//	Spell.Creature+wasCastByYou): a scope record in the exile-instead table
//	(cold exMark) naming the controller; the kernel's uncounterable reads it
//	for every spell of that controller, cast before or after the Effect.
//	TurnReplPlayerHex: "you gain hexproof until end of turn", the player half
//	of a Pump's Defined$ You & Valid <permanents> grant (Dawn's Truce,
//	Lazotep Plating, Surge of Salvation): a scope record naming the player; a
//	spell or ability an opponent controls can't target them this turn.
//	TurnReplHexUB: "you and permanents you control gain hexproof from blue and
//	from black until end of turn" (Veil of Summer's Pump KW$ Hexproof:Black &
//	Hexproof:Blue): the same record with the two colours, and every permanent
//	the controller controls as it resolves marked in the arena's mechHexUB
//	plane. A source of either colour its controller's opponents control can't
//	target them (CR 702.11d). Both end at cleanup.
//
// No other field. dirac/repl_turnrepl.go.
//
// The ordinal is 318: 316 is ExtCopyPicked's and 317 ExtDmgMark's.
const ExtTurnRepl ExtOp = 318

// The C values of ExtTurnRepl.
const (
	TurnReplExileOpp = 1 + iota
	TurnReplPlusCreatureCtr
	TurnReplUncounter
	TurnReplUncounterCreature
	TurnReplPlayerHex
	TurnReplHexUB
)

func init() { extOpNames[ExtTurnRepl] = "TurnRepl" }
