package dirac

// CR 500.7's extra-turn rotation across seats. The pending grants are ONE
// queue over every seat, taken most recently created first, each spent at
// the cleanup of the turn that follows its creation; the ordinary rotation
// resumes after the LAST NORMAL turn's holder once the queue drains (gorge's
// ExtraTurnQueue, engine/rules/turn.go's advanceStep consumer). The per-seat
// counter this file replaced spent a grant only at its own seat's next
// cleanup, so a flash extra-turn card (Nexus of Fate) cast on an opponent's
// turn pended until the caster's next turn and, past two seats, the granted
// turn began several seats late.

// maxExTurnQ caps the grants one turn may append: a grant is spent at the
// very next cleanup, so the queue only ever holds the resolutions of the
// turn in progress. Gorge clamps its fold at maxQueuedGrants (1<<10,
// engine/events/apply.go); the kernel stops instead, a declared capacity.
const maxExTurnQ = 32

// maxExRide caps the riders armed at once: one per granted turn still to
// reach its end step.
const maxExRide = 4

// exTurnRiderAmt is the abDelayed entry's trigAmt marking the extra turn's
// end-step lose rider. The trigger is source-less: its action is the granted
// seat's loss, read off the entry's controller (turn.go resolveDelayed). The
// amount sits in the kernel-internal negative space beside speed's -1, the
// venture's -2, the monarch's -3/-4, the rad drain's -5 and the ring's -6/-7/-8: a dly record's
// act is a kop Op ordinal (uint8, never negative), so no delayed directive
// can ever collide with it -- the positive 230/231/232 sentinels
// (opLeaveReturn, opSacUntracked, opDashReturn) must not be reused here.
const exTurnRiderAmt = -9

// DebugExtraTurnFIFO is the mutation test's wrong order: the queue is taken
// oldest first instead of most recently created first (CR 500.7).
var DebugExtraTurnFIFO bool

// DebugExRideAtGrant is the rider mutation test's wrong registration: the
// rider arms at the GRANT, so the granting turn's own end step fires the
// loss and the granted turn never begins. Gorge registers at the -1
// consumption with the granted turn as MinTurn (engine/events/apply_turn.go
// foldExtraTurn), so the granting turn's end step never fires it.
var DebugExRideAtGrant bool

// exPush appends one pending grant (interp.go's OpAddTurn). rider is the
// card's ExtraTurnDelayedTrigger$ rider: the granted turn's end step fires
// its Execute body (Final Fortune's "At the beginning of that turn's end
// step, you lose the game"), with src the resolving spell the rider's
// trigger names as its source (gorge's registration carries c.Source).
func (s *State) exPush(p uint8, rider bool, src Obj) {
	if s.g.exN >= maxExTurnQ {
		s.capFull(StallExTurn)
		return
	}
	s.g.exQ[s.g.exN] = p
	s.g.exQSrc[s.g.exN] = src
	if rider {
		s.g.exRide |= 1 << s.g.exN
		if DebugExRideAtGrant {
			s.exRideArm(p, src) // the mutation: the granting turn's end step fires it
		}
	}
	s.g.exN++
}

// exTake spends pending grants at this cleanup: most recently created first,
// a grant to a seat that has lost is consumed with no turn and the next
// grant is spent in the same cleanup, and the first grant that produces a
// turn begins it -- the rest wait for that turn's own cleanup (gorge's
// advanceStep loop). Once the queue drains the ordinary rotation resumes
// after the last normal turn's holder; the granted turns left exBase alone.
func (s *State) exTake() {
	for s.g.exN > 0 {
		var p uint8
		var src Obj
		var rider, noPrev, lose bool
		if DebugExtraTurnFIFO {
			// the mutation test's wrong order: the oldest entry first
			p = s.g.exQ[0]
			src = s.g.exQSrc[0]
			rider = s.g.exRide&1 != 0 // seat-ok: a grant-flag bit, not a seat
			noPrev, lose = s.exNoPrevTake(0, true)
			copy(s.g.exQ[:s.g.exN-1], s.g.exQ[1:s.g.exN])
			copy(s.g.exQSrc[:s.g.exN-1], s.g.exQSrc[1:s.g.exN])
			s.g.exRide >>= 1
			s.g.exN--
		} else {
			s.g.exN--
			p = s.g.exQ[s.g.exN]
			src = s.g.exQSrc[s.g.exN]
			rider = s.g.exRide&(1<<s.g.exN) != 0
			noPrev, lose = s.exNoPrevTake(s.g.exN, false)
			s.g.exRide &^= 1 << s.g.exN
		}
		if s.pl[p].lost {
			if goadOn(s.k()) {
				s.goadTurn(s.k(), p) // that turn would have begun: the seat's goads end (CR 800.4m, gorge foldRotationExtraTurn)
			}
			continue // gorge's consumer: consumed, no turn, the next grant now
		}
		if rider {
			s.exRideArm(p, src)
		}
		if noPrev {
			s.exNoPrevPendAdd(p, lose, src) // armed at the turn's first upkeep (act_extraturnnoprev.go)
		}
		if !DebugNoExtraTurnBegun {
			s.g.extraTurnBegun |= 1 << SeatIx(p) // an extra turn's TurnChange fires
		}
		s.beginTurn(p)
		return
	}
	s.ordinaryTurn() // act_goad.go
}

// exNextTurn is the seat whose turn the next cleanup begins: the most
// recently created pending grant's seat -- skipping a grant whose seat has
// lost, which the consumer spends with no turn -- else the ordinary rotation
// after the last normal turn's holder.
func (s *State) exNextTurn() uint8 {
	n := int(s.g.exN)
	for i := 0; i < n; i++ {
		ix := n - 1 - i
		if DebugExtraTurnFIFO {
			ix = i
		}
		if p := s.g.exQ[ix]; !s.pl[p].lost {
			return p
		}
	}
	return s.nextAlive(s.g.exBase)
}

// exRideArm registers the granted turn's ExtraTurnDelayedTrigger rider, src
// the spell the trigger names as its source. The registration rides the
// consumption, not the grant: the turn a grant produces is exactly the one
// about to begin (gorge foldExtraTurn registers the delayed Mode$ Phase
// trigger here with the granted turn as MinTurn, so the granting turn's own
// end step never fires it). Riders accumulate: a granted turn that
// end-the-turn skipped (Time Stop) leaves its rider armed into the following
// turn, whose end step fires it, the way gorge's MinTurn gate does.
func (s *State) exRideArm(p uint8, src Obj) {
	if s.g.exRideN >= maxExRide {
		s.capFull(StallExTurn)
		return
	}
	s.g.exRideS[s.g.exRideN] = p
	s.g.exRideO[s.g.exRideN] = src
	s.g.exRideV[s.g.exRideN] = uint8(s.objs[src].ver)
	s.g.exRideC[s.g.exRideN] = s.objs[src].card
	s.g.exRideN++
}

// exRideFire is the end step's rider: each armed record's "At the beginning
// of that turn's end step, you lose the game" (Final Fortune, Last Chance,
// Warrior's Oath). A record is armed when its granted turn begins and
// cleared here, so it fires exactly once; a seat that has already lost is
// skipped, as gorge's effLosesGame is.
func (s *State) exRideFire() {
	for i := uint8(0); i < s.g.exRideN; i++ {
		p, o := s.g.exRideS[i], s.g.exRideO[i]
		if !s.pl[p].lost {
			s.enqueue(o, s.g.exRideV[i], s.g.exRideC[i], abDelayed, p, NoObj, p, exTurnRiderAmt)
		}
	}
	s.g.exRideN = 0
}

func init() { stallNames[StallExTurn] = "extraturn" }
