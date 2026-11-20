package kop

// SetID names one SetExpr in Table.Sets. Index 0 is invalid: a SetRef or a
// Ref of RefSet that carries SetID 0 names no set.
type SetID uint16

// SetOrder is the order a set's members are visited in, which is what T07
// resolves from gorge's reader of the same set.
type SetOrder uint8

const (
	// OrderObj visits objects in object index order (a players set: seat order).
	OrderObj SetOrder = iota
	// OrderSeat visits in seat order from the controller (gorge AliveFrom): a
	// players set read as "each player" / "each opponent". For an object set
	// it is the objects grouped by controller in that seat order, each seat's
	// in object index order.
	OrderSeat
	// OrderList visits in the base's own order: the target slot order, the
	// remembered insertion order. It cannot be rebuilt from membership bits,
	// so a suspension-capable op refuses it.
	OrderList
)

// SetOpKind is one op of a SetExpr program. The values are the manifest
// family SetOp (dirac/manifest/SetOp), generated into manifest_gen.go.
type SetOpKind uint8

// SetOp is one step of a SetExpr's postfix program over a stack of sets, at
// most SetMaxDepth deep. Which operands an op reads is on its manifest entry.
type SetOp struct {
	Kind SetOpKind
	// Arg is a kop.Zone (SoBaseValid).
	Arg int16
	// F is the filter SoBaseValid and SoInter match.
	F FilterID
	// R is the ref SoBaseRef reads.
	R Ref
}

// SetMaxDepth is the deepest the evaluation stack of a SetExpr may go; kgen
// refuses a deeper program and the kernel's evalSet keeps that many sets.
const SetMaxDepth = 3

// SetExpr is a program that names any set of objects or players an effect
// acts on. It leaves exactly one set on the stack.
type SetExpr struct {
	Ops []SetOp
	// Players says whether the result holds players or objects. A set of
	// both is refused by kgen.
	Players bool
	Order   SetOrder
}

// SetRef binds an instruction whose Obj is RefSet to its set: Ability.SetRefs.
type SetRef struct {
	// PC is the index of the instruction in Ability.Prog.
	PC  uint8
	Set SetID
	// Flags is SrLKI / SrOncePerController.
	Flags uint8
}

const (
	// SrLKI reads the set's members through last known information: a member
	// that has left the zone it was in is still visited.
	SrLKI uint8 = 1 << iota
	// SrOncePerController visits an op once per distinct controller of the
	// set's members (the op acts on the controller, not on each member).
	SrOncePerController
)

// MaxSetRefs is how many SetRefs an Ability holds: the kernel's setOf is a
// linear scan of at most this many.
const MaxSetRefs = 4

// RefSet is the set reference: the Instr's Obj names a SetExpr, found through
// the ability's SetRefs entry for the instruction's index. An op that reads
// it acts on every member of the set (objects or players) in the set's Order.
// It is a hand constant, high and clear of the appended Ref enum, beside
// RefTgtCtrl1 (241) and RefTriggerOpponents (247).
const RefSet Ref = 248
