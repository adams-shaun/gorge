package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// cloneEnterGrant reads an as-enters copy's AddAbilities$ (Mercurial
// Pretender: "enter as a copy of a creature you control, except it has
// '{2}{U}{U}: Return this creature to its owner's hand.'"). gorge's effClone
// folds the grant into the copy's copiable values (CR 707.9b; a CopyExcept
// abilities pair), so the copy has it as one of its own abilities and a copy
// of the copy has it too (CR 707.3). The kernel mirrors that: each named SVar
// lowers as a latent activated ability of this card (the Equipment-grant
// shape, grantedAbility), held in an override record of its own that
// OpBecomeCopy (A = the record's index+1) applies to the copy once a template
// was picked, sourced from the printed card and marked ovCopyGrant; a later
// as-enters copy of that copy carries the record (copyGrantsAdd). Entering as
// itself grants nothing. At most two abilities (kop.Override.Grant).
func (lc *lowerCtx) cloneEnterGrant(ps *params) (*lOverride, error) {
	o := &lOverride{}
	if v, ok := ps.get("AddAbilities"); ok {
		for _, n := range strings.Split(v, ",") {
			n = strings.TrimSpace(n)
			if n == "" {
				return nil, refuse("ETB copy AddAbilities$ %q", v)
			}
			idx, err := lc.grantedAbility(n)
			if err != nil {
				return nil, err
			}
			if lc.out.Abilities[idx].Kind != akActivated {
				return nil, refuse("ETB copy AddAbilities$ %q is not an activated ability", n)
			}
			o.GrantAbility = append(o.GrantAbility, idx)
		}
	}
	// AddTriggers$ (Phantasmal Image, Copycrook, Progenitor Mimic, Vesuvan
	// Doppelganger): each named SVar is a trigger line of this face, lowered
	// as a latent trigger the copy is granted beside any AddAbilities$ (CR
	// 707.9b, gorge clone.go folds it into the copy's copiable values).
	// AddSVars$ names the SVars the granted bodies read: they stay on the
	// printed face the latent abilities lower from, so each must exist there.
	if v, ok := ps.get("AddTriggers"); ok {
		for _, n := range strings.Split(v, ",") {
			idx, err := lc.cloneGrantTrigger(strings.TrimSpace(n))
			if err != nil {
				return nil, err
			}
			o.GrantAbility = append(o.GrantAbility, idx)
		}
	}
	if sv, ok := ps.get("AddSVars"); ok {
		if len(o.GrantAbility) == 0 {
			return nil, refuse("ETB copy AddSVars$ without a grant")
		}
		for _, n := range strings.Split(sv, ",") {
			if _, ok := lc.face.SVars[strings.TrimSpace(n)]; !ok {
				return nil, refuse("ETB copy AddSVars$ %q not on the face", n)
			}
		}
	}
	if len(o.GrantAbility) == 0 {
		return nil, nil
	}
	if len(o.GrantAbility) > 2 {
		return nil, refuse("ETB copy grants %d abilities: more than two", len(o.GrantAbility))
	}
	return o, nil
}

// cloneGrantTrigger lowers one AddTriggers$ SVar of an as-enters copy as a
// latent triggered ability of this card and returns its index.
func (lc *lowerCtx) cloneGrantTrigger(name string) (int, error) {
	lc.granted++
	defer func() { lc.granted-- }()
	t, ok := cards.ParseTriggerLine(lc.face.SVars[name])
	if !ok {
		return 0, refuse("ETB copy AddTriggers$ %q", name)
	}
	if t.Effect = cards.ResolveSVar(lc.face.SVars, t.Params["Execute"]); t.Effect == nil {
		return 0, refuse("ETB copy AddTriggers$ %q body", name)
	}
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) != idx+1 {
		// lowerTriggered appends exactly one ability: the grant record holds
		// this one (Grant indexes it), so a second emission would stay a
		// printed ability of the carrier the copy is never granted (an
		// Eminence trigger with TriggerZones$ Command,Battlefield would).
		return 0, refuse("ETB copy AddTriggers$ %q lowered %d abilities: want one", name, len(lc.out.Abilities)-idx)
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = "Granted"
	return idx, nil
}
