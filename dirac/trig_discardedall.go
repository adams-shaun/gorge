package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Mode$ DiscardedAll, "whenever you discard one or more
// cards" (CR 603.2c; gorge trigger_match.go discard batch). A batch is one
// resolving OpDiscard or OpDiscardUnless (gorge effDiscard's
// BeginDiscardBatch bracket, cardflow.go:536-550, covers the whole effect
// including its UnlessType$ arm; open across the op's player-choice
// suspensions, each-player form included):
// the first matching card queues one instance, every later one adds 1 to
// that instance's amount (TriggerCount$Amount). A discard outside the op (a
// cost, cleanup, cycling) is a batch of one. No slot subscription and no
// State field: the batch is the resolving instruction, the instance the
// pending entry of the same source and ability.

// DebugDiscAllPerCard makes every discarded card queue its own instance
// with amount 1 (no batching). The lockstep mutation test flips it and
// requires a divergence.
var DebugDiscAllPerCard bool

// DebugDiscAllJoins counts cards that joined an open batch's instance, and
// DebugDiscAllOpp the matches of a listener by a card its opponent
// discarded (test evidence only; never read by the kernel).
var DebugDiscAllJoins, DebugDiscAllOpp, DebugDiscAllEachJoins, DebugDiscAllUnlessJoins int

// prepDiscAll sets anyDiscAll when some printed trigger is a DiscardedAll.
// A DiscardedAll granted by AddTrigger$ is not scanned (no corpus carrier).
func prepDiscAll(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			g.anyDiscAll = g.anyDiscAll || a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigDiscardedAll
		}
	}
}

// discBatchOpen returns the resolving Discard / DiscardUnless instruction,
// or nil when no batch is open.
func (s *State) discBatchOpen(k *prep) *kop.Instr {
	if !s.res.active {
		return nil
	}
	prog := s.prog(k)
	if int(s.res.pc) < len(prog) && (prog[s.res.pc].Op == kop.OpDiscard || prog[s.res.pc].Op == kop.OpDiscardUnless) {
		return &prog[s.res.pc]
	}
	return nil
}

// discAllNote is called after card o was discarded: each battlefield
// DiscardedAll listener whose Whose and filter match either joins its
// instance already queued for this batch or queues a new one.
//
//go:noinline
func (s *State) discAllNote(k *prep, o Obj) {
	who := s.objs[o].owner()
	var op *kop.Instr
	if !DebugDiscAllPerCard {
		op = s.discBatchOpen(k)
	}
	mo := o
	if s.objs[o].zone != kop.ZoneGraveyard {
		// Another replacement put the discarded card somewhere other than
		// the graveyard (Progenitus): ValidCard has no zone to match.
		// A madness discard never reaches here (discardObj gates on it).
		mo = NoObj
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	for _, ls := range ord[:n] {
		s.slotTriggers(k, ls, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigDiscardedAll }, func(l trigListener, a *kop.Ability) {
			t := &a.Trig
			if !whoseOK(t.Whose, l.ctrl, who) || !s.matchCardGen(k, t.Filter, s.objs[o].card, who, l.ctrl, mo) {
				return
			}
			if who != l.ctrl {
				DebugDiscAllOpp++
			}
			if op != nil {
				for i := 0; i < int(s.npend); i++ {
					p := &s.pend[i]
					if p.obj == l.obj && p.ab == l.ab && p.card == l.card && p.srcVer == s.objs[l.obj].ver {
						p.trigAmt++
						DebugDiscAllJoins++
						if op.Op == kop.OpDiscardUnless {
							DebugDiscAllUnlessJoins++
						} else if op.Obj == kop.RefEachPlayer {
							DebugDiscAllEachJoins++
						}
						return
					}
				}
			}
			s.playerTrigGen(k, kop.TrigDiscardedAll, l, a, who, 1)
		})
	}
}
