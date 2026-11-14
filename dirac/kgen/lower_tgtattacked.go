package kgen

// TargetsWithDefinedController$ TriggeredAttackedTarget ("target creature
// that player controls" under "whenever one or more Warriors you control
// attack a player": Gornog, the Red Reaper; Karazikar, the Eye Tyrant). The
// referent is the player the trigger's attackers attack, which the
// per-defender AttackersDeclared line (Trigger.DefPlayer set) binds as the
// defending player (triggeredAttackedTargetDefined). The battlefield slot
// is restricted to permanents that player controls with the kernel's
// defending-controller role (kop.F2DefendingCtrl: the non-active seat at two
// seats, the entry's bound defending seat past two), which the slot's
// legality recheck on resolution (CR 608.2b) reads the same way.
func (lc *lowerCtx) tgtCtrlAttackedTarget(t *lTarget) error {
	if _, err := lc.triggeredAttackedTargetDefined("TriggeredAttackedTarget"); err != nil {
		return refuse("TargetsWithDefinedController$ TriggeredAttackedTarget outside a per-defender AttackersDeclared trigger")
	}
	if !lc.trigBody || lc.ab.Latent {
		return refuse("TargetsWithDefinedController$ TriggeredAttackedTarget outside a trigger body")
	}
	f := t.F
	if t.Zone != zBattlefield || f == nil || f.Players != 0 || len(f.Clauses) == 0 || t.ParentPlus1 != 0 {
		return refuse("TargetsWithDefinedController$ TriggeredAttackedTarget: not a battlefield permanent slot")
	}
	nf := *f
	nf.Clauses = append([]lClause(nil), f.Clauses...)
	for i := range nf.Clauses {
		c := &nf.Clauses[i]
		if c.Owner != 0 || c.Ctrl != 0 {
			return refuse("TargetsWithDefinedController$ TriggeredAttackedTarget with an owner/controller filter")
		}
		if err := c.setWho(&c.Ctrl, whoDefending); err != nil {
			return err
		}
	}
	t.F = &nf
	return nil
}
