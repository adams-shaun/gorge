package kgen

import (
	"regexp"
	"strings"
)

// Two remember writers whose set a later link of the same resolution reads
// (and a closing Cleanup ClearRemembered$ forgets): kremWrites sees each by
// its note, so the reader no longer looks cross-ability.

// noteRememberPumped marks the ExtRememberAll a PumpAll RememberPumped$ emits.
const noteRememberPumped = "remember pumped"

// pumpAllRemember reads PumpAll's RememberPumped$ True (gorge effPumpAll
// remembers every permanent its walk pumps): an ExtRememberAll over the same
// filter runs right before the pump, which matches the same permanents.
// A ValidTgts$ player scope refuses (the remember op sweeps every seat).
func (lc *lowerCtx) pumpAllRemember(ps *params, f *lFilter, scope lRef) error {
	rp, err := rememberParam(ps, "RememberPumped")
	if err != nil || !rp {
		return err
	}
	if scope.Kind != rNone {
		return refuse("PumpAll RememberPumped$ with a ValidTgts$ player scope")
	}
	lc.emit(lInstr{Op: "RememberAll", F: f, Note: noteRememberPumped})
	return nil
}

// remLKIValueRead: a card SVar that reads a remembered object's
// characteristics or controller through a count head, which gorge answers
// from Destroy's RememberLKI$ snapshot (count_ref_property.go) and the kernel
// cannot.
var remLKIValueRead = regexp.MustCompile(`\b(RememberedLKI\$|Remembered\$|RememberedController|PlayerCountRemembered)`)

// destroyRememberLKIDestroyed reads Destroy's RememberLKI$ True without
// AlwaysRemember$ (gorge effDestroy: the target is remembered only when the
// destroy put it into a graveyard, with its LKI snapshot): the kernel's
// RememberDestroyed$ path (C bit 1, destroyRemember) for a one-object
// target. A later ConditionDefined$ RememberedLKI reads the remembered set
// (gorge effDestroy records the last controller in Snap.ChangeZone and
// rememberedControllerLKI reads it: C bit 4 marks the kernel's LKI so a
// remembered read takes the last-known controller, not the owner). A card
// whose SVars read the snapshot's characteristics refuses. done: the shape
// was handled here.
func (lc *lowerCtx) destroyRememberLKIDestroyed(ps *params, in *lInstr) (done bool, err error) {
	if _, always := ps.m["AlwaysRemember"]; always || strings.TrimSpace(ps.m["RememberLKI"]) != "True" {
		return false, nil
	}
	if _, rd := ps.m["RememberDestroyed"]; rd {
		return false, nil // destroyRememberDestroyed refuses the pair
	}
	if in.Obj.Kind != rTarget {
		return true, refuse("Destroy RememberLKI$ of %v", in.Obj)
	}
	if tm, ok := ps.m["TargetMax"]; ok && strings.TrimSpace(tm) != "1" {
		return true, refuse("Destroy RememberLKI$ with TargetMax$ %q", tm)
	}
	for _, v := range lc.face.SVars {
		if remLKIValueRead.MatchString(v) {
			return true, refuse("Destroy RememberLKI$ beside a remembered characteristic read")
		}
	}
	ps.consume("RememberLKI")
	in.C |= 2 | 4
	in.Note = noteRememberDestroyed
	return true, nil
}
