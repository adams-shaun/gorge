package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// effectStaticTriggerPair lowers a DB$ Effect that remembers its targeted
// creature and carries one StaticAbilities$ body beside a Triggers$ body. Two
// pairs are exact:
//
//   - a CantBlockBy static over the remembered creature (ExileOnMoved$
//     Battlefield) beside a combat DamageDealtOnce trigger of that creature
//     (Long River Lurker): the creature gets the unblockable record the
//     static-only Effect lowers to, and the trigger is the capturing emblem's
//     (emblem, with the body's first Remembered read naming the damage source);
//   - a CanAttackDefender static over the remembered creature (ForgetOnMoved$
//     Battlefield) beside a DamageDone trigger whose ValidSource$ is
//     Card.Self (The Pride of Hull Clade). In Forge that Card.Self is the
//     Effect card, which never deals damage, so gorge registers nothing for the
//     trigger (effectTriggerDealerIsEffect); the creature's real trigger is the
//     sibling Animate grant. The static is a capturing emblem.
//
// Anything else refuses by name.
func (lc *lowerCtx) effectStaticTriggerPair(ps *params, tr string) error {
	list, _ := ps.get("StaticAbilities")
	why := func(more string) error {
		return refuse("Effect StaticAbilities$ %q beside Triggers$ (%s)", list, more)
	}
	if _, ok := ps.get("Duration"); ok {
		return why("a Duration$")
	}
	if strings.TrimSpace(ps.str("RememberObjects")) != "Targeted" {
		return why("RememberObjects$ " + ps.str("RememberObjects"))
	}
	t, ok := cards.ParseTriggerLine(lc.face.SVars[tr])
	if !ok {
		return refuse("emblem trigger %q", tr)
	}
	ss, err := lc.effectStaticBodies(list)
	if err != nil {
		return err
	}
	if len(ss) != 1 {
		return why("several statics")
	}
	s := ss[0]
	inert := t.Mode == "DamageDone" && strings.EqualFold(strings.TrimSpace(t.Params["ValidSource"]), "Card.Self")
	switch {
	case inert && s.Mode == "CanAttackDefender":
	case t.Mode == "DamageDealtOnce" && s.Mode == "CantBlockBy" && strings.TrimSpace(t.Params["CombatDamage"]) == "True":
		// combat damage only: the emblem hook's once-per-source latch is
		// verified against a combat damage step, not a noncombat batch
	default:
		return why("static " + s.Mode + " with trigger " + t.Mode)
	}
	var capture lRef
	if _, ok := ps.m["ValidTgts"]; ok {
		if capture, err = lc.objOrTarget(ps, "", zBattlefield); err != nil {
			return err
		}
	} else if capture, err = lc.parentCapture(); err != nil {
		return err
	}
	if capture.Kind != rTarget {
		return why("RememberObjects$ Targeted over " + capture.String())
	}
	if inert {
		return lc.capturedCanAttack(ps, s, capture)
	}
	// the unblockable record, then the capturing emblem trigger over the same slot
	if err := lc.effectTargetStatic(ps, s, capture); err != nil {
		return err
	}
	lc.pairCap = &capture
	err = lc.emblem(ps, tr)
	lc.pairCap = nil
	if err != nil {
		return err
	}
	lc.out.Emblems[len(lc.out.Emblems)-1].CaptureLapse = true // ExileOnMoved$ Battlefield, read above
	return nil
}

// capturedCanAttack lowers The Pride of Hull Clade's Effect static: an
// end-of-turn emblem whose CanAttackDefender grant reaches only the captured
// creature, and lapses when that creature leaves the battlefield.
func (lc *lowerCtx) capturedCanAttack(ps *params, s cards.Static, capture lRef) error {
	if strings.TrimSpace(ps.str("ForgetOnMoved")) != "Battlefield" {
		return refuse("Effect CanAttackDefender ForgetOnMoved$ %q", ps.str("ForgetOnMoved"))
	}
	sp := newParams(s.Params)
	sp.consume("Mode", "Description")
	if strings.TrimSpace(sp.str("ValidCard")) != "Card.IsRemembered" {
		return refuse("Effect CanAttackDefender ValidCard$ %q", s.Params["ValidCard"])
	}
	if left := sp.unread(); len(left) > 0 {
		return refuse("Effect CanAttackDefender unread %v", left)
	}
	// the emblem's capture gate is the Remembered test: the filter keeps the
	// creature check only
	f, err := lc.filter("Creature")
	if err != nil {
		return err
	}
	ps.consume("RememberObjects", "Name", "Image", "Stackable")
	lc.out.Emblems = append(lc.out.Emblems, lEmblem{
		Statics:  []lStatic{{Kind: "CanAttackDefender", F: f}},
		UntilEOT: true, Capture: capture, CaptureSet: true, CaptureLapse: true,
	})
	lc.emit(lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rYou}, C: len(lc.out.Emblems) - 1})
	return nil
}
