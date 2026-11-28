package kop

import "strconv"

// Set expressions (design 2026-11-19-dirac-operand-algebras, K3): a
// set-valued operand. An instruction whose Obj is RefSet names a SetExpr of
// Table.Sets through its ability's SetRefs entry for the instruction's PC; the
// kernel evaluates the expression when the instruction runs (dirac
// value_setexpr.go evalSet) and runs the op once per member.
//
// A SetExpr is a postfix program: a base kind pushes a set, a combinator pops
// its operands and pushes the result, and the program leaves exactly one set.
// A set holds objects and players. Its order, when an op walks it, is object
// index order, then the seats in turn order from the resolving controller
// (gorge's AliveFrom); a target base keeps its slot order (gorge's
// chainTargetUnion) until a combinator other than SetAndFilter consumes it.

// SetOpKind is one step of a SetExpr. Append-only: tables store the values.
type SetOpKind uint8

// The push-base kinds the kernel evaluates.
const (
	SetNone          SetOpKind = 0  // never valid in a program
	SetTarget        SetOpKind = 1  // target slot Arg's legal objects and players, in slot order (gorge Defined$ Targeted on one slot)
	SetTargets       SetOpKind = 2  // the union of the target slots in mask Arg, in slot order (gorge's chain target union)
	SetSelf          SetOpKind = 3  // the resolving source object (Defined$ Self)
	SetController    SetOpKind = 4  // the resolving controller (Defined$ You)
	SetTriggerObj    SetOpKind = 5  // the object that caused the trigger (Defined$ TriggeredCard)
	SetTriggerPlayer SetOpKind = 6  // the player the trigger event names (Defined$ TriggeredPlayer)
	SetTriggerSource SetOpKind = 7  // the source of the event that caused the trigger (Defined$ TriggeredSource)
	SetEachOpponent  SetOpKind = 8  // every opponent of the controller still in the game (Defined$ Opponent)
	SetEachPlayer    SetOpKind = 9  // every player still in the game (Defined$ Player)
	SetRemembered    SetOpKind = 10 // declared: the remembered set (its own ticket; evalSet refuses it)
	SetImprinted     SetOpKind = 11 // declared: the source's imprinted cards
	SetExiledWith    SetOpKind = 12 // declared: the cards exiled with the source
	SetChosen        SetOpKind = 13 // declared: the chosen cards
	SetAttached      SetOpKind = 14 // declared: what the source is attached to
	SetSacrificed    SetOpKind = 15 // declared: what the cost sacrificed
	SetDiscarded     SetOpKind = 16 // declared: what the cost discarded
)

// The combinators.
const (
	SetAndFilter         SetOpKind = 32 // pop s; push s's objects that match filter Arg (a FilterID, which may be a PredProg); s's players are dropped
	SetUnion             SetOpKind = 33 // pop b, a; push a ∪ b (index order: a union keeps no slot order)
	SetPlayersComplement SetOpKind = 34 // pop s; push the players still in the game that s does not hold (gorge's Non prefix over players); s's objects are dropped
	SetControllerOf      SetOpKind = 35 // pop s; push the controller of each of s's objects, and s's players (a player is its own)
	SetOwnerOf           SetOpKind = 36 // pop s; push the owner of each of s's objects, and s's players
	SetLKIOf             SetOpKind = 37 // declared: s's objects as they last existed (its own ticket; evalSet refuses it)
	SetSpawnerOf         SetOpKind = 38 // declared: the objects that spawned s's objects
)

// MaxSetDepth is the deepest operand stack a SetExpr may need: the kernel
// evaluates on a fixed array of this many sets.
const MaxSetDepth = 4

// SetOp is one SetExpr step: its kind and its argument (a target slot or slot
// mask, a FilterID; 0 for the kinds that take none).
type SetOp struct {
	Kind SetOpKind
	Arg  uint16
}

// SetExpr is a postfix program over SetOps.
type SetExpr []SetOp

// SetRef names the set an instruction of an ability reads: the instruction at
// PC of the ability's Prog has Obj RefSet, and Set indexes Table.Sets.
type SetRef struct {
	PC  uint8
	Set uint16
}

// SetPushes reports whether k pushes a base (it pops nothing).
func (k SetOpKind) SetPushes() bool { return k >= SetTarget && k <= SetDiscarded }

// SetArity is the number of sets k pops: 0 for a base, 2 for SetUnion, 1
// for every other combinator.
func (k SetOpKind) SetArity() int {
	switch {
	case k.SetPushes():
		return 0
	case k == SetUnion:
		return 2
	}
	return 1
}

// SetDepth is the operand-stack depth e needs, and false when e is not a
// well-formed program (an unknown kind, a pop of an empty stack, or other than
// one set left at the end).
func (e SetExpr) SetDepth() (int, bool) {
	d, deep := 0, 0
	for _, op := range e {
		if op.Kind == SetNone || op.Kind > SetSpawnerOf || op.Kind > SetDiscarded && op.Kind < SetAndFilter {
			return 0, false
		}
		n := op.Kind.SetArity()
		if d < n {
			return 0, false
		}
		d = d - n + 1
		if d > deep {
			deep = d
		}
	}
	return deep, d == 1
}

var setOpNames = [...]string{
	SetNone: "SetNone", SetTarget: "SetTarget", SetTargets: "SetTargets", SetSelf: "SetSelf",
	SetController: "SetController", SetTriggerObj: "SetTriggerObj", SetTriggerPlayer: "SetTriggerPlayer",
	SetTriggerSource: "SetTriggerSource", SetEachOpponent: "SetEachOpponent", SetEachPlayer: "SetEachPlayer",
	SetRemembered: "SetRemembered", SetImprinted: "SetImprinted", SetExiledWith: "SetExiledWith",
	SetChosen: "SetChosen", SetAttached: "SetAttached", SetSacrificed: "SetSacrificed", SetDiscarded: "SetDiscarded",
	SetAndFilter: "SetAndFilter", SetUnion: "SetUnion", SetPlayersComplement: "SetPlayersComplement",
	SetControllerOf: "SetControllerOf", SetOwnerOf: "SetOwnerOf", SetLKIOf: "SetLKIOf", SetSpawnerOf: "SetSpawnerOf",
}

// String is k's constant name ("SetKind(n)" for a value with none).
func (k SetOpKind) String() string {
	if int(k) < len(setOpNames) && setOpNames[k] != "" {
		return setOpNames[k]
	}
	return "SetKind(" + strconv.Itoa(int(k)) + ")"
}
