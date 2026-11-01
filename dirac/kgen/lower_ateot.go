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
// moved one targeted object.
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
