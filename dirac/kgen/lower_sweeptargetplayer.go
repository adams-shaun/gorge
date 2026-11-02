package kgen

import (
	"strings"
)

// the target-player scope of a sweep.
//
// gorge's effPutCounterAll (engine/effects/counters.go:251) and effUntapAll
// (engine/effects/combatfx.go:134) are All primitives: with no player
// selector they walk every seat's battlefield. A ValidTgts$ Player makes
// them act on the chosen player targets of the resolving LINK only
// (putCounterAllSweep, counters.go:295, through targetedGroup,
// conditions.go:1402; allPlayersFor, combatfx.go:179, through
// DefinedRef/Defined). The kernel has one expression for that scope: the
// F2TgtPlayerCtrl filter bit (kop/table.go) matches a permanent whose
// controller is a player target of the current stack entry. kgen spells it
// TargetedPlayerCtrl (filter.go, whoTgtPlayer).
//
// The bit is entry-wide while gorge reads this link's targets, so
// sweepTgtPlayer refuses the card at lowerTail when the ability declares a
// second player target slot, in ANY mode: a multi-pick charm's other mode's
// player target is a target of the same stack entry, and ctrlRoleF2 would
// sweep it too.

// sweepTgtPlayerNote tags an instruction lowered from a ValidTgts$ Player
// sweep.
const sweepTgtPlayerNote = "sweepTgtPlayer"

// sweepTgtPlayer scopes spec to the chosen player targets of the current
// node (gorge putCounterAllSweep / allPlayersFor). With no ValidTgts$ it
// returns spec unchanged: the sweep is the whole battlefield. Only the
// plain ValidTgts$ Player spelling is read; any other selector is exotic
// in gorge and, for a sweep, would silently widen or narrow the set, so it
// refuses. The slot itself is declared by objOrTarget, which also
// reads TargetMin$/TargetMax$/TgtZone$ and friends.
func (lc *lowerCtx) sweepTgtPlayer(ps *params, spec string) (string, error) {
	vt, ok := ps.get("ValidTgts")
	if !ok {
		return spec, nil
	}
	if strings.TrimSpace(vt) != "Player" {
		return "", refuse("sweep ValidTgts %q", vt)
	}
	scoped, err := lc.sweepTgtPlayerSpec(spec)
	if err != nil {
		return "", err
	}
	if _, err := lc.objOrTarget(ps, "", zNone); err != nil {
		return "", err
	}
	ps.consume("TgtPrompt")
	return scoped, nil
}

// sweepTgtPlayerSpec is sweepTgtPlayer's filter half for a node whose
// player target slot is already declared (the second PutCounterAll batch).
func (lc *lowerCtx) sweepTgtPlayerSpec(spec string) (string, error) {
	f, err := lc.filter(spec)
	if err != nil {
		return "", err
	}
	if f.Players != 0 {
		return "", refuse("sweep ValidCards %q names players", spec)
	}
	for i := range f.Clauses {
		// A controller selector already in the sweep and the scope's
		// implicit TargetedPlayerCtrl would be two controller words in one
		// clause; ActivePlayerCtrl is a controller comparison, not a
		// selector, and would be read beside the scope bit. Both refuse.
		if f.Clauses[i].Ctrl != 0 || f.Clauses[i].CmpStat == cmpActiveCtrl {
			return "", refuse("sweep ValidCards %q with a controller selector", spec)
		}
	}
	return sweepAddTgtPlayerCtrl(spec, lc.p.isTypeWord), nil
}

// sweepAddTgtPlayerCtrl adds TargetedPlayerCtrl to every comma alternative of a
// Valid expression: X.TargetedPlayerCtrl, or X.p+TargetedPlayerCtrl when X
// already carries a property.
func sweepAddTgtPlayerCtrl(spec string, isType func(string) bool) string {
	alts := splitAlternatives(spec, isType)
	for i, alt := range alts {
		alt = strings.TrimSpace(alt)
		if strings.Contains(alt, ".") {
			alts[i] = alt + "+TargetedPlayerCtrl"
		} else {
			alts[i] = alt + ".TargetedPlayerCtrl"
		}
	}
	return strings.Join(alts, ",")
}

// refuseSweepTgtPlayerMulti is the lowerTail check that the kernel's
// entry-wide F2TgtPlayerCtrl can stand in for gorge's link-wide
// targetedGroup: the ability must declare exactly one player target. A
// second player target slot would make the kernel sweep permanents gorge
// leaves untouched. Every ab.Targets entry counts, whatever its Mode:
// ctrlRoleF2 matches every chosen player target of the stack entry, and in
// a multi-pick charm the other mode's chosen player is one of them.
func (lc *lowerCtx) refuseSweepTgtPlayerMulti() {
	for ai := range lc.out.Abilities {
		ab := lc.out.Abilities[ai]
		scoped := false
		for i := range ab.Prog {
			if ab.Prog[i].Note == sweepTgtPlayerNote {
				scoped = true
				break
			}
		}
		if !scoped {
			continue
		}
		n := 0
		for ti := range ab.Targets {
			t := &ab.Targets[ti]
			if t.F != nil && t.F.Players != 0 {
				n++
			}
		}
		if n > 1 {
			lc.refuseNode("card", "sweep ValidTgts$ Player beside another player target slot (the kernel matches every player target of the entry; gorge reads this link's)")
			return
		}
	}
}
