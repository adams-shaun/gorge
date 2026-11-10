package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Start your engines! and speed (CR 702.179), copied
// from gorge (engine/rules/speed.go, engine/effects/speed.go and
// change_speed.go). Cold and out of line: a table without a Start your
// engines! card or an OpChangeSpeed never takes these paths (prep.speed,
// prep.engines).
//
// State: genState.speed is one byte -- bits 0-2 seat 0's speed, bits 3-5
// seat 1's (0..4), bit 6 the active player's speed trigger already queued
// this turn (gorge speedTriggeredThisTurn: a queued trigger counts, a
// countered one too; cleared as a turn begins). The byte sits in the
// padding after genState.poison, so State does not grow.
//
//   - 702.179a: a post-placement scan (speedStart) from the one battlefield
//     entry (enterNF: spells, tokens, copies, returns) and from control
//     changes (ctrlMove): a seat with no speed that controls a permanent
//     carrying Start your engines! gets speed 1.
//   - 702.179d: a player losing life (loseLife: damage, LoseLife, paid life)
//     during the active player's turn queues the active player's source-less
//     inherent trigger (a delayed-trigger entry with obj NoObj and
//     trigAmt speedTrigAmt) when they have speed 1..3 and none was queued
//     this turn; it resolves to +1 speed with its intervening if re-checked.
//   - 702.179e: a Gate{Ctr: kop.CtrMaxSpeed} ability exists only while its
//     controller's speed is 4 (gateOK); a gated trigger is not re-checked on
//     resolution (gorge grants the trigger, it does not gate its body).

// DebugIgnoreEntryStart, DebugIgnoreControlStart and DebugIgnoreSpeedLatch
// make the kernel skip a speed hook: the battlefield-entry scan, the
// control-change scan and the once-per-turn latch. The lockstep mutation test
// flips the entry and latch hooks and requires a divergence; the control hook
// is unit-tested in speed_test.go because a later entry (Involuntary
// Employment's Treasure) re-runs the scan and masks its removal.
var DebugIgnoreEntryStart, DebugIgnoreControlStart, DebugIgnoreSpeedLatch bool

// speedCount is a plain counter: the lockstep test reads it single-threaded.
type speedCount struct{ v uint64 }

func (c *speedCount) Load() uint64 { return c.v }
func (c *speedCount) Add(n uint64) { c.v += n }

// SpeedCounts counts the speed events the kernel played since the process
// began (write-only observability: the lockstep test asserts that every
// sub-rule of CR 702.179 fired, since a carrier that never fires proves
// nothing). Indexed by the SpeedCnt* constants.
//
// SpeedCounting gates every write. Production leaves it false, so a
// transition does no shared write at all: parallel searches cannot contend
// on the counters' cache line. The lockstep test sets
// it before reading the counts.
var SpeedCounting bool
var SpeedCounts [NumSpeedCnt]speedCount

const (
	SpeedCntStartEntry   = iota // 702.179a: a battlefield entry started a seat's speed
	SpeedCntStartControl        // 702.179a: a control change started a seat's speed
	SpeedCntQueue               // 702.179d: the inherent trigger was queued
	SpeedCntLatch               // 702.179d: a second loss in the turn was refused by the latch
	SpeedCntOwnTurn             // 702.179d: a loss on the player's own turn queued nothing
	SpeedCntTo2                 // the trigger resolved to speed 2
	SpeedCntTo3                 // ... to 3
	SpeedCntTo4                 // ... to 4
	SpeedCntDecrease            // ChangeSpeed decreased a speed
	SpeedCntDecreaseHeld        // ChangeSpeed left a speed (at 1, or not the highest)
	SpeedCntGatedTrigger        // 702.179e: a max-speed trigger was enumerated at speed 4
	SpeedCntGatedOffer          // 702.179e: a max-speed activated ability was offered at speed 4
	NumSpeedCnt
)

const (
	speedMaxV    = 4
	speedLatch   = 1 << 6 // the active player's speed trigger was queued this turn
	speedTrigAmt = -1     // the abDelayed entry's trigAmt marking the speed trigger
)

// prepSpeed reports whether some card carries Start your engines! (engines)
// and whether the table touches speed at all (speed: engines or an
// OpChangeSpeed program).
func prepSpeed(t *kop.Table) (speed, engines bool) {
	for i := range t.Cards {
		c := &t.Cards[i]
		engines = engines || c.Engines
		for j := range c.Abilities {
			for _, in := range c.Abilities[j].Prog {
				speed = speed || in.Op == kop.OpChangeSpeed
			}
		}
	}
	return speed || engines, engines
}

func (s *State) speedOf(p uint8) int { return int(s.g.speed >> (3 * (p & 1)) & 7) }

// Speed is player p's speed, 0..4 (oracle / tests).
func (s *State) Speed(p uint8) int { return s.speedOf(p) }

// speedAdd folds one SpeedChange (gorge foldSpeedChange): the delta added
// and the result clamped to 0..4.
func (s *State) speedAdd(p uint8, d int) {
	v := min(max(s.speedOf(p)+d, 0), speedMaxV)
	sh := 3 * (p & 1)
	s.g.speed = s.g.speed&^(7<<sh) | uint8(v)<<sh
}

// speedStart is gorge checkSpeedStart: each speed-less seat controlling a
// permanent with Start your engines! (not one that lost all its abilities)
// starts at 1.
//
//go:noinline
func (s *State) speedStart(k *prep, cnt int) {
	if s.over || (cnt == SpeedCntStartEntry && DebugIgnoreEntryStart) {
		return
	}
	var need uint8
	for p := uint8(0); p < MaxSeats; p++ {
		if s.speedOf(p) == 0 {
			need |= 1 << p
		}
	}
	for m := s.occ; m.any() && need != 0; m = m.rest() {
		sl := m.first()
		p := &s.perms[sl]
		if need&(1<<p.ctrl) != 0 && k.t.Cards[s.cardAt(sl)].Engines && !s.genLost(k, sl) {
			s.speedAdd(p.ctrl, 1)
			if SpeedCounting {
				SpeedCounts[cnt].Add(1)
			}
			need &^= 1 << p.ctrl
		}
	}
}

// speedControl is speedStart after a control change.
//
//go:noinline
func (s *State) speedControl(k *prep) {
	if !DebugIgnoreControlStart {
		s.speedStart(k, SpeedCntStartControl)
	}
}

// speedLoss is gorge checkSpeedGain after player lp lost life: the active
// player, another seat with speed 1..3, queues the inherent trigger once a
// turn.
//
//go:noinline
func (s *State) speedLoss(lp uint8) {
	a := s.active
	if s.over {
		return
	}
	if v := s.speedOf(a); v == 0 || v >= speedMaxV {
		return
	}
	if a == lp {
		if SpeedCounting {
			SpeedCounts[SpeedCntOwnTurn].Add(1)
		}
		return
	}
	if s.g.speed&speedLatch != 0 && !DebugIgnoreSpeedLatch {
		if SpeedCounting {
			SpeedCounts[SpeedCntLatch].Add(1)
		}
		return
	}
	if SpeedCounting {
		SpeedCounts[SpeedCntQueue].Add(1)
	}
	s.g.speed |= speedLatch
	s.enqueue(NoObj, 0, 0, abDelayed, a, NoObj, a, speedTrigAmt)
}

// speedTurn clears the once-per-turn latch as a turn begins.
func (s *State) speedTurn() { s.g.speed &^= speedLatch }

// resolveSpeed is the resolving speed trigger (gorge effSpeedIncrease): the
// intervening if (speed 1..3) is re-checked, then +1.
func (s *State) resolveSpeed(e *stackEntry) {
	if v := s.speedOf(e.ctrl); v >= 1 && v < speedMaxV {
		s.speedAdd(e.ctrl, 1)
		if SpeedCounting {
			SpeedCounts[SpeedCntTo2+v-1].Add(1)
		}
	}
}

// changeSpeedOp is OpChangeSpeed (gorge effChangeSpeed): each player in
// Obj changes speed by one. C=1 increases (stopping at 4), C=2 decreases
// (stopping at 1). B.N bit 0: only a speed-less player (increase from
// zero); bit 1: only a player whose speed exceeds every other player's.
//
//go:noinline
func (s *State) changeSpeedOp(k *prep, in *kop.Instr) {
	q := s.players(k, in.Obj)
	for i := uint8(0); i < q.n; i++ {
		p := q.s[i]
		v := s.speedOf(p)
		if in.C == 1 && v >= speedMaxV || in.C == 2 && v <= 1 || in.B.N&1 != 0 && v != 0 {
			if SpeedCounting {
				SpeedCounts[SpeedCntDecreaseHeld].Add(1)
			}
			continue
		}
		if in.B.N&2 != 0 && s.speedMatched(p, v) {
			if SpeedCounting {
				SpeedCounts[SpeedCntDecreaseHeld].Add(1)
			}
			continue
		}
		if in.C == 1 {
			s.speedAdd(p, 1)
		} else {
			s.speedAdd(p, -1)
			if SpeedCounting {
				SpeedCounts[SpeedCntDecrease].Add(1)
			}
		}
	}
}

// maxSpeedGate is gateOK for a max-speed gate: the source's controller has
// speed 4 (CR 702.179e).
//
//go:noinline
func (s *State) maxSpeedGate(sl Slot, a *kop.Ability) bool {
	if s.speedOf(s.perms[sl].ctrl) < speedMaxV {
		return false
	}
	if a.Kind == kop.AbTriggered {
		if SpeedCounting {
			SpeedCounts[SpeedCntGatedTrigger].Add(1)
		}
	} else {
		if SpeedCounting {
			SpeedCounts[SpeedCntGatedOffer].Add(1)
		}
	}
	return true
}

// speedMatched reports whether a player other than p still in the game has
// speed v or more (bit 1 of OpChangeSpeed: p's must exceed every other's).
func (s *State) speedMatched(p uint8, v int) bool {
	for q := uint8(0); q < s.nSeats(); q++ {
		if q != p && !s.out(q) && s.speedOf(q) >= v {
			return true
		}
	}
	return false
}
