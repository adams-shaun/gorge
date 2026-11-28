package mtgish

import "github.com/adams-shaun/gorge/engine/cardc/ir"

// Trigger variants of the proof slice. Subject is the event's actor, Object
// the other party; argument indexes below are mtgish tuple positions.

// trigSpec describes one trigger: its event and which argument is the
// subject, object, comparison and counter word (-1 for none).
type trigSpec struct {
	ev                      ir.EventKind
	subj, obj, cmp, counter int
}

func trigger(variant string, s trigSpec) {
	register("Trigger."+variant, func(c *Ctx, n node) (any, error) {
		t := &ir.Trigger{Event: s.ev}
		var err error
		if s.subj >= 0 {
			if t.Subject, err = c.Filter(argN(n, s.subj)); err != nil {
				return nil, err
			}
		}
		if s.obj >= 0 {
			if t.Object, err = c.Filter(argN(n, s.obj)); err != nil {
				return nil, err
			}
		}
		if s.cmp >= 0 {
			if t.Cmp, err = c.Compare(argN(n, s.cmp)); err != nil {
				return nil, err
			}
		}
		if s.counter >= 0 {
			if t.Word, err = filtStr(c, argN(n, s.counter)); err != nil {
				return nil, err
			}
		}
		return t, nil
	})
}

func init() {
	s := func(ev ir.EventKind) trigSpec { return trigSpec{ev, 0, -1, -1, -1} }
	so := func(ev ir.EventKind) trigSpec { return trigSpec{ev, 0, 1, -1, -1} }
	trigger("AtTheBeginningOfAPlayersEndStep", s(ir.EvEndStep))
	trigger("AtTheBeginningOfAPlayersUpkeep", s(ir.EvUpkeep))
	trigger("AtTheBeginningOfCombatDuringAPlayersTurn", s(ir.EvBeginCombat))
	trigger("WhenACreatureAttacks", s(ir.EvAttacks))
	trigger("WhenACreatureDealsCombatDamage", s(ir.EvDealsCombatDamage))
	trigger("WhenACreatureDealsCombatDamageToAPlayer", so(ir.EvDealsCombatDamageToPlayer))
	trigger("WhenASourceDealsNoncombatDamageToAPlayer", so(ir.EvDealsNoncombatDamageToPlayer))
	trigger("WhenAPermanentBecomesTheTargetOfASpell", so(ir.EvBecomesTarget))
	trigger("WhenAPermanentBecomesUntapped", s(ir.EvUntaps))
	trigger("WhenAPermanentDies", s(ir.EvDies))
	trigger("WhenAPermanentEntersTheBattlefield", s(ir.EvEnters))
	trigger("WhenAPermanentEntersTheBattlefieldUnderAPlayersControl", so(ir.EvEnters))
	trigger("WhenAPermanentIsPutIntoAPlayersGraveyard", so(ir.EvPutIntoGraveyard))
	trigger("WhenAPermanentLeavesTheBattlefield", s(ir.EvLeaves))
	trigger("WhenAPlayerAttacks", s(ir.EvPlayerAttacks))
	trigger("WhenAPlayerAttacksWithANumberOfCreatures", trigSpec{ir.EvPlayerAttacks, 0, 2, 1, -1})
	trigger("WhenAPlayerCastsASpell", so(ir.EvCast))
	trigger("WhenAPlayerDiscardsACard", so(ir.EvDiscards))
	trigger("WhenAPlayerDrawsACard", s(ir.EvDraws))
	trigger("WhenAPlayerDrawsTheirNthCardEachTurn", trigSpec{ir.EvDraws, 0, -1, 1, -1})
	trigger("WhenAPlayerGainsLife", s(ir.EvGainsLife))
	trigger("WhenAPlayerLosesLife", s(ir.EvLosesLife))
	trigger("WhenAPlayerPlaysALand", so(ir.EvPlaysLand))
	trigger("WhenAPlayerPutsAnyNumberOfCountersOfTypeOnAPermanent", trigSpec{ir.EvCountersPut, 0, 2, -1, 1})
	trigger("WhenAnyNumberOfCountersOfTypeArePutOnAPermanent", trigSpec{ir.EvCountersPut, -1, 1, -1, 0})

	register("Trigger.WhenAPlayerGainsLifeForTheFirstTimeEachTurn", func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Trigger{Event: ir.EvGainsLife, Subject: f, Cmp: &ir.Compare{Op: ir.CmpEQ, V: constV(1)}}, nil
	})
	register("Trigger.If", func(c *Ctx, n node) (any, error) {
		cond, err := c.Cond(argN(n, 0))
		if err != nil {
			return nil, err
		}
		t, err := c.Trigger(argN(n, 1))
		if err != nil {
			return nil, err
		}
		t.Cond = andCond(cond, t.Cond)
		return t, nil
	})
	register("Trigger.Or", func(c *Ctx, n node) (any, error) {
		t := &ir.Trigger{Event: ir.EvOr}
		for _, x := range list(rawArgs(n)) {
			a, err := c.Trigger(x)
			if err != nil {
				return nil, err
			}
			t.Args = append(t.Args, a)
		}
		return t, nil
	})
}
