package kgen

// The triggered-object table: which Defined$ Triggered* spellings a trigger
// mode lets a body read, and the kernel object each names.
//
// gorge's TriggerHandler run params (Blocker, Target, Source,
// SourceController) are per mode, and each mode's kernel trigger record
// carries a different object as the entry's trigger object or player. This
// table is the one place that pairs the two: a row is (spelling, mode) -> a
// read that names the kernel reference when the mode's record carries the
// object for the trigger's shape. A spelling the table lists but no row
// admits refuses with the mode named (triggeredObjectRefused); a spelling the
// table does not list is none of its business (Triggered{Card,Player,...}
// keep their own arms in defined).
//
// A row is added only for a mode whose kernel trigger already records the
// object (lower_damagedoneonce.go and lower_trigsrcctrl.go for the damage
// modes, the btWatchers queue for BecomesTarget, the Blocks / AttackerBlocked
// queue for the blocker).

// trigRefRow admits spelling on a trigger of one of events.
type trigRefRow struct {
	spellings []string
	events    []string
	read      func(lc *lowerCtx, tr *lTrigger, spelling string) (lRef, bool)
}

var (
	blockerSpellings = []string{"TriggeredBlocker", "TriggeredBlockerLKICopy"}
	targetSpellings  = []string{"TriggeredTarget", "TriggeredTargetLKICopy"}
	srcCtrlSpelling  = []string{"TriggeredSourceController"}
)

var triggeredObjectTable = []trigRefRow{
	// Blocks: gorge TriggerBlocker is the blocker the ValidCard$ filter
	// matched. "Whenever this blocks" is the source; "whenever equipped /
	// enchanted creature blocks" is the host the source's attachment names.
	{blockerSpellings, []string{"Blocks"}, readBlocksBlocker},
	// AttackerBlocked by a creature (lower_trigzonebatch.go): the blocker's
	// own trigger is the source, the attacker's the blocker as trigger object.
	{blockerSpellings, []string{"AttackerBlocked"}, readBlockedByBlocker},
	// BecomesTarget watcher: the targeted permanent is the trigger object the
	// kernel's btWatchers queue carries (gorge becomesTargetReferent); an
	// LKI copy of it is the same object there.
	{targetSpellings, []string{"BecomesTarget"}, readWatchTarget},
	// DamageDone / DamageDealtOnce: the damaged object of a per-referent line,
	// or the damaged player.
	{targetSpellings, []string{"DamageDone", "DamageDealtOnce"}, readDamageTarget},
	// The controller of the causing spell or ability (BecomesTarget watcher
	// over an opponent's source) or of the damage source (a plain damage line,
	// whose trigger object is the source).
	{srcCtrlSpelling, []string{"BecomesTarget"}, readWatchSourceCtrl},
	{srcCtrlSpelling, []string{"DamageDone", "DamageDealtOnce"}, readDamageSourceCtrl},
}

// triggeredObjectRef resolves spelling v against the table for the current
// trigger. found is false when the table has no row for v at all.
func (lc *lowerCtx) triggeredObjectRef(v string) (ref lRef, found bool, err error) {
	var tr *lTrigger
	if lc.ab != nil {
		tr = lc.ab.Trigger
	}
	for _, row := range triggeredObjectTable {
		if !containsStr(row.spellings, v) {
			continue
		}
		found = true
		if tr == nil || !containsStr(row.events, tr.Event) {
			continue
		}
		if r, ok := row.read(lc, tr, v); ok {
			return r, true, nil
		}
	}
	if !found {
		return lRef{}, false, nil
	}
	return lRef{}, true, triggeredObjectRefused(v, tr)
}

// triggeredObjectRefused names the mode whose kernel record does not carry
// the object spelling v reads.
func triggeredObjectRefused(v string, tr *lTrigger) error {
	mode := "no trigger"
	if tr != nil {
		mode = "Mode$ " + tr.Event
	}
	switch v {
	case "TriggeredBlocker", "TriggeredBlockerLKICopy":
		return refuse("Defined$ %q outside a whenever-this-blocks trigger (%s)", v, mode)
	case "TriggeredSourceController":
		return refuse("Defined$ %q outside an opponent-source BecomesTarget watcher (%s)", v, mode)
	}
	return refuse("Defined$ %q outside a damage trigger the kernel's trigger object or player names (%s)", v, mode)
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func readBlocksBlocker(lc *lowerCtx, tr *lTrigger, v string) (lRef, bool) {
	if lc.emblemBlocks {
		// an Effect's Blocks emblem: the kernel hook's trigger object is the
		// blocker (dirac/trig_emblemblocks.go)
		return lRef{Kind: rTriggered}, true
	}
	if tr.F == nil || len(tr.F.Clauses) != 1 || tr.F.Players != 0 {
		return lRef{}, false
	}
	switch c := tr.F.Clauses[0]; c {
	case lClause{Flags: fSelf}, lClause{Types: tCreature, Flags: fSelf}:
		return lRef{Kind: rSelf}, true
	case lClause{Flags: fEquippedBy}, lClause{Flags: fEnchantedBy}:
		// the blocker is the source's host (the one RefAttachedCtrl's
		// controller read names for TriggeredBlockerController)
		return lRef{Kind: rEquipped}, true
	}
	return lRef{}, false
}

func readBlockedByBlocker(lc *lowerCtx, tr *lTrigger, v string) (lRef, bool) {
	if !tr.ByCreature {
		return lRef{}, false
	}
	if tr.AnchorBlocker {
		return lRef{Kind: rSelf}, true
	}
	return lRef{Kind: rTriggered}, true
}

func readWatchTarget(lc *lowerCtx, tr *lTrigger, v string) (lRef, bool) {
	if !tr.Watch {
		return lRef{}, false
	}
	return lRef{Kind: rTriggered}, true
}

func readDamageTarget(lc *lowerCtx, tr *lTrigger, v string) (lRef, bool) {
	// a per-referent line whose ValidTarget$ is an object filter carries the
	// damaged permanent as the trigger object (gorge's LKI copy names the
	// same object; a recipient that already left is a gone object, and an
	// effect on a gone object is a no-op in the kernel as in gorge)
	if ddoObjectReferent(tr) {
		return lRef{Kind: rTriggered}, true
	}
	// The player dealt the damage. The kernel's trigger player there is the
	// opponent of the source's controller (two players), the damaged player
	// when the damage is combat damage to a player, or damage to an opponent
	// from a source the listener controls. seat-exact: with more seats the
	// kernel's trigger player is the damaged seat itself (dirac dmgSeat).
	if lc.damagedPlayerIsTriggerPlayer() || ddoPlayerReferent(tr) {
		return lRef{Kind: rTriggeredPlayer}, true
	}
	return lRef{}, false
}

func readWatchSourceCtrl(lc *lowerCtx, tr *lTrigger, v string) (lRef, bool) {
	// gorge definedSpecTriggeredSourceController: the controller of the spell
	// or ability that targeted the watched permanent. The watcher arm that
	// sets Whose = opp is ValidSource$ SpellAbility.OppCtrl, so the
	// controller is an opponent of the listener's controller, read
	// seat-exact below (CR 603.2; Thunderbreak Regent).
	if tr.Watch && tr.SrcSpell {
		// a spell source form: the controller is the spell's, whichever seat
		// it is (the kernel's trigger player, btWatchers); a YouCtrl form is
		// the listener's own controller
		if tr.Whose == whoYou {
			return lRef{Kind: rYou}, true
		}
		if tr.Whose == whoAny {
			return lRef{Kind: rTriggeredPlayer}, true
		}
	}
	if !tr.Watch || tr.Whose != whoOpp {
		return lRef{}, false
	}
	// an opponent's spell or ability: the targeting entry's controller is
	// read as the trigger is queued, while the entry is on the stack
	// (kop.TrigNSrcCtrl, kernel btTriggerPlayer), the source forms' trigger
	// player above (seat-exact)
	if !tr.SrcSpell && !tr.SrcAbility {
		tr.SrcCtrl = true
	}
	return lRef{Kind: rTriggeredPlayer}, true
}

func readDamageSourceCtrl(lc *lowerCtx, tr *lTrigger, v string) (lRef, bool) {
	if !dmgSourceIsTriggerObj(tr) { // lower_trigsrcctrl.go
		return lRef{}, false
	}
	return lRef{Kind: rTriggeredTgtCtrl}, true
}
