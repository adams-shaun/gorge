package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// statictrig: a SpellCast trigger whose line carries
// Static$ True resolves OFF the stack, by the trigger drain, before the
// CR 603.3b ordering ask — gorge's resolveStaticTriggers
// (engine/rules/static_trigger.go) removes each queued trigger whose line
// carries Static$ True and whose body is provably ask-free, in queue order,
// and runs it inline: no priority pass, no order choice. The carriers'
// bodies are the "this spell enters with [a finality / an extra +1/+1]
// counter" rider Effects (ExtEffEtbCtr / ExtFinalityCtr), which only stamp
// the cast spell's stack entry, so the resolution poses no decision by
// construction: kgen refuses Static$ True on every other body (targets,
// Charm, optional/unless/cost riders).
//
// placeTriggers calls placeStaticTriggers before the ordering ask. The
// helper resolves ONE static pending trigger per call and returns: the
// engine loop re-enters placeTriggers, so the queue is scanned again (a
// static's own queued triggers first, as gorge's drain re-runs until
// nothing remains) and only then the rest takes the ordinary path. The
// resolution ends with afterResolve's ctlPriority, whose SBA pass runs before
// the queue is drained further, as gorge's drainTriggers repeats its check.

// A spell's stackEntry.sid is its FINALITY rider count:
// the stack-identity byte is given only to ability entries (stackSid: kind
// != seSpell) and every reader skips spell entries, so a spell's byte is
// provably unused. A count, not a flag: several grants can cover one cast
// (Leonardo, Sewer Samurai beside active Osteomancer Adept effects) and
// gorge runs each body's PutCounter through the counter replacements
// separately. The count is split by the grant's controller, who puts the
// counter (CR 122.6a): the low nibble counts player 0's grants and the high
// nibble player 1's, so an opponent's Overflow Submatrix grant is not
// doubled by the caster's Dr. Joseph M'Benga. A spell copy's count is
// cleared (copy_gen.go).
const sidFinalMax = 15

// StallStaticAsk: a static trigger's off-stack build or resolution posed a
// decision, which kgen's ask-free proof forbids (a lowering bug, loud).
const StallStaticAsk = 125

// StallFinality: a 16th FINALITY grant by one player on one spell entry.
const StallFinality = 126

func init() {
	stallNames[StallStaticAsk] = "staticask"
	stallNames[StallFinality] = "finality"
}

// DebugMayPlaySrcAny makes every cast read as a may-play cast
// (TestMayplaystaticMutation: the lockstep must diverge).
var DebugMayPlaySrcAny bool

// MayPlayStats counts the lockstep test's coverage (dirac/oracle/
// mayplaystatic_test.go).
var MayPlayStats struct {
	StaticResolved atomic.Int64 // static SpellCast triggers resolved off the stack
	FinalityEnter  atomic.Int64 // permanents entering with the FINALITY rider
	OrdinaryCasts  atomic.Int64 // casts a may-play-source gate refused (an ordinary cast of a carrier)
}

// prepStaticTrig sets the cold-path prep gates (placeTriggers, enterGen).
func prepStaticTrig(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		cd := &t.Cards[i]
		for j := range cd.Abilities {
			if a := &cd.Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Flags3&kop.Tf3Static != 0 {
				g.anyStaticTrig = true
			}
			for _, in := range cd.Abilities[j].Prog {
				if in.Op == kop.OpExt && in.A.K == kop.VConst && kop.ExtOp(in.A.N) == kop.ExtFinalityCtr {
					// the FINALITY rider also arms dieZone's finality
					// exile (ctr_finality.go)
					g.anyFinality = true
				}
			}
		}
	}
}

// mayPlaySpell reports whether stack entry e is a cast made through a play
// permission (gorge stamps FlagMayPlay for the "mayplay" cast mode; the
// kernel skips sfMayPlay only for the hand's own free alternative cost,
// which the oracle canonicalizes as gorge's AltCostIndex-1 cast whose mode
// is "", so gorge stamps no flag there either — aligned by construction).
func (s *State) mayPlaySpell(e *stackEntry) bool {
	return DebugMayPlaySrcAny || e.flags&sfMayPlay != 0
}

// staticPend reports whether pending trigger i is a static SpellCast
// trigger: one resolved off the stack, before the ordering ask.
func (s *State) staticPend(k *prep, pt *pendingTrig) bool {
	if pt.ab >= abCascade {
		return false // a sentinel (cascade, ward, delayed): never static
	}
	a := &k.t.Cards[pt.card].Abilities[pt.ab]
	return a.Kind == kop.AbTriggered && a.Trig.Flags3&kop.Tf3Static != 0
}

// placeStaticTriggers resolves the first pending static trigger in queue
// order, off the stack, and reports whether it resolved one. The build and
// the resolution must not pose a decision — kgen proved the body ask-free —
// so a posed ask is a loud stall, not an off-stack election.
func (s *State) placeStaticTriggers(k *prep, d *Decision) bool {
	for i := 0; i < int(s.npend); i++ {
		pt := s.pend[i]
		if !s.staticPend(k, &pt) {
			continue
		}
		// the resolution keeps the priority holder and the pass count:
		// gorge runs the body inline in its trigger drain, so the caster of
		// a may-play spell on another player's turn keeps priority (CR
		// 117.3c), where afterResolve would hand it to the active player
		prio, passes := s.prio, s.passes
		s.takePending(k, i)
		for s.ctl == ctlBuild {
			if s.buildStep(k, d) {
				s.capFull(StallStaticAsk)
				return true
			}
		}
		s.resolveTop(k)
		if s.res.active {
			s.capFull(StallStaticAsk)
			return true
		}
		s.prio, s.passes = prio, passes
		MayPlayStats.StaticResolved.Add(1)
		return true
	}
	return false
}

// finalityCtr is ExtFinalityCtr: the resolving SpellCast trigger's spell
// (still on the stack) will enter with one more FINALITY counter. The
// rider lives on the spell's stack entry (its sid count), so a spell gone
// from the stack (countered) gets nothing, as gorge's effect dies with the
// spell's move.
func (s *State) finalityCtr(k *prep, in *kop.Instr) {
	o := s.res.e.trigObj
	if o == NoObj {
		return
	}
	for i := int(s.sp) - 1; i >= 0; i-- {
		e := &s.stack[i]
		if e.kind != seSpell || e.obj != o {
			continue
		}
		if MaxSeats > 2 && s.res.e.ctrl > 1 {
			s.capFull(StallFinality) // sid holds two seats' grant counts, 4 bits each
			return
		}
		sh := 4 * uint(s.res.e.ctrl&1) // the grant's controller puts the counter
		if e.sid>>sh&sidFinalMax >= sidFinalMax {
			s.capFull(StallFinality)
			return
		}
		e.sid += 1 << sh
		return
	}
}

// finalityEnter puts the resolving spell's pending FINALITY counters on the
// permanent at sl as it enters (after the printed enters-with counters,
// before the entry's triggers), one PutCounter per grant through the
// counter replacements, as gorge composes the riders' bodies separately
// (replacement_updated.go).
func (s *State) finalityEnter(k *prep, sl Slot, p *perm) {
	sid := s.res.e.sid
	if sid == 0 {
		return
	}
	for adder := uint8(0); adder < 2; adder++ {
		for n := sid >> (4 * adder) & sidFinalMax; n > 0; n-- {
			if s.placeCounters(k, sl, int(kop.CtrFinality), 1, adder, ctrPutBy) == 0 {
				return
			}
		}
	}
	MayPlayStats.FinalityEnter.Add(1)
}
