package kgen

import "strings"

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
	v, ok := ps.get("AddAbilities")
	if !ok {
		return nil, nil
	}
	names := strings.Split(v, ",")
	if len(names) > 2 {
		return nil, refuse("ETB copy AddAbilities$ %q: more than two abilities", v)
	}
	o := &lOverride{}
	for _, n := range names {
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
	return o, nil
}
