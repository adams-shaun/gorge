//go:build !dirac_mp

package dirac

// The default size class: two seats. The size knobs that depend on the seat
// count live here and in seats_mp.go; every other file derives from them.
const MaxSeats = 2

// turnExhaustShift is the first bit of a player's Exhaust-activation fact in
// genState.turnF: bits 0-1 hold the coin flips, bits 2-3 the Exhaust
// activations (4-7 stay free; static_turnbits.go).
const turnExhaustShift = 2

// TCode is a target code: an Obj, a stack ability, a player or none, as an
// Action's source and target fields and the stack's target records hold it.
// Its width is the class's object width, so a code never passes through a
// narrower integer (TestObjCodeCastsOnlyShrink).
type TCode uint8

// Action's packing (types.go): kind<<24 | source<<16 | target<<8 | mode,
// eight bits for each code.
const (
	actKindShift = 24
	actSrcShift  = 16
	actTgtShift  = 8
	actCodeMask  = 0xff
)

// The target-code space (types.go): objects 0..MaxObj-1, stack abilities
// StackTarget0..PlayerTarget0-1 (16 codes), players PlayerTarget0.. (one
// code per seat), 0xFE a departed stack target and 0xFF none.
const (
	MaxObj                = 236  // card instances: both decks + tokens (at most StackTarget0)
	StackTarget0    TCode = 0xEC // the bottom stack ability's target code
	PlayerTarget0   TCode = 0xFC // seat 0's target code
	StackTargetGone TCode = 0xFE // a departed stack target (tgt_stackability.go)
	NoTarget        TCode = 0xFF // no target
)

// Obj is a card instance (types.go): a byte in the default class.
type Obj uint8

// NoObj is no object.
const NoObj Obj = 255

// maxEntered is the zone entries one turn's ledger records
// (value_enteredledger.go); the next stalls StallEntered. Synthetic Standard
// pools reach 47 at 300 pairs; a loop can reach more, and stalls.
const maxEntered = 64

// object.acts packs the owner above this turn's activation count: one owner
// bit for two seats.
const (
	objActsMask   uint8 = 0x7f // object.acts: this turn's non-mana activation count
	objOwnerBits  uint8 = 0x80 // object.acts: the owner (0 or 1)
	objOwnerShift       = 7
)

// ovRec.fl (gen.go) packs the controller an UntilYourNextTurn override
// record waits for (one seat bit) above the merge count.
const (
	ovNTWho      uint8 = 0x20 // ovNT record: the waiting controller (0 or 1)
	ovNTWhoShift       = 5
	ovReps       uint8 = 0x1f // the merge count's mask
)

// maxXTgt is how many objects a stack entry's multi-object target slots hold
// beyond the first (xTargets); a pick past it stalls StallXTgt. xtMask holds
// a bit per extra and one more (xCrimRec).
const maxXTgt = 7

type xtMask = uint8

// embRemSlots is how many objects one emblem remembers (static_emblemrem.go):
// the frozen-number byte, embR and embX's two.
const embRemSlots = 4

// linkPadN pads genState after its sixteen exile-until-leaves links so perms
// keeps its cache-line phase (statelayout_test.go).
const linkPadN = 16

// seatWord is a packed field holding a few bits per seat beside its other
// bits (genState.speed, genState.freeP): a byte at two seats.
type seatWord = uint8

// objWord is a multi-use field that may hold an object (or object+1) as well
// as a byte-sized value: a version, flags, a paid X. It is the object width,
// a byte here and wider under dirac_mp, so an object stored in it is never
// truncated; a conversion to or from it is no narrowing
// (TestObjCodeCastsOnlyShrink). It is an alias, so the default class's code
// is unchanged.
type objWord = uint8

// mayWord is mayPlay.turn: a turn number, a cost surcharge, or a source
// object below its version (mayHeld), mayVerShift bits up.
type mayWord = uint16

const mayVerShift = 8
