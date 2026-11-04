package kgen

// assimilate lowers DB$ Assimilate (gorge effects/assimilate.go) of a creature card
// to a return under your control with a +1/+1 counter and an override that applies
// as the card enters: an added Artifact, Creature and Borg, and no other creature type.
// Two shapes lower: a graveyard target (Borg Queen), and Defined$ ChosenCard after a
// ChooseCard among the exile cards (Borg Cube, Assimilation Assessor: "assimilate a
// creature card exiled with it"), the chosen card still in exile as the ability
// resolves. A battlefield creature (gorge's control-gain arm) stays refused.
func (lc *lowerCtx) assimilate(ps *params) error {
	ret := map[string]string{"Destination": "Battlefield", "GainControl": "True",
		"WithCountersType": "P1P1", "WithCountersAmount": "1"}
	var obj lRef
	switch {
	case ps.is("TgtZone", "Graveyard"):
		var err error
		if obj, err = lc.objOrTarget(ps, "Self", zGraveyard); err != nil {
			return err
		}
		ret["Origin"], ret["Defined"] = "Graveyard", "Targeted"
	case ps.is("Defined", "ChosenCard") && lc.chosenFromExile():
		ps.consume("Defined")
		obj = lRef{Kind: rChosen}
		ret["Origin"], ret["Defined"] = "Exile", "ChosenCard"
	default:
		return refuse("assimilate not of a graveyard target")
	}
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

// chosenFromExile reports whether the ability's program so far holds exactly
// one ChooseCard, a mandatory pick among the exile cards, so the chosen set is
// at most one card in exile.
func (lc *lowerCtx) chosenFromExile() bool {
	if lc.ab == nil {
		return false
	}
	n := 0
	ok := false
	for _, in := range lc.ab.Prog {
		if in.Op != "ChooseCard" {
			continue
		}
		n++
		ok = in.C == zExile && in.CS == ""
	}
	return n == 1 && ok
}
