//go:build dirac_mp

package dirac

// The dirac_mp size class: four seat slots. The size knobs that depend on the
// seat count live here and in seats_default.go; every other file derives from
// them.
const MaxSeats = 4

// turnExhaustShift is the first bit of a player's Exhaust-activation fact in
// genState.turnF: bits 0-3 hold the coin flips, bits 4-7 the Exhaust
// activations (static_turnbits.go).
const turnExhaustShift = 4

// TCode is a target code: an Obj, a stack ability, a player or none, as an
// Action's source and target fields and the stack's target records hold it.
// Its width is the class's object width, so a code never passes through a
// narrower integer (TestObjCodeCastsOnlyShrink).
type TCode uint16

// Action's packing (types.go): kind 5 | source 9 | target 9 | mode 8 bits,
// kind<<26 | source<<17 | target<<8 | mode, so a code may reach 511.
const (
	actKindShift = 26
	actSrcShift  = 17
	actTgtShift  = 8
	actCodeMask  = 0x1ff
)

// The target-code space (types.go), nine bits: objects 0..MaxObj-1, stack
// abilities StackTarget0..StackTarget0+15 (16 codes, as in the default
// class), players PlayerTarget0..PlayerTarget0+3 (one code per seat), 508
// and 509 unused, 510 a departed stack target and 511 none. MaxObj 488 is a
// four-seat Commander pod's measured object peak (480; p99 452) with
// headroom, and keeps [MaxObj] byte arrays 8-aligned.
const (
	MaxObj                = 488 // card instances: the decks + tokens (at most StackTarget0)
	StackTarget0    TCode = 488 // the bottom stack ability's target code
	PlayerTarget0   TCode = 504 // seat 0's target code
	StackTargetGone TCode = 510 // a departed stack target (tgt_stackability.go)
	NoTarget        TCode = 511 // no target
)

// Obj is a card instance (types.go): nine bits under dirac_mp.
type Obj uint16

// NoObj is no object, the same value as NoTarget.
const NoObj Obj = 511

// maxEntered is the zone entries one turn's ledger records
// (value_enteredledger.go); the next stalls StallEntered. Four seats move
// four boards: in four-seat 100-card MBC pods under max^n search 64 stalled 6
// of 264 pods (2.3%), and over every searched and played turn of those pods
// (about 160,000 that reached 16 entries) 2,238 reached 64, 70 reached 80, 7
// reached 96, one 112 and none 128. Packed to four bytes an entry, 96 costs what 64
// did at six.
const maxEntered = 96

// object.acts packs the owner above this turn's activation count: two owner
// bits for four seats, so the count saturates at 63 (cast.go stalls with
// StallActs rather than wrapping).
const (
	objActsMask   uint8 = 0x3f // object.acts: this turn's non-mana activation count
	objOwnerBits  uint8 = 0xc0 // object.acts: the owner (0..3)
	objOwnerShift       = 6
)

// ovRec.fl (gen.go) packs the controller an UntilYourNextTurn override
// record waits for (two seat bits) above the merge count, which then caps at
// 16 (addOvRec moves on to a new record once the count saturates).
const (
	ovNTWho      uint8 = 0x30 // ovNT record: the waiting controller (0..3)
	ovNTWhoShift       = 4
	ovReps       uint8 = 0x0f // the merge count's mask
)

// maxXTgt is how many objects a stack entry's multi-object target slots hold
// beyond the first (xTargets); a pick past it stalls StallXTgt. xtMask holds
// a bit per extra and one more (xCrimRec). Four seats put three opponents'
// boards in reach: a "divided as you choose among any number of target
// creatures" spell (Monstrous Onslaught, X = the greatest power you control)
// cast by the heuristic bot into a four-seat Commander pod picked a ninth
// creature in 18 of 300 pods, so the class holds sixteen objects.
const maxXTgt = 15

type xtMask = uint16

// embRemSlots is how many objects one emblem remembers (static_emblemrem.go):
// the frozen-number byte and embR. The four-seat class has no reserved bytes
// left for embX, so it holds two and a larger set stalls (StallEmbRem).
const embRemSlots = 2

// linkPadN pads genState after its sixteen exile-until-leaves links so perms
// keeps its cache-line phase (statelayout_mp_test.go): forty-eight bytes here
// bring four seats of player.lastLost, the two-byte seatWord fields, the
// sixteen-object xTargets, the sixteen-object free-cast arrays (maxFree) and
// the class's sixteen-bit codes round to phase 0.
const linkPadN = 48

// seatWord is a packed field holding a few bits per seat beside its other
// bits (genState.speed, genState.freeP): two bytes at four seats.
type seatWord = uint16

// objWord is a multi-use field that may hold an object (or object+1) as well
// as a byte-sized value (seats_default.go); the object width.
type objWord = uint16

// mayWord is mayPlay.turn (seats_default.go); mayHeld keeps an object below
// its version, mayVerShift bits up.
type mayWord = uint32

const mayVerShift = 16

// maxFree is how many cast-without-paying candidates (genState.free, freeAll)
// and queued casts (freeQ) one resolution holds; a larger pool stalls
// StallFree. The four-seat class holds sixteen: a cast-without-paying effect
// over a 100-card deck's graveyard and exile found more than eight candidates
// in 4-seat cmd5 pods, none at 50 cards.
const maxFree = 16
