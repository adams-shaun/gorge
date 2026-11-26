package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Event-ended Effect records (Orcish Farmer, Soul Sculptor). An Effect whose
// one static changes its one targeted permanent and whose one Static$ True
// trigger only exiles the Effect (CR 611.2a, 400.7) lowers to an override
// record on the target (kop.Override.UntilCtrlUntap / UntilCast). gorge's
// registration is a delayed trigger beside a Permanent continuous effect:
//
//   - a Mode$ Phase | Phase$ Untap | ValidPlayer$ Player.controlsCard.
//     IsRemembered trigger fires at every untap step (checkDelayedTriggers on
//     the StepChange) whose active player controls the remembered card, and its
//     Static$ True body resolves in the trigger drain: after the untap action,
//     before the upkeep's triggers go on the stack. dropCtrlUntapOv runs as the
//     untap step ends.
//   - a Mode$ SpellCast | ValidActivatingPlayer$ You trigger fires inline at the
//     cast's PutOnStack walk (checkEventDelayedTriggers, fireStaticDelayed),
//     after the face triggers and before the cascade triggers: dropUntilCastOv
//     runs at the same point as fireDelayedSpell. "You" is the registration's
//     controller, the player who resolved the Effect.
//
// The registration is bound to its source as gorge's delayedRegistrationLive
// reads it: a Duration$ Permanent trigger of a source that was on the
// battlefield when the Effect resolved is removed when that incarnation leaves,
// or when the registration's controller has lost, and the continuous effect
// then stays for good. When any registration fires, its self-exile (the Effect's
// ChangeZone from the command zone, EndEffectSource) ends every Effect record of
// the same source OBJECT, live or not, whatever it targeted. The applied record
// carries all of that in the fields a set P/T would use (the rows have none):
// r.p the source object; r.t the source's version in its low byte, the
// registration controller in bits 8-9 and evBound (bit 10) when the source was
// on the battlefield at resolution.

// DebugNoEventOvDrop is the mutation switch: an event-ended Effect record is
// never dropped, so a lockstep that expects the drop must fail.
var DebugNoEventOvDrop bool

// EventOvDropStats counts the records each drop ended, for the lockstep tests.
var EventOvDropStats struct{ CtrlUntap, Cast atomic.Int64 }

// useCtrlUntapOv and useUntilCastOv are the genPrep.uses bits of a table with
// such an Override row: registered uses, not genPrep fields, so the fixed layout
// (TestNoNewFixedStateFields) is unchanged.
var (
	useCtrlUntapOv = newGenUse(func(t *kop.Table) bool {
		for i := range t.Overrides {
			if t.Overrides[i].UntilCtrlUntap {
				return true
			}
		}
		return false
	})
	useUntilCastOv = newGenUse(func(t *kop.Table) bool {
		for i := range t.Overrides {
			if t.Overrides[i].UntilCast != 0 {
				return true
			}
		}
		return false
	})
)

// evBound is r.t's bit for a source that was on the battlefield when its Effect
// resolved (gorge's SourceBattlefield).
const evBound = 1 << 10

// ovEndStamp writes the resolving Effect's source binding into a new record's
// p and t when its row is event-ended. It runs from applyOverridePT.
func (s *State) ovEndStamp(r *ovRec, ov *kop.Override) {
	if !ov.UntilCtrlUntap && ov.UntilCast == 0 {
		return
	}
	e := &s.res.e
	r.p = int16(e.obj)
	r.t = int16(e.srcVer) | int16(e.ctrl&3)<<8
	if s.hostHere(e) {
		r.t |= evBound
	}
}

// ovEndLive reports whether event-ended record r still has its trigger:
// gorge's delayedRegistrationLive (the registration's controller has not lost
// and, when the source was on the battlefield at resolution, the same
// incarnation is still there).
func (s *State) ovEndLive(r *ovRec) bool {
	if s.out(uint8(r.t >> 8 & 3)) {
		return false
	}
	if r.t&evBound != 0 {
		ob := &s.objs[r.p]
		return ob.zone == kop.ZoneBattlefield && ob.ver == uint8(r.t)
	}
	return true
}

// ctrlUntapEnd is the untap step's exit: it drops the records of permanents the
// active player controls.
func (s *State) ctrlUntapEnd(k *prep) {
	if k.gen != nil && k.gen.uses.has(useCtrlUntapOv) {
		s.dropCtrlUntapOv(k)
	}
}

// ovEndFired returns the source object of the first event-ended record whose
// trigger fires now (ctrl: the untap step when true, else a cast by e), or -1.
func (s *State) ovEndFired(k *prep, untap bool, e *stackEntry) int16 {
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.kind != fxOverride {
			continue
		}
		o := &k.t.Overrides[r.r.idx-1]
		if untap {
			if !o.UntilCtrlUntap || s.perms[r.sl].ctrl != s.active {
				continue
			}
		} else if o.UntilCast == 0 || uint8(r.r.t>>8&3) != e.ctrl || !s.matchCardGen(k, o.UntilCast, e.card, e.ctrl, e.ctrl, e.obj) {
			continue
		}
		if s.ovEndLive(&r.r) {
			return r.r.p
		}
	}
	return -1
}

// ovEndSource ends every event-ended record of source object host.
func (s *State) ovEndSource(k *prep, host int16) {
	s.dropFx(func(i int) bool {
		r := &s.g.fx[i]
		if r.kind != fxOverride || r.r.p != host {
			return false
		}
		o := &k.t.Overrides[r.r.idx-1]
		return o.UntilCtrlUntap || o.UntilCast != 0
	})
	s.refreshTy(k)
}

// dropCtrlUntapOv ends the Orcish Farmer records whose permanent the active
// player controls (the trigger's ValidPlayer$ Player.controlsCard.IsRemembered),
// with every other record of the same Farmer.
func (s *State) dropCtrlUntapOv(k *prep) {
	if DebugNoEventOvDrop || s.g.nfx == 0 {
		return
	}
	for h := s.ovEndFired(k, true, nil); h >= 0; h = s.ovEndFired(k, true, nil) {
		EventOvDropStats.CtrlUntap.Add(1)
		s.ovEndSource(k, h)
	}
}

// dropUntilCastOv ends the Soul Sculptor records whose resolver has just cast
// spell e as a spell matching the row's filter, with every other record of the
// same Sculptor.
func (s *State) dropUntilCastOv(k *prep, e *stackEntry) {
	if DebugNoEventOvDrop || s.g.nfx == 0 {
		return
	}
	for h := s.ovEndFired(k, false, e); h >= 0; h = s.ovEndFired(k, false, e) {
		EventOvDropStats.Cast.Add(1)
		s.ovEndSource(k, h)
	}
}
