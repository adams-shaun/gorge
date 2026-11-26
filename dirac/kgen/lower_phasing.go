package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// api Phases (CR 702.26, Spectral Adversary). gorge effPhases phases
// out each affected permanent (the targets of ValidTgts$, or the source by
// default), and every permanent attached to it indirectly with it (702.26g).
// The kernel's Ext Phases does the same for the permanent Obj names; a target
// slot of several objects runs it once per object, in pick order.
//
// Read exactly: ValidTgts$ with TargetMin$/TargetMax$ and no Defined$, or no
// ValidTgts$ and Defined$ Self (or absent). Defined$ "Self & Targeted" beside
// ValidTgts$ (U.S.S. Defiant, Mirror Migrant: "it and up to one target
// creature you control phase out") is gorge's Defined list, the source then
// the targets: the source phases out first, then each target; one that has
// already phased out (the source chosen as its own target) is skipped.
//
// AllValid$ <filter> with none of ValidTgts$, Defined$ or the refused keys
// (The City on the Edge of Forever) is the sweep: every battlefield permanent
// the filter admits phases out, in gorge's object order (effPhases' arena
// walk), one at a time, an attached permanent that the sweep has not reached
// yet going out indirectly with its host. The toggle (PhaseInOrOut$), the
// phase-in override Phaseout$, RememberAffected$, WontPhaseInNormal$ and
// Tapped$ are refused, as is K:Phasing (the keyword toggle) elsewhere.
func (lc *lowerCtx) phases(ps *params) error {
	for _, k := range []string{"PhaseInOrOut", "Phaseout", "RememberAffected", "WontPhaseInNormal", "Tapped"} {
		if _, ok := ps.get(k); ok {
			return refuse("Phases %s$", k)
		}
	}
	if spec, ok := ps.get("AllValid"); ok {
		for _, k := range []string{"ValidTgts", "Defined"} {
			if _, bad := ps.get(k); bad {
				return refuse("Phases AllValid$ beside %s$", k)
			}
		}
		f, err := lc.filter(strings.TrimSpace(spec))
		if err != nil {
			return err
		}
		if f == nil || f.Players != 0 {
			return refuse("Phases AllValid$ %q with a player scope", spec)
		}
		lc.emit(lInstr{Op: "Phases", Obj: lRef{Kind: rNone}, F: f})
		return nil
	}
	_, tgts := ps.get("ValidTgts")
	if d, ok := ps.get("Defined"); ok && !tgts && d != "Self" {
		return lc.phasesDefined(ps, strings.TrimSpace(d))
	}
	if d, ok := ps.get("Defined"); ok {
		switch {
		case tgts && d == "Self & Targeted":
			lc.emit(lInstr{Op: "Phases", Obj: lRef{Kind: rSelf}})
		case tgts && d != "Targeted":
			return refuse("Phases Defined$ %q beside ValidTgts$", d)
		}
	}
	tps := ps
	if ps.str("Defined") == "Self & Targeted" {
		// the target slot is the subject of the second Phases
		m := make(map[string]string, len(ps.m))
		for k, v := range ps.m {
			if k != "Defined" {
				m[k] = v
			}
		}
		tps = &params{m: m, used: ps.used}
	}
	obj, err := lc.objOrTarget(tps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Phases", Obj: obj})
	return nil
}

// phasesDefined is Phases with a Defined$ subject and no ValidTgts$ of its
// own (gorge phasesAffectedObjects: Defined's objects, in its order, each
// phased out unless an earlier one's attachment sweep already took it).
//
// Valid <filter> is gorge's battlefieldValidTargets: every battlefield
// permanent the filter admits, read once, seat 0 first and each battlefield
// in order. Only a filter naming the controller's own permanents lowers
// (Taniwha, Teferi's Protection): the kernel's card loop lists the
// controller's battlefield first, so with one seat in play the two orders
// are the same; the order is observable through an attachment phasing out
// with its host (CR 702.26g) rather than on its own.
//
// Any other spelling lowers through the Defined$ grammar when it names one
// object the kernel's ExtPhases reads: a target slot of the chain
// (Targeted / ParentTarget: Slip Out the Back), the source's host (Equipped /
// Enchanted / AttachedTo: Robe of Stars, Vanishing).
func (lc *lowerCtx) phasesDefined(ps *params, d string) error {
	if spec, ok := strings.CutPrefix(d, "Valid "); ok {
		if strings.Contains(d, " & ") {
			return refuse("Phases Defined$ %q", d)
		}
		f, err := lc.filter(strings.TrimSpace(spec))
		if err != nil {
			return err
		}
		if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
			return refuse("Phases Defined$ %q: not a card filter", d)
		}
		for _, c := range f.Clauses {
			if c.Ctrl != whoYou {
				return refuse("Phases Defined$ %q: a filter over more than the controller's permanents", d)
			}
		}
		ps.consume("Defined")
		lc.emit(lInstr{Op: "RepeatEachCard", Obj: lRef{Kind: rNone}, F: f, C: kop.RepeatPlain, Skip: 1})
		lc.emit(lInstr{Op: "Phases", Obj: lRef{Kind: rEach}})
		return nil
	}
	switch d {
	case "Targeted", "ParentTarget", "ParentTargeted", "Equipped", "Enchanted", "AttachedTo":
	default:
		return refuse("Phases Defined$ %q", d)
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	switch obj.Kind {
	case rEquipped:
	case rTarget:
		if t := lc.ab.Targets[obj.N]; t.F == nil || t.F.Players != 0 {
			return refuse("Phases Defined$ %q over a player target slot", d)
		}
	default:
		return refuse("Phases Defined$ %q", d)
	}
	lc.emit(lInstr{Op: "Phases", Obj: obj})
	return nil
}
