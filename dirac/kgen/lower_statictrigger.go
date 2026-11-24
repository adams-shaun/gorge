package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/scriptfacts"
)

// staticTriggerCand reports whether a printed trigger line is an off-stack
// Static$ True candidate the body's proof settles after it lowers
// (lowerTriggered; staticTriggerBody).
func staticTriggerCand(tp *params, t cards.Trigger, zone string) (bool, error) {
	// gorge resolveStaticTriggers (static_trigger.go) resolves a queued
	// trigger whose line carries Static$ True off the stack, by the drain,
	// before the CR 603.3b ordering ask — gated on its ask-free judgement
	// (staticTriggerAskFree). The SpellCast arm (its mode case) keeps the
	// stricter rider-shape proof it already lowers. Every other printed
	// line reads the param here: a line with an optional/unless/cost rider
	// (gorge TriggerLineMayAsk) never resolves off the stack — gorge keeps
	// it in the queue and pushes it like any other trigger — so it lowers
	// as the ordinary trigger it stays; a TapsForMana line is the CR 605.3b
	// mana path (staticTriggerAskFree refuses it); the rest are the
	// off-stack candidates the body's proof settles after it lowers. A
	// registration (zone "Command") belongs to the delayed path
	// (lower_delayed.go reads its Static$ there; gorge's fire-time
	// DelayedStatic arm).
	staticCand := false
	if zone == "" && t.Mode != "SpellCast" {
		if v, ok := tp.get("Static"); ok {
			switch {
			case v != "True":
				return false, refuse("%s Static$ %q", t.Mode, v)
			case cards.TriggerLineMayAsk(t), t.Mode == "TapsForMana":
			default:
				staticCand = true
			}
		}
	}
	return staticCand, nil
}

// staticTriggerBody settles a non-SpellCast Static$ True candidate against
// the lowered trigger ab, and marks tr static when it holds.
func (lc *lowerCtx) staticTriggerBody(staticCand bool, ab *lAbility, tr *lTrigger, t cards.Trigger, svars map[string]string) error {
	if !staticCand {
		return nil
	}
	// The non-SpellCast off-stack candidate, settled against gorge's
	// staticTriggerAskFree from text alone. CR 603.3d announces a
	// triggered ability's targets on the stack; a static trigger never
	// is, so any target refuses, as does a cost window on the body
	// (triggerBodyNeedsCostWindow). Every chain API must be one of
	// staticBodyCompetes' safe list whose mayask entry carries no
	// replaceable event — {Animate, Cleanup, Effect, Pump, PumpAll,
	// StoreSVar}: an event-proposing API's gate (a replacement on the
	// board that can apply to the proposed event) is board-dependent,
	// gorge keeps such a trigger on the stack when it fires and the
	// kernel could not know when, so the card refuses rather than lower
	// wrongly. The text half (SAChainMayAskCond) must also read ask-free
	// with no conditional reason. Anything else refuses: the card stays
	// out of the kernel rather than resolve from the wrong place.
	if len(ab.Targets) != 0 {
		return refuse("static trigger body carries an unannounced target")
	}
	if c := t.Effect.ParamStr(cards.PKCost); c != "" {
		return refuse("static trigger body Cost$ %q", c)
	}
	for sa := t.Effect; sa != nil; sa = sa.Sub {
		switch sa.API {
		case "Animate", "Cleanup", "Effect", "Pump", "PumpAll", "StoreSVar":
			// mayask's eventless APIs that staticBodyCompetes' safe
			// list also holds: no board gate, no ask
		default:
			return refuse("static trigger body api %s resolves from the stack in gorge", sa.API)
		}
	}
	may, cond := cards.SAChainMayAskCond(t.Effect, svars, lc.face, true)
	if may || cond != 0 {
		return refuse("static trigger body may ask (may %v cond %#x)", may, cond)
	}
	tr.Static = true
	return nil
}

// etbRememberedRead refuses an ETB replacement program that reads the
// remembered set without writing it.
func etbRememberedRead(prog []lInstr) error {
	// the entry's own resolution wrote nothing before this move: a
	// remembered-set read here sees gorge's ctx unioned with the
	// spell's persistent event-backed list (rememberedWithSource) —
	// the write another ability's resolution made — where the kernel's
	// per-resolution set is empty (Naya Soulbeast's "X is the total
	// mana value of all cards revealed this way"). A program that
	// writes the set it reads is its own resolution's and lowers.
	writesRem := false
	for _, in := range prog {
		writesRem = writesRem || kremWrites(in)
	}
	if !writesRem {
		for _, in := range prog {
			if kremReads(in.String()) {
				return refuse("the ETB replacement reads Remembered written by another ability (the kernel's remembered set is per resolution; gorge's persists on the source)")
			}
		}
	}
	return nil
}

// untapRepl lowers an Untap replacement's shape into rl (lowerRepl's Untap case).
func (lc *lowerCtx) untapRepl(r cards.Repl, rp *params, rl *lRepl) error {
	if rp.str("Layer") != "CantHappen" {
		return refuse("Untap replacement shape")
	}
	vc := rp.str("ValidCard")
	if vc == "" {
		return refuse("Untap replacement without ValidCard$")
	}
	if _, ok := rp.get("ValidStepTurnToController"); !ok {
		// no step key: gorge cards.ReplUntap matches every Untap event
		// (rules/replacement_match.go), the "can't become untapped" class
		if r.With != nil {
			return refuse("Untap replacement ReplaceWith$ without ValidStepTurnToController$")
		}
		if err := lc.untapReplFilter(rp, rl, vc, "AnyUntap"); err != nil {
			return err
		}
		return nil
	}
	if rp.str("ValidStepTurnToController") != "You" {
		return refuse("Untap replacement shape")
	}
	rl.Kind = "DoesntUntap"
	if vc != "Card.Self" {
		return lc.othersDoesntUntap(rp, rl, vc)
	}
	return lc.selfDoesntUntapGate(rp, rl) // lower_untaprepl.go
}

// triggerIsPresentConds appends a trigger's IsPresent$ intervening-if to
// conds. A clause that only mirrors the trigger's own optional SubCounter
// cost (Sun Droplet; scriptfacts.PresentIsOwnCostPrecheck, gorge
// isPresentGateHolds) is no gate: the ability triggers (CR 603.2) and the
// unpayable cost does nothing as it resolves.
func (lc *lowerCtx) triggerIsPresentConds(t cards.Trigger, tp *params, tr *lTrigger, conds []lCond) ([]lCond, error) {
	ip, ok := tp.get("IsPresent")
	if !ok || scriptfacts.PresentIsOwnCostPrecheck(t) {
		return conds, nil
	}
	c, err := lc.triggerIsPresent(tp, tr, ip)
	if err != nil {
		return conds, err
	}
	return append(conds, c), nil
}

// triggerIsPresent lowers a trigger's IsPresent$ intervening-if (and its
// PresentDefined$ / PresentCompare$ / PresentZone$ riders) into a condition.
// A PresentDefined$ Self clause carries Zone zBattlefield: gorge
// presentClauseHolds answers it from the departed source's last known
// information (CR 608.2h: Graft, House Cartographer's Survival), while a plain
// IsPresent$ Card.Self+... counts the live battlefield, so a source destroyed
// in response fails it (Sun Droplet; kernel countPermCond).
func (lc *lowerCtx) triggerIsPresent(tp *params, tr *lTrigger, ip string) (lCond, error) {
	c, err := lc.triggerIsPresentCond(tp, tr, ip)
	if _, pd := tp.get("PresentDefined"); err == nil && pd && c.Kind == cPresent {
		c.Zone = zBattlefield
	}
	return c, err
}

func (lc *lowerCtx) triggerIsPresentCond(tp *params, tr *lTrigger, ip string) (lCond, error) {
	if pd, ok := tp.get("PresentDefined"); ok {
		// gorge presentClauseHolds: PresentDefined$ Self counts the
		// source alone, 1 when it is on the battlefield and matches
		// the spec (PresentZone$ unread on that path), so each
		// alternative is the spec narrowed to Self. Any other
		// Defined group refuses.
		if strings.TrimSpace(pd) != "Self" {
			return lCond{}, refuse("trigger PresentDefined$ %q", pd)
		}
		if _, ok := tp.get("PresentZone"); ok {
			return lCond{}, refuse("trigger PresentDefined$ beside PresentZone$")
		}
		alts := strings.Split(ip, ",")
		for i, a := range alts {
			a = strings.TrimSpace(a)
			if strings.Contains(a, ".") {
				alts[i] = a + "+Self"
			} else {
				alts[i] = a + ".Self"
			}
		}
		ip = strings.Join(alts, ",")
	}
	if strings.Contains(ip, "blockingSource") || strings.Contains(ip, "blockedBySource") {
		// the block relation read when the trigger fires: Lairwatch
		// Giant's Blocks trigger (IsPresent$ Creature.blockedBySource,
		// GE2) diverges from gorge in a lockstep game (the damage
		// division asked of another creature), so it stays refused
		return lCond{}, refuse("trigger IsPresent$ %q reads the block relation at trigger time", ip)
	}
	lc.atkPresent = tr.Event == "Attacks"
	f, err := lc.filter(ip)
	lc.atkPresent = false
	if err != nil {
		return lCond{}, err
	}
	if tr.Event == "ETB" && filterSelfOnly(tr.F) {
		// a self-enters intervening-if that reads the source's host
		// (Historian's Wisdom: IsPresent$ Permanent.EnchantedBy+greatestPower)
		// is checked by gorge with the Aura already attached (CR 303.4f);
		// the kernel queues the ETB trigger inside enter(), before
		// resolve.go attaches the Aura, so EnchantedBy matches nothing
		// and the trigger never fires.
		for _, c := range f.Clauses {
			if c.Flags&fEnchantedBy != 0 {
				return lCond{}, refuse("self-enters IsPresent$ %q reads EnchantedBy before the Aura attaches", ip)
			}
		}
	}
	c := lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
	if pc, ok := tp.get("PresentCompare"); ok {
		op, v, err := lc.cmpSpec(pc)
		if err != nil {
			return lCond{}, err
		}
		c.Op, c.A = op, v
	}
	if c, err = lc.triggeredPresentZoneCond(c, tp, ip); err != nil {
		return lCond{}, err
	}
	return c, nil
}
