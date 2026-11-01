package kgen

// Mode$ Explores and Mode$ CollectEvidence.
//
// gorge trigmatch exploresMatches: the completed events.Explore record; Obj
// is the explorer (ValidCard$, from the source's controller), IDs[0] the
// revealed card (ValidExplored$, matched in whatever zone it now is, so only
// zone-independent card-type predicates lower). collectEvidenceMatches:
// ValidPlayer$ against the collecting player.

// exploresTrigger lowers Mode$ Explores: ValidCard$ is Filter, a type-only
// ValidExplored$ is Filter2. ValidPlayer$ and every other key refuse.
func (lc *lowerCtx) exploresTrigger(tp *params, tr *lTrigger) error {
	if err := trigExplEvZone(tr, "Explores"); err != nil {
		return err
	}
	for _, k := range sortedParamKeys(tp.m) {
		switch k {
		case "Mode", "Execute", "ValidCard", "ValidExplored", "TriggerZones", "TriggerDescription", "OptionalDecider":
		default:
			return refuse("Explores %s$", k)
		}
	}
	var err error
	if v, ok := tp.get("ValidCard"); ok {
		if tr.F, err = lc.filter(v); err != nil {
			return err
		}
	}
	if v, ok := tp.get("ValidExplored"); ok {
		f, err := lc.filter(v)
		if err != nil {
			return err
		}
		if f.Players != 0 || len(f.Clauses) == 0 {
			return refuse("Explores ValidExplored %q", v)
		}
		for _, c := range f.Clauses {
			if c != (lClause{Types: c.Types, NotTypes: c.NotTypes}) {
				return refuse("Explores ValidExplored %q", v)
			}
		}
		tr.F2 = f
	}
	tr.Event = "Explores"
	return nil
}

// collectEvidenceTrigger lowers Mode$ CollectEvidence: ValidPlayer$ is Whose.
func (lc *lowerCtx) collectEvidenceTrigger(tp *params, tr *lTrigger) error {
	if err := trigExplEvZone(tr, "CollectEvidence"); err != nil {
		return err
	}
	for _, k := range sortedParamKeys(tp.m) {
		switch k {
		case "Mode", "Execute", "ValidPlayer", "TriggerZones", "TriggerDescription", "OptionalDecider":
		default:
			return refuse("CollectEvidence %s$", k)
		}
	}
	w, ok := whoOfPlayer(tp.str("ValidPlayer"))
	if !ok {
		return refuse("CollectEvidence ValidPlayer %q", tp.str("ValidPlayer"))
	}
	tr.Event, tr.Whose = "CollectEvidence", w
	return nil
}

// trigExplEvZone refuses a line outside the battlefield (the kernel scans
// the battlefield only).
func trigExplEvZone(tr *lTrigger, mode string) error {
	if tr.Zone != zNone && tr.Zone != zBattlefield {
		return refuse("%s trigger outside the battlefield", mode)
	}
	return nil
}
