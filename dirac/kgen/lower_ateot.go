package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// AtEOT$ on Animate and on a ChangeZoneAll sweep onto the
// battlefield. gorge scheduleAtEOT (engine/effects/ateot.go) registers one
// next-end-step promise per affected object: effAnimate's Defined$ subjects,
// effChangeZoneAll's moved set. The kernel's OpAtEOT takes one object, so an
// Animate accepts a one-object subject only; a sweep rides MvEntryRiders'
// per-entrant hook (kop.EntryAtEOT, documented in dirac/kop/operands.go).

// animateAtEOT reads an Animate's AtEOT$ for subject obj and defers the
// OpAtEOT emit to the end of the enclosing function (after the override, as
// gorge schedules after registerAnimateEffects). The caller defers the
// returned emit.
func (lc *lowerCtx) animateAtEOT(ps *params, obj lRef) (func(), error) {
	v, ok := ps.get("AtEOT")
	if !ok {
		return nil, nil
	}
	act, err := atEOTAct(v)
	if err != nil {
		return nil, err
	}
	if !lc.oneObjectSubject(obj) {
		return nil, refuse("Animate AtEOT on a set")
	}
	return func() { lc.emit(lInstr{Op: "AtEOT", Obj: obj, C: act}) }, nil
}

// oneObjectSubject reports whether ref names at most one object: Self, one
// object-target slot with no TargetMax$ (a single target), or the remembered
// set when exactly one earlier instruction of this ability remembers and it
// moved (or destroyed) one targeted object.
func (lc *lowerCtx) oneObjectSubject(obj lRef) bool {
	switch obj.Kind {
	case rSelf:
		return true
	case rTarget:
		return lc.singleObjectSlot(obj.N)
	case rRemembered:
		n := 0
		var one bool
		for _, in := range lc.ab.Prog {
			if in.Op == "ClearRemembered" {
				return false
			}
			if in.Op == "Destroy" && in.Note == noteRememberDestroyed {
				n++ // a single targeted Destroy that remembers the object it destroyed
				one = in.Obj.Kind == rTarget && lc.singleObjectSlot(in.Obj.N)
			}
			for _, f := range strings.Split(in.CS, "|") {
				if f == "Remember" {
					n++
					one = in.Obj.Kind == rTarget && lc.singleObjectSlot(in.Obj.N)
				}
			}
		}
		return n == 1 && one
	}
	return false
}

// singleObjectSlot: target slot n is an object slot choosing exactly one.
func (lc *lowerCtx) singleObjectSlot(n int) bool {
	t := lc.ab.Targets[n]
	if t.F == nil || t.F.Players != 0 || len(t.F.Clauses) == 0 || t.Spell {
		return false
	}
	one := func(v lValue) bool { return v.Kind == vNone || v.isConst() && v.N == 1 }
	return one(t.Max) && (one(t.Min) || t.Min.isConst() && t.Min.N == 0)
}

// changeZoneAllAtEOT reads a ChangeZoneAll's AtEOT$ onto in as "|atEOT=<act>":
// one promise per entrant (gorge effChangeZoneAll's moved set), registered by
// the kernel's per-entrant hook (kop.MvEntryRiders + kop.EntryAtEOT).
func (lc *lowerCtx) changeZoneAllAtEOT(ps *params, in *lInstr, origin, dest int, dual bool) error {
	v, ok := ps.get("AtEOT")
	if !ok {
		return nil
	}
	act, err := atEOTAct(v)
	if err != nil {
		return err
	}
	if dest != zBattlefield {
		return refuse("ChangeZoneAll AtEOT onto %s", zoneLabel[dest])
	}
	if dual {
		return refuse("ChangeZoneAll AtEOT with Origin$ Graveyard,Exile")
	}
	if origin != zGraveyard && origin != zExile {
		return refuse("ChangeZoneAll AtEOT from %s: only a Graveyard or Exile sweep is lockstep-checked", zoneLabel[origin])
	}
	in.CS += "|atEOT=" + strconv.Itoa(act)
	return nil
}

// entryAtEOTBits is A's AtEOT image (kop.EntryAtEOT, plus EntryAtEOTExile for
// act 1); built through variables, the constants' int16 image overflows.
func entryAtEOTBits(act int) int16 {
	hi := uint16(kop.EntryAtEOT)
	if act == 1 {
		hi |= kop.EntryAtEOTExile
	}
	return int16(hi)
}

// atEOTAct maps an AtEOT$ value to OpAtEOT's C: Sacrifice 0, Exile 1
// (gorge atEOTRiders' next-end-step bodies); other values refuse. gorge
// ateot.go:26-31 reads YourExile/YourSacrifice as the plain values (the
// carriers resolve on their controller's own turn).
func atEOTAct(v string) (int, error) {
	switch v {
	case "Sacrifice", "YourSacrifice":
		return 0, nil
	case "Exile", "YourExile":
		return 1, nil
	}
	return 0, refuse("AtEOT %q", v)
}

// changeZoneAtEOT reads a ChangeZone's AtEOT$ and takes it off the params, so
// the move lowers as it would without it; emitChangeZoneAtEOT then schedules
// the promise for the moved object. gorge effChangeZone schedules one
// next-end-step promise per object the move actually moved
// (zone_change.go scheduleAtEOT on the Defined$ walk).
func (lc *lowerCtx) changeZoneAtEOT(ps *params) (act int, ok bool, err error) {
	v, ok := ps.get("AtEOT")
	if !ok {
		return 0, false, nil
	}
	if act, err = atEOTAct(v); err != nil {
		return 0, false, err
	}
	if ps.str("Defined") != "Remembered" || ps.str("Destination") != "Battlefield" {
		return 0, false, refuse("ChangeZone AtEOT beside Defined$ %q Destination$ %q", ps.str("Defined"), ps.str("Destination"))
	}
	ps.consume("AtEOT")
	return act, true, nil
}

// emitChangeZoneAtEOT schedules the AtEOT promise on the one object a
// Defined$ Remembered return moved. The kernel's OpAtEOT takes one object, so
// the remembered set must be the one targeted object the same ability
// remembered (oneObjectSubject); the move must be a single Return, which the
// ability's condition then skips along with the promise.
func (lc *lowerCtx) emitChangeZoneAtEOT(start, act int) error {
	moves := 0
	for _, in := range lc.ab.Prog[start:] {
		if in.Op != "Return" || in.Obj.Kind != rRemembered {
			return refuse("ChangeZone AtEOT beside a %s", in.Op)
		}
		moves++
	}
	if moves != 1 {
		return refuse("ChangeZone AtEOT over %d moves", moves)
	}
	if !lc.oneObjectSubject(lRef{Kind: rRemembered}) {
		return refuse("ChangeZone AtEOT on a set of remembered objects")
	}
	lc.emit(lInstr{Op: "AtEOT", Obj: lRef{Kind: rRemembered}, C: act})
	return nil
}
