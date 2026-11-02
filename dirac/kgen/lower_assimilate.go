package kgen

// assimilate lowers DB$ Assimilate of a graveyard target (gorge effects/assimilate.go)
// to a return under your control with a +1/+1 counter and an override that applies
// as the card enters: an added Artifact, Creature and Borg, and no other creature type.
func (lc *lowerCtx) assimilate(ps *params) error {
	if !ps.is("TgtZone", "Graveyard") {
		return refuse("assimilate not of a graveyard target")
	}
	obj, err := lc.objOrTarget(ps, "Self", zGraveyard)
	if err != nil {
		return err
	}
	ret := map[string]string{"Origin": "Graveyard", "Destination": "Battlefield", "GainControl": "True",
		"WithCountersType": "P1P1", "WithCountersAmount": "1", "Defined": "Targeted"}
	if err := lc.changeZone(newParams(ret)); err != nil {
		return err
	}
	an := newParams(map[string]string{"Types": "Artifact,Creature,Borg", "RemoveCreatureTypes": "True", "Duration": "Permanent"})
	o, dur, _, err := lc.animateOverride(an, obj)
	if err != nil {
		return err
	}
	lc.emitOverride(obj, o, dur+"|enterWith")
	return nil
}
