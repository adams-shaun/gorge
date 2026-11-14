package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A DamageAll sweep's DamageDealtOnce batch — gorge effDamageAll brackets the
// whole sweep in BeginDamageBatch/EndDamageBatch ("one DamageAll call is ONE
// damage batch"), so the sweep's per-recipient damage events are ONE
// simultaneous batch for the DamageDealtOnce latch (CR 510.4): each once line
// queues at its first matching event, with that event's amount, and
// closeDamageBatch patches the queued trigger's amount to the batch's
// matching-event total (trigger_match.go latchDamageBatch, closeDamageBatch).
// Without this a sweep queues the once line once per matching recipient event.
//
// The bracket (genState.dmgBOn) and the line count (dmgBn) live outside the
// shared scratch, because the damage hooks that read them also run while
// another arm owns it (combat damage, a cast's mana-ability damage); the
// records (armRes.dmgBRecs) are read only while the bracket holds. The sweep's
// later events can wait on a CR 616.1 replacement-order ask (damageSeqNext's
// parked event): the batch spans the pause, like dmgSeqOrd. A resolution
// enters with the batch closed (scrResEnter), and the close of the sweep ends
// the window, so a read of the bracket outside it is never true, and a batch
// without records is still open, so an outside-sweep once line cannot arm
// into it. Nested sweeps
// (impossible today: nothing resolves inside a sweep) would share the one
// batch, as gorge's depth-1 brackets do.

// maxDmgBatch is the DamageDealtOnce lines one sweep's batch latches.
const maxDmgBatch = 8

// dmgBatchEnt is one latched line: the batch key (the sweep's source object
// and the line) and the pending entry its amount is patched on.
type dmgBatchEnt struct {
	src   Obj    // the sweep's source object (an ability whose source is gone keeps its LKI object)
	obj   Obj    // the triggering object
	card  CardID // the line's card
	ab    uint8  // the line's ability index
	pend  uint8  // the pending entry queued at the first matching event
	total int16  // the batch's matching-event total (patched in at close)
}

func init() { stallNames[StallDmgBatch] = "dmgbatch" }

// dmgBatchOpen starts a DamageAll sweep's batch: the latched lines are
// forgotten (the previous sweep's are closed by then). A reentrant bracket
// belongs to the same batch (gorge openDamageBatch's depth): an open batch is
// neither reset nor re-counted.
func (s *State) dmgBatchOpen() {
	if s.g.dmgBOn {
		return
	}
	s.g.dmgBOn, s.g.dmgBn = true, 0
}

// dmgBatchSeen reports whether the sweep batch has latched line l of source
// src already; when it has, this matching event only accumulates its amount
// (gorge latchDamageBatch: ent.amount += damageAmount, return false).
//
//go:noinline
func (s *State) dmgBatchSeen(src Obj, l trigListener, amt int16) bool {
	a := s.resArm()
	for i := 0; i < int(s.g.dmgBn) && i < maxDmgBatch; i++ {
		if e := &a.dmgBRecs[i]; e.src == src && e.obj == l.obj && e.card == l.card && e.ab == l.ab {
			e.total = s.addSat16(e.total, int(amt))
			return true
		}
	}
	return false
}

// dmgBatchArm records a just-queued line as the sweep batch's entry for it:
// the pending entry queued at n0 with the first matching event's amount,
// which dmgBatchClose patches to the total. Cold: only a full pool reaches
// the stall.
//
//go:noinline
func (s *State) dmgBatchArm(src Obj, l trigListener, amt int16, n0 uint8) {
	a := s.resArm()
	if s.g.dmgBn >= maxDmgBatch {
		s.capFull(StallDmgBatch)
		return
	}
	a.dmgBRecs[s.g.dmgBn] = dmgBatchEnt{src: src, obj: l.obj, card: l.card, ab: l.ab, pend: n0, total: amt}
	s.g.dmgBn++
}

// dmgBatchClose ends the sweep's batch: every record's queued trigger gets its
// amount patched to the batch's matching-event total (gorge closeDamageBatch;
// the pending queue is append-only while a batch is open, so the index holds).
func (s *State) dmgBatchClose() {
	if !s.g.dmgBOn {
		return
	}
	a := s.resArm()
	s.g.dmgBOn = false
	for i := 0; i < int(s.g.dmgBn) && i < maxDmgBatch; i++ {
		e := &a.dmgBRecs[i]
		s.pend[e.pend].trigAmt = e.total
	}
	s.g.dmgBn = 0
}

// dmgBatchOnce reports whether line a takes the sweep batch's DamageDealtOnce
// latch: a DamageDealtOnce line without the per-referent read (gorge Mode$
// DamageDoneOnce; a DamageDealtOnce line never carries it, but the flag is
// the kernel's per-referent key).
func dmgBatchOnce(a *kop.Ability) bool {
	return a.Trig.Event == kop.TrigDamageDealtOnce && a.Trig.Flags2&kop.Tf2PerReferent == 0
}
