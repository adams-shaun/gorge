// Package policynet is the feature layer of the learned per-option policy:
// a deterministic, dependency-light encoder from one seat's
// view.View plus its decision surface into fixed-width dense scalars and
// hashed sparse rows, and a streaming loader for the search-teacher label
// corpus (the search teacher -labels).
//
// The design contract:
//
//   - Everything is read from the view only — never the record's
//     traceboard board snapshot, never the card corpus registry.
//   - The hash is FNV-1a 64 over the exact feature string, masked to the
//     shared 16,384-row table. It is pinned by a golden test because a
//     trained checkpoint is worthless if the hash drifts.
//   - No map iteration order reaches any output: every map is read by a
//     fixed key order, every counter map is sorted before use, and the
//     sparse bags are emitted in a fixed zone order and then sorted by row.
//
// Package dependencies: state, decision, view, bots/traceboard — nothing
// further; no new module dependencies.
package policynet

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TableRows is the shared hashed-embedding row count: one shared 16,384-row
// table serves the state bag and the option's hashed ids.
const TableRows = 1 << 14

const rowMask = TableRows - 1

// hashID is FNV-1a 64 over the exact string, masked to TableRows. The
// function, the string formats below and the masking are all part of the
// pinned encoding contract: a golden test holds literal row numbers, so any
// drift in any of the three fails the suite loudly.
func hashID(s string) uint16 {
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return uint16(h & rowMask)
}

// HashID is the exported form of the pinned FNV-1a-64-masked feature hash:
// the same function the encoders use internally, for tools that must mint
// feature rows against the same table (the trainer's synthetic corpora, a
// future inference host). Its function, masking and string-format contract
// are pinned by the encoding goldens; a checkpoint is worthless if this
// moves.
func HashID(s string) uint16 { return hashID(s) }

// Feature is one sparse entry: a row in a fixed-width space and its value.
// State sparse features live in the shared TableRows space; option slot
// features live in OptionSlotWidth slots (a separate, dense one-hot space).
type Feature struct {
	Row   uint16
	Value float32
}

// State is one encoded decision-time state: the dense scalar vector and the
// sorted hashed bag of card ids.
type State struct {
	Dense  []float32
	Sparse []Feature
	// Cards is the entity list (FeaturesEntity only; entity.go):
	// one EntityCard per visible card, in a fixed zone order. nil for every
	// other feature set, which therefore encodes exactly as before.
	Cards []EntityCard
}

// Dense section offsets. The layout is a pinned contract (golden test).
const (
	DenseWidth = 92
	// DenseWidthV1 is the 68-wide dense vector every network and training
	// corpus recorded before the Commander block: loaders accept it and read
	// only the first 68 columns, so such a network scores bit-identically.
	DenseWidthV1 = 68

	denseTurn       = 0
	denseTurnBucket = 1
	denseStep0      = 2 // 12 one-hot slots
	numStepSlots    = 12
	densePhase0     = 14 // 5 one-hot slots
	numPhaseSlots   = 5
	denseIsActive   = 19
	denseIsPriority = 20
	denseStackDepth = 21
	densePool0      = 22 // 6: my floating pool WUBRG C
	numPoolSlots    = 6
	denseSeat0      = 28 // maxSeatBlocks blocks of seatBlockWidth
	seatBlockWidth  = 10
	maxSeatBlocks   = 4

	// The Commander block: maxSeatBlocks relative seats of
	// commanderBlockWidth values, appended after the seat blocks.
	denseCmd0           = 68
	commanderBlockWidth = 6
	// Within a seat's 6 values: the three damage columns (from the other
	// seats, in relative turn order from this seat), its casts, its commander
	// on the battlefield, and whether it has left the game.
	denseCmdDamage0 = 0
	denseCmdCasts   = 3
	denseCmdOnField = 4
	denseCmdLost    = 5
)

// stepNames is the fixed step one-hot vocabulary — the same 12 names
// engine/state/ids.go's stepNames renders (view.View.Step is that String()).
var stepNames = [...]string{
	"untap", "upkeep", "draw", "main1",
	"begin-combat", "declare-attackers", "declare-blockers", "combat-damage",
	"end-combat", "main2", "end", "cleanup",
}

// phaseNames is the fixed phase one-hot vocabulary — view.PhaseOf's five
// phases. An unrecognised step/phase string sets no bit at all (never a
// wrong one).
var phaseNames = [...]string{"beginning", "main1", "combat", "main2", "ending"}

// poolKeys is the fixed key order every pool/available map is read in
// (state.Mana's WUBRGC slot order, as the view projects the maps).
var poolKeys = [...]string{"W", "U", "B", "R", "G", "C"}

// Zone tags hashed into the state bag. A card id string is "<name>|<tag>";
// the tags distinguish the same card in different zones.
const (
	zoneTagHand       = "hand"
	zoneTagBattleMe   = "battlefield"
	zoneTagBattleOpp  = "battlefield-opp"
	zoneTagGraveMe    = "graveyard"
	zoneTagGraveOpp   = "graveyard-opp"
	zoneTagExileMe    = "exile"
	zoneTagExileOpp   = "exile-opp"
	zoneTagStack      = "stack"
	zoneTagCommand    = "command"
	zoneTagCommanders = "commanders"
)

// EncodeState encodes the seat's view into the fixed-width dense vector and
// the sorted hashed sparse bag.
//
// Dense layout (92 floats, each section pinned by the golden test):
//
//	 0      turn (raw)
//	 1      turn bucket min(turn, 40)/40
//	 2..13  step one-hot (stepNames order; unknown step = no bit)
//	14..18  phase one-hot (phaseNames order; unknown phase = no bit)
//	19      the viewer seat is active
//	20      the viewer seat has priority
//	21      stack depth
//	22..27  viewer's floating pool by colour (WUBRG C)
//	28..67  four seat blocks of 10 (viewer first, then other seats in
//	        turn order; a block beyond the table's seats stays all-zero):
//	        life, hand size, library size, graveyard size,
//	        battlefield count, creature count, total derived power, total
//	        derived toughness, untapped permanents, untapped lands
//	68..91  four Commander blocks of 6 (same relative seat order; zeros
//	        outside a Commander game except the left-the-game bit):
//	        commander damage from the next three relative seats /21
//	        clamped, commander casts /4 clamped, commander on battlefield,
//	        seat has left the game
//
// Sparse layout (sorted ascending by row, duplicates kept — it is a bag):
// one id per card as "<name>|<zone tag>" over my hand, my battlefield,
// each other battlefield, my graveyard, each other graveyard, my exile, each
// other exile, and one per stack entry ("Name|stack"); every battlefield
// card ALSO hashes a state-decorated id
// "Name|t0|a0|s0|c<counters>" (t=tapped, a=attacking, s=summon-sick,
// counters sorted "Kind=v" joined by ",", "-" when empty), so the model can
// distinguish a tapped Thalia from an untapped one.
func EncodeState(v view.View, seat state.PlayerID) State { return encodeState(nil, &v, seat) }

// encodeState is EncodeState into buf's storage (fresh storage for a nil
// buf).
func encodeState(buf *StateBuf, vp *view.View, seat state.PlayerID) State {
	v := vp
	dense := buf.denseBuf(DenseWidth)
	dense[denseTurn] = float32(v.Turn)
	b := float32(v.Turn)
	if b > 40 {
		b = 40
	}
	dense[denseTurnBucket] = b / 40
	if i := indexOf(stepNames[:], v.Step); i >= 0 {
		dense[denseStep0+i] = 1
	}
	if i := indexOf(phaseNames[:], v.Phase); i >= 0 {
		dense[densePhase0+i] = 1
	}
	if v.Active == seat {
		dense[denseIsActive] = 1
	}
	if v.Priority == seat {
		dense[denseIsPriority] = 1
	}
	dense[denseStackDepth] = float32(len(v.Stack))

	sparse := buf.sparseBuf()
	// Every id is hashed as it is streamed (hashstream.go): the row is
	// hashID of the string spelt beside it, which is never built.
	pushH := func(h uint64) { sparse = append(sparse, Feature{Row: fnvRow(h), Value: 1}) }
	// "<name>|<tag>"
	nameTag := func(name, tag string) { pushH(fnvAdd(fnvAdd(fnvAdd(fnvOffset, name), "|"), tag)) }
	// "<name>|t<b>|a<b>|s<b>|c<counterString>"
	decorated := func(cv *view.CardView) {
		h := fnvAdd(fnvAdd(fnvAdd(fnvOffset, cv.Name), "|t"), bit(cv.Tapped))
		h = fnvAdd(fnvAdd(h, "|a"), bit(cv.Attacking))
		h = fnvAdd(fnvAdd(h, "|s"), bit(cv.SummonSick))
		pushH(fnvCounterString(fnvAdd(h, "|c"), cv.Counters))
	}
	battleBlock := func(tag string, cards []view.CardView) {
		for i := range cards {
			nameTag(cards[i].Name, tag)
			decorated(&cards[i])
		}
	}

	// Seat blocks: the viewer's own block first, then the other seats in
	// turn order from the viewer (next, the one after, ...), zero-filled
	// beyond the table. A seat that has left keeps its place in the view, so
	// it keeps its block. With two seats this is the old ascending order
	// exactly. The DENSE blocks cap at maxSeatBlocks (a fixed-width vector
	// must); the sparse bag below iterates everyOther uncapped, in ascending
	// seat order — its length is variable by design.
	var me *view.PlayerView
	meIdx := -1
	var othersBuf [8]*view.PlayerView
	everyOther := othersBuf[:0]
	for i := range v.Players {
		pv := &v.Players[i]
		if pv.ID == seat {
			me, meIdx = pv, i
		} else {
			everyOther = append(everyOther, pv)
		}
	}
	var seatsBuf [maxSeatBlocks]*view.PlayerView
	seats := seatsBuf[:]
	seats[0] = me
	if meIdx >= 0 {
		n := len(v.Players)
		for r := 1; r < n && r < maxSeatBlocks; r++ {
			seats[r] = &v.Players[(meIdx+r)%n]
		}
	} else {
		for i := 0; i < len(everyOther) && i < maxSeatBlocks-1; i++ {
			seats[1+i] = everyOther[i]
		}
	}
	for blk, pv := range seats {
		base := denseSeat0 + blk*seatBlockWidth
		if pv == nil {
			continue
		}
		dense[base+0] = float32(pv.Life)
		dense[base+1] = float32(pv.HandSize)
		dense[base+2] = float32(pv.LibrarySize)
		dense[base+3] = float32(pv.GraveyardSize)
		bfCount, crCount, pw, th, untapped, untappedLands := zoneTotals(pv.Battlefield)
		dense[base+4] = float32(bfCount)
		dense[base+5] = float32(crCount)
		dense[base+6] = float32(pw)
		dense[base+7] = float32(th)
		dense[base+8] = float32(untapped)
		dense[base+9] = float32(untappedLands)
	}

	// The Commander block, after the seat blocks: for each relative seat,
	// commander damage taken from each other relative seat's commanders,
	// its command-zone casts, whether one of its commanders is on the
	// battlefield, and whether it has left the game. A non-Commander game
	// leaves the whole block zero except the left-the-game bit.
	commanderGame := false
	for i := range v.Players {
		if len(v.Players[i].Commanders) > 0 {
			commanderGame = true
			break
		}
	}
	n := len(v.Players)
	// relPlayer is the player at relative seat r in turn order from the
	// viewer, the order the seat blocks use. The block buffer holds only the
	// first maxSeatBlocks relative seats; a 5+ seat game still needs the
	// player behind a damage column, so read past it in the full list rather
	// than wrapping into the short buffer.
	relPlayer := func(r int) *view.PlayerView {
		if n == 0 {
			return nil
		}
		if meIdx >= 0 {
			return &v.Players[(meIdx+r)%n]
		}
		if r < len(seats) {
			return seats[r]
		}
		return nil
	}
	// The battlefield is public and zoned by CONTROLLER, so a commander
	// controlled by another player is not in its owner's list. Search every
	// seat's list for the object id: the bit means "this commander is on the
	// battlefield", not "it is on its owner's battlefield".
	onField := func(id state.ObjID) bool {
		for j := range v.Players {
			for i := range v.Players[j].Battlefield {
				if v.Players[j].Battlefield[i].ID == id {
					return true
				}
			}
		}
		return false
	}
	for blk, pv := range seats {
		base := denseCmd0 + blk*commanderBlockWidth
		if pv == nil {
			continue
		}
		if pv.Lost {
			dense[base+denseCmdLost] = 1
		}
		if !commanderGame {
			continue
		}
		// (a)-(c): damage from the other relative seats, in turn order from
		// this seat. A partner pair's commanders each carry their own tally,
		// so the column takes the max over the source seat's commanders.
		if n > 0 {
			for k := 1; k < n && k <= 3; k++ {
				spv := relPlayer(blk + k)
				if spv == nil || len(spv.Commanders) == 0 {
					continue
				}
				var dmg int32
				for _, cmd := range spv.Commanders {
					if d := pv.CmdDamage[cmd.ID]; d > dmg {
						dmg = d
					}
				}
				dense[base+denseCmdDamage0+k-1] = clamp01(float32(dmg) / 21)
			}
		}
		// (d): partner pairs sum their command-zone casts.
		var casts int32
		for _, c := range pv.CommanderCasts {
			casts += c
		}
		dense[base+denseCmdCasts] = clamp01(float32(casts) / 4)
		// (e): any of its commanders is currently on the battlefield.
		for _, cmd := range pv.Commanders {
			if onField(cmd.ID) {
				dense[base+denseCmdOnField] = 1
				break
			}
		}
	}

	// Sparse bag, in the fixed zone order documented above.
	if me := seats[0]; me != nil {
		for i := range me.Hand {
			nameTag(me.Hand[i].Name, zoneTagHand)
		}
		battleBlock(zoneTagBattleMe, me.Battlefield)
	}
	for _, pv := range everyOther {
		battleBlock(zoneTagBattleOpp, pv.Battlefield)
	}
	if me := seats[0]; me != nil {
		for i := range me.Graveyard {
			nameTag(me.Graveyard[i].Name, zoneTagGraveMe)
		}
	}
	for _, pv := range everyOther {
		for i := range pv.Graveyard {
			nameTag(pv.Graveyard[i].Name, zoneTagGraveOpp)
		}
	}
	if me := seats[0]; me != nil {
		for i := range me.Exile {
			nameTag(me.Exile[i].Name, zoneTagExileMe)
		}
	}
	for _, pv := range everyOther {
		for i := range pv.Exile {
			nameTag(pv.Exile[i].Name, zoneTagExileOpp)
		}
	}
	for i := range v.Stack {
		nameTag(v.Stack[i].Name, zoneTagStack)
	}

	if me := seats[0]; me != nil {
		for c, key := range poolKeys {
			if me.Mana.OK {
				dense[densePool0+c] = float32(me.Mana.Pool[c])
			} else {
				dense[densePool0+c] = float32(me.Pool[key])
			}
		}
	}

	stableSortByRow(sparse)
	buf.keepSparse(sparse)
	return State{Dense: dense, Sparse: sparse}
}

// zoneTotals folds one battlefield list into the six per-seat scalars.
func zoneTotals(cvs []view.CardView) (bf, creatures, power, toughness, untapped, untappedLands int) {
	for i := range cvs {
		cv := &cvs[i]
		bf++
		if hasType(cv, cards.TypeCreature) {
			creatures++
			power += int(cv.Power)
			toughness += int(cv.Toughness)
		}
		if !cv.Tapped {
			untapped++
			if hasType(cv, cards.TypeLand) {
				untappedLands++
			}
		}
	}
	return
}

func bit(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func formatInt(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		digits[i] = '-'
	}
	return string(digits[i:])
}

func indexOf(names []string, s string) int {
	for i, n := range names {
		if n == s {
			return i
		}
	}
	return -1
}
