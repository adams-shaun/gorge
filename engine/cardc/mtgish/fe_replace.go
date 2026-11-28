package mtgish

import (
	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// Replacement and prevention rules, the events they replace and the actions
// that replace them.

// replRule registers a Rule that is {AbReplacement, Trigger: arg0 event,
// Body: arg1 steps}.
func replRule(key string) {
	register(key, func(c *Ctx, n node) (any, error) {
		tr, err := c.Trigger(argN(n, 0))
		if err != nil {
			return nil, err
		}
		body, err := c.StepList(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		return &ir.Ability{Kind: ir.AbReplacement, Trigger: tr, Body: body}, nil
	})
}

// evFilter registers an event variant {Event, Subject: arg0 filter}.
func evFilter(key string, ev ir.EventKind) {
	register(key, func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Trigger{Event: ev, Subject: f}, nil
	})
}

// stepConst registers a variant that is one fixed step, built fresh each time.
func stepConst(key string, mk func() *ir.Step) {
	register(key, func(*Ctx, node) (any, error) { return mk(), nil })
}

func init() {
	register("Rule.AsPermanentEnters", func(c *Ctx, n node) (any, error) {
		r, err := argRef(c, n, 0)
		if err != nil {
			return nil, err
		}
		body, err := c.StepList(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		return &ir.Ability{Kind: ir.AbReplacement, Trigger: &ir.Trigger{Event: ir.EvWouldEnter, Ref: r}, Body: body}, nil
	})
	for _, k := range []string{"ReplaceWouldEnter", "ReplaceWouldPutCounters", "ReplaceWouldDealDamage",
		"ReplaceAnEffectWouldCreateAnyNumberOfTokens", "PreventDamage"} {
		replRule("Rule." + k)
	}

	// Events.
	evFilter("ReplacableEventWouldEnter.APermanentWouldEnterTheBattlefield", ir.EvWouldEnter)
	register("ReplacableEventWouldPutCounters.AnEffectWouldPutCountersOnAPermanent", func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Trigger{Event: ir.EvWouldPutCounters, Object: f}, nil
	})
	register("ReplacableEventWouldDealDamage.DamageWouldBeDealtByASourceToARecipient", func(c *Ctx, n node) (any, error) {
		s, err := c.Filter(argN(n, 0))
		if err != nil {
			return nil, err
		}
		o, err := c.Filter(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Trigger{Event: ir.EvWouldDealDamage, Subject: s, Object: o}, nil
	})
	register("ReplacableEventAnEffectWouldCreateAnyNumberOfTokens.AnEffectWouldCreateAnyNumberOfTokensUnderAPlayersControl", func(c *Ctx, n node) (any, error) {
		s, err := c.Filter(argN(n, 0))
		if err != nil {
			return nil, err
		}
		o, err := c.Filter(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Trigger{Event: ir.EvWouldCreateTokens, Subject: s, Object: o}, nil
	})
	evFilter("ReplacableEventWouldPutIntoGraveyard.APermanentWouldDie", ir.EvWouldDie)
	register("EventPreventDamage.CombatDamageWouldBeDealtToRecipient", func(c *Ctx, n node) (any, error) {
		r, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Trigger{Event: ir.EvWouldDealDamage, Ref: r, Word: "combat"}, nil
	})
	register("EventPreventDamage.NoncombatDamageWouldBeDealtToARecipient", func(c *Ctx, n node) (any, error) {
		o, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Trigger{Event: ir.EvWouldDealDamage, Object: o, Word: "noncombat"}, nil
	})
	register("DamageRecipientsList.APermanent", func(c *Ctx, n node) (any, error) { return c.Filter(rawArgs(n)) })
	register("DamageRecipientsList.APlayerOrAPermanent", func(c *Ctx, n node) (any, error) {
		pl, err := c.Filter(argN(n, 0))
		if err != nil {
			return nil, err
		}
		pm, err := c.Filter(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Filter{Subject: ir.SubjObject, Op: ir.FAny, Args: []*ir.Filter{pl, pm}}, nil
	})

	// ReplacementActionWouldEnter.
	stepConst("ReplacementActionWouldEnter.EntersTapped", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbModifyEnter, Enter: &ir.Enter{Tapped: true}}
	})
	register("ReplacementActionWouldEnter.EntersWithACounter", func(c *Ctx, n node) (any, error) {
		w, err := counterWord(c, rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbModifyEnter, Enter: &ir.Enter{Counter: w, N: refConst1()}}, nil
	})
	register("ReplacementActionWouldEnter.EntersWithNumberCounters", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		w, err := counterWord(c, argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbModifyEnter, Enter: &ir.Enter{Counter: w, N: v}}, nil
	})
	register("ReplacementActionWouldEnter.ChooseAColor", func(c *Ctx, n node) (any, error) {
		cs, err := colorSet(c, rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbChooseColor, Colors: cs}, nil
	})
	stepConst("ReplacementActionWouldEnter.ChooseACreatureType", func() *ir.Step { return &ir.Step{Verb: ir.VerbChooseCreatureType} })
	register("ReplacementActionWouldEnter.ChooseANumberGreaterThanNumber", func(c *Ctx, n node) (any, error) {
		return numStep(c, n)
	})
	register("ReplacementActionWouldEnter.If", func(c *Ctx, n node) (any, error) {
		cond, err := c.Cond(argN(n, 0))
		if err != nil {
			return nil, err
		}
		then, err := c.StepList(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbIf, Cond: cond, Then: then}, nil
	})
	register("ReplacementActionWouldEnter.Unless", func(c *Ctx, n node) (any, error) {
		cond, err := c.Cond(argN(n, 0))
		if err != nil {
			return nil, err
		}
		then, err := c.StepList(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CNot, Args: []*ir.Cond{cond}}, Then: then}, nil
	})
	register("ReplacementActionWouldEnter.IfPassesFilter", func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(argN(n, 0))
		if err != nil {
			return nil, err
		}
		then, err := c.StepList(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CRefMatches, Ref: refEvent(), Filter: f}, Then: then}, nil
	})
	register("ReplacementActionWouldEnter.MayCost", func(c *Ctx, n node) (any, error) {
		cost, err := c.Cost(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbPayCost, Optional: true, Cost: cost}, nil
	})

	// Other replacement actions.
	setAmount := func(key string) {
		register(key, func(c *Ctx, n node) (any, error) {
			v, err := c.Value(rawArgs(n))
			if err != nil {
				return nil, err
			}
			return &ir.Step{Verb: ir.VerbSetAmount, N: v}, nil
		})
	}
	setAmount("ReplacementActionWouldPutCounters.PutNewAmount")
	setAmount("ReplacementActionWouldDealDamage.DealDamageInstead")
	register("ReplacementActionAnEffectWouldCreateAnyNumberOfTokens.CreateTokensInstead", func(c *Ctx, n node) (any, error) {
		then, err := c.StepList(list(rawArgs(n)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbDo, Then: then}, nil
	})
	stepConst("ReplacementActionWouldPutIntoGraveyard.ExileItInstead", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbMove, Obj: refEvent(), To: ir.ZoneExile}
	})
	stepConst("ActionPreventDamage.PreventThatDamage", func() *ir.Step { return &ir.Step{Verb: ir.VerbPrevent} })
}

// numStep is {VerbChooseNumber, N: const arg}.
func numStep(c *Ctx, n node) (any, error) {
	v, ok := intOf(rawArgs(n))
	if !ok {
		return nil, &RefusalError{Key: "ReplacementActionWouldEnter.ChooseANumberGreaterThanNumber:value", Path: c.Src()}
	}
	return &ir.Step{Verb: ir.VerbChooseNumber, N: constV(int32(v))}, nil
}
