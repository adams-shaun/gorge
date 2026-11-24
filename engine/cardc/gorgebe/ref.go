package gorgebe

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() { refFn = printRef }

// playerWord names a player operand: You, Opp, Active or TargetedPlayer.
func playerWord(p *pr, r *ir.Ref) (string, bool) {
	if r == nil {
		return "", false
	}
	switch r.Kind {
	case ir.RefYou:
		return "You", true
	case ir.RefActivePlayer:
		return "Active", true
	case ir.RefTarget:
		if targetIsPlayer(p, r.Slot) {
			return "TargetedPlayer", true
		}
	case ir.RefAll:
		f := r.Filter
		if f != nil && f.Op == ir.FNot && len(f.Args) == 1 && isYouFilter(f.Args[0]) {
			return "YouDont", true
		}
		if f == nil || f.Op != ir.FPred || f.Ref == nil || f.Ref.Kind != ir.RefYou {
			return "", false
		}
		switch f.Pred {
		case ir.PPlayerIs:
			return "You", true
		case ir.POpponentOf:
			return "Opp", true
		}
	}
	return "", false
}

// targetIsPlayer reports whether the innermost frame's slot is a player target.
func targetIsPlayer(p *pr, slot uint8) bool {
	f := p.frame()
	if f == nil || int(slot) >= len(f.targets) {
		return false
	}
	t := f.targets[slot]
	return t != nil && t.Filter != nil && t.Filter.Subject == ir.SubjPlayer
}

func targetBound(p *pr, slot uint8) bool {
	f := p.frame()
	return f != nil && int(slot) < len(f.targets)
}

func refBad(r *ir.Ref, p *pr) error {
	what := "ref:" + r.Kind.String()
	if (r.Kind == ir.RefEventObject || r.Kind == ir.RefEventPlayer) && p.ev != ir.EvNone {
		what += "@" + p.ev.String()
	}
	return unprintable(what, r.Src)
}

func printRef(p *pr, r *ir.Ref) (string, error) {
	r = slotRef(r)
	bad := func() (string, error) { return "", refBad(r, p) }
	switch r.Kind {
	case ir.RefSelf:
		return "Self", nil
	case ir.RefYou:
		return "You", nil
	case ir.RefTarget:
		if !targetBound(p, r.Slot) {
			return "", unprintable("ref:target-unbound", r.Src)
		}
		if p.frame().parent(r.Slot) {
			if targetIsPlayer(p, r.Slot) {
				return "", unprintable("ref:parent-player", r.Src)
			}
			return "ParentTarget", nil
		}
		if targetIsPlayer(p, r.Slot) {
			return "TargetedPlayer", nil
		}
		return "Targeted", nil
	case ir.RefEventObject:
		if p.evSelf {
			return "Self", nil
		}
		switch p.ev {
		case ir.EvEnters, ir.EvCountersPut:
			return "TriggeredCard", nil
		case ir.EvDies, ir.EvPutIntoGraveyard, ir.EvLeaves:
			return "TriggeredNewCardLKICopy", nil
		case ir.EvAttacks:
			return "TriggeredAttacker", nil
		case ir.EvDealsCombatDamage, ir.EvDealsCombatDamageToPlayer, ir.EvDealsNoncombatDamageToPlayer:
			return "TriggeredSourceLKICopy", nil
		case ir.EvCast:
			return "TriggeredSpellAbility", nil
		case ir.EvBecomesTarget:
			return "TriggeredSourceSA", nil
		case ir.EvWouldEnter, ir.EvWouldPutCounters, ir.EvWouldDealDamage, ir.EvWouldCreateTokens, ir.EvWouldDie:
			return "ReplacedCard", nil
		}
		return bad()
	case ir.RefEventPlayer:
		switch p.ev {
		case ir.EvGainsLife, ir.EvLosesLife, ir.EvUpkeep, ir.EvEndStep:
			return "TriggeredPlayer", nil
		case ir.EvDraws:
			return "TriggeredCardController", nil
		case ir.EvDealsCombatDamageToPlayer, ir.EvDealsNoncombatDamageToPlayer:
			return "TriggeredTarget", nil
		case ir.EvCast:
			return "TriggeredActivator", nil
		}
		return bad()
	case ir.RefControllerOf:
		if r.Of == nil {
			return bad()
		}
		switch r.Of.Kind {
		case ir.RefTarget:
			return "TargetedController", nil
		case ir.RefSelf:
			return "You", nil
		case ir.RefEventObject:
			switch p.ev {
			case ir.EvBecomesTarget:
				return "TriggeredSourceSAController", nil
			case ir.EvCast:
				return "TriggeredActivator", nil
			case ir.EvEnters, ir.EvDies, ir.EvPutIntoGraveyard:
				return "TriggeredCardController", nil
			}
		}
		return bad()
	case ir.RefAttachedTo:
		if r.Of != nil && r.Of.Kind == ir.RefSelf {
			if w := p.attachWord(); w != "" {
				return w, nil
			}
		}
		return bad()
	case ir.RefOwnerOf:
		if r.Of != nil && r.Of.Kind == ir.RefTarget {
			return "TargetedOwner", nil
		}
		return bad()
	case ir.RefPlayerOrControllerOf:
		if r.Of != nil && r.Of.Kind == ir.RefTarget {
			return "TargetedOrController", nil
		}
		return bad()
	case ir.RefAll:
		if r.Filter == nil || r.Filter.Subject != ir.SubjPlayer {
			return bad()
		}
		v, err := p.valid(r.Filter)
		if err != nil {
			return "", err
		}
		if v == "Opponent" {
			v = "Player.Opponent"
		}
		return v, nil
	}
	return bad()
}
