package mtgish

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// Static, characteristic-defining and cast-effect rules, and the effect
// enums they hold (StaticLayerEffect, PermanentRule, PlayerEffect,
// CastEffect, SpellEffect, SettableColor).

// static is the one step every static ability has: the effects on obj for as
// long as the ability applies.
func static(obj *ir.Ref, effs []*ir.Effect) *ir.Program {
	return &ir.Program{Steps: []*ir.Step{{Verb: ir.VerbApplyEffect, Obj: obj, Effects: effs, Dur: ir.DurWhileActive}}}
}

// allZones is every zone a characteristic-defining ability works in (CR 604.3).
var allZones = func() ir.ZoneSet {
	var zs ir.ZoneSet
	for z := ir.ZoneBattlefield; z <= ir.ZoneCommand; z++ {
		zs |= ir.MakeZoneSet(z)
	}
	return zs
}()

// colorSet lowers a SettableColor.
func colorSet(c *Ctx, x any) (ir.ColorSet, error) {
	v, err := c.lower(x)
	if err != nil {
		return 0, err
	}
	cs, ok := v.(ir.ColorSet)
	if !ok {
		return 0, fmt.Errorf("%s lowers to %T, want a color set", qualified(x), v)
	}
	return cs, nil
}

// alias registers key as the lowering already registered under from, looked
// up when it runs.
func alias(key, from string) {
	register(key, func(c *Ctx, n node) (any, error) { return registry[from](c, n) })
}

// effReg registers a variant lowering to one *ir.Effect.
func effReg(key string, f func(c *Ctx, n node) (*ir.Effect, error)) {
	register(key, func(c *Ctx, n node) (any, error) { return f(c, n) })
}

// effFilter registers a variant that is {kind, Filter: arg0}.
func effFilter(key string, kind ir.EffectKind) {
	effReg(key, func(c *Ctx, n node) (*ir.Effect, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: kind, Filter: f}, nil
	})
}

// effFixed registers a variant that is just {kind}.
func effFixed(key string, kind ir.EffectKind) {
	effReg(key, func(c *Ctx, n node) (*ir.Effect, error) { return &ir.Effect{Kind: kind}, nil })
}

// effRef registers a variant that is {kind, Ref: arg0}.
func effRef(key string, kind ir.EffectKind) {
	effReg(key, func(c *Ctx, n node) (*ir.Effect, error) {
		r, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: kind, Ref: r}, nil
	})
}

// effects lowers argument i as a list of effects.
func effectsArg(c *Ctx, n node, i int) ([]*ir.Effect, error) { return c.Effects(list(argN(n, i))) }

func staticRule(key string, obj func(c *Ctx, n node) (*ir.Ref, error), effs func(c *Ctx, n node) ([]*ir.Effect, error)) {
	register(key, func(c *Ctx, n node) (any, error) {
		o, err := obj(c, n)
		if err != nil {
			return nil, err
		}
		es, err := effs(c, n)
		if err != nil {
			return nil, err
		}
		return &ir.Ability{Kind: ir.AbStatic, Body: static(o, es)}, nil
	})
}

func init() {
	// Rules.
	refArg0 := func(c *Ctx, n node) (*ir.Ref, error) { return argRef(c, n, 0) }
	eachArg0 := func(c *Ctx, n node) (*ir.Ref, error) { return refAllOf(c, n, 0) }
	effs1 := func(c *Ctx, n node) ([]*ir.Effect, error) { return effectsArg(c, n, 1) }
	staticRule("Rule.PermanentLayerEffect", refArg0, effs1)
	staticRule("Rule.EachPermanentLayerEffect", eachArg0, effs1)
	staticRule("Rule.PermanentRuleEffect", refArg0, effs1)
	staticRule("Rule.PlayerEffect", refArg0, effs1)
	staticRule("Rule.EachPlayerEffect", eachArg0, effs1)
	staticRule("Rule.CastEffect",
		func(*Ctx, node) (*ir.Ref, error) { return &ir.Ref{Kind: ir.RefSelf}, nil },
		func(c *Ctx, n node) ([]*ir.Effect, error) {
			e, err := c.Effect(rawArgs(n))
			return []*ir.Effect{e}, err
		})
	register("Rule.ThisSpellEffect", func(c *Ctx, n node) (any, error) {
		es, err := c.Effects(list(rawArgs(n)))
		if err != nil {
			return nil, err
		}
		return &ir.Ability{Kind: ir.AbStatic, Zones: ir.MakeZoneSet(ir.ZoneStack),
			Body: static(&ir.Ref{Kind: ir.RefSelf}, es)}, nil
	})
	cda := func(key string, mk func(c *Ctx, n node) (*ir.Effect, error)) {
		register(key, func(c *Ctx, n node) (any, error) {
			e, err := mk(c, n)
			if err != nil {
				return nil, err
			}
			return &ir.Ability{Kind: ir.AbStatic, Zones: allZones,
				Body: static(&ir.Ref{Kind: ir.RefSelf}, []*ir.Effect{e})}, nil
		})
	}
	cda("Rule.CDA_Power", func(c *Ctx, n node) (*ir.Effect, error) {
		v, err := c.Value(rawArgs(n))
		return &ir.Effect{Kind: ir.EffCDAPower, P: v}, err
	})
	cda("Rule.CDA_Toughness", func(c *Ctx, n node) (*ir.Effect, error) {
		v, err := c.Value(rawArgs(n))
		return &ir.Effect{Kind: ir.EffCDAToughness, T: v}, err
	})
	cda("Rule.CDA_Color", func(c *Ctx, n node) (*ir.Effect, error) {
		cs, err := colorSet(c, rawArgs(n))
		return &ir.Effect{Kind: ir.EffCDAColor, Colors: cs}, err
	})

	// StaticLayerEffect: the LayerEffect shapes where they coincide.
	for _, v := range []string{"AddAbility", "AddCreatureType", "AdjustPT", "AdjustPTForEach", "SetColor", "SetCreatureType", "SetPT"} {
		alias("StaticLayerEffect."+v, "LayerEffect."+v)
	}
	effFixed("StaticLayerEffect.LosesAllAbilities", ir.EffLoseAllAbilities)
	effReg("StaticLayerEffect.SetCardtype", func(c *Ctx, n node) (*ir.Effect, error) {
		s, _ := rawArgs(n).(string)
		return &ir.Effect{Kind: ir.EffSetCardType, Words: []string{s}}, nil
	})
	effReg("StaticLayerEffect.SetName", func(c *Ctx, n node) (*ir.Effect, error) {
		s, _ := rawArgs(n).(string)
		return &ir.Effect{Kind: ir.EffSetName, Words: []string{s}}, nil
	})

	// SettableColor: a color set by value.
	register("SettableColor.SimpleColorList", func(c *Ctx, n node) (any, error) {
		var cs ir.ColorSet
		for _, x := range list(rawArgs(n)) {
			s, _ := x.(string)
			bit, ok := charsColorNames[s]
			if !ok {
				return nil, fmt.Errorf("cardc: %s: unknown color %q", c.Src(), s)
			}
			cs |= bit
		}
		return cs, nil
	})
	register("SettableColor.Colorless", func(*Ctx, node) (any, error) { return ir.ColorSet(0), nil })

	// PermanentRule.
	effFixed("PermanentRule.CantBeBlocked", ir.EffCantBeBlocked)
	effFilter("PermanentRule.CantBeBlockedByDefenders", ir.EffCantBeBlockedBy)
	effFilter("PermanentRule.CantBeTheTargetOfSpellsOrAbilities", ir.EffCantBeTargeted)
	effFixed("PermanentRule.CantBlock", ir.EffCantBlock)
	effFixed("PermanentRule.DoesntUntapDuringControllersUntap", ir.EffNoUntap)
	effFixed("PermanentRule.MustAttack", ir.EffMustAttack)
	register("SpellsAndAbilities.ControlledByAPlayer", func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Filter{Subject: ir.SubjObject, Op: ir.FPred, Pred: ir.PControlledBy,
			Ref: &ir.Ref{Kind: ir.RefAll, Filter: f}}, nil
	})

	// PlayerEffect.
	effFilter("PlayerEffect.CantCastSpells", ir.EffCantCast)
	effFixed("PlayerEffect.CantLoseTheGame", ir.EffCantLose)
	effFixed("PlayerEffect.CantWinTheGame", ir.EffCantWin)
	effReg("PlayerEffect.DecreaseSpellCost", func(c *Ctx, n node) (*ir.Effect, error) {
		f, err := c.Filter(argN(n, 0))
		if err != nil {
			return nil, err
		}
		r, err := reduction(c, list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffSpellsCostLess, Filter: f, N: constV(r)}, nil
	})
	effReg("PlayerEffect.IncreaseSpellCost", func(c *Ctx, n node) (*ir.Effect, error) {
		f, err := c.Filter(argN(n, 0))
		if err != nil {
			return nil, err
		}
		cost, err := c.Cost(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffSpellsCostMore, Filter: f, Cost: cost}, nil
	})
	effFixed("PlayerEffect.HasNoMaximumHandSize", ir.EffNoMaxHandSize)
	effFixed("PlayerEffect.Hexproof", ir.EffHexproof)
	effRef("PlayerEffect.MayCastGraveyardCard", ir.EffMayCast)
	effReg("PlayerEffect.MayCastCardFromGraveyardByPayingAddedCost", func(c *Ctx, n node) (*ir.Effect, error) {
		r, err := argRef(c, n, 0)
		if err != nil {
			return nil, err
		}
		cost, err := c.Cost(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffMayCast, Ref: r, Cost: cost}, nil
	})
	effRef("PlayerEffect.MayPlayExiledCard", ir.EffMayPlay)
	effFilter("PlayerEffect.MayCastSpellsAsThoughTheyHadFlash", ir.EffFlash)
	effFilter("PlayerEffect.MayCastSpellsFromHandWithoutPaying", ir.EffCastWithoutPaying)
	effReg("PlayerEffect.MayPlayAnAdditionalLand", func(c *Ctx, n node) (*ir.Effect, error) {
		return &ir.Effect{Kind: ir.EffExtraLand, N: refConst1()}, nil
	})

	// CastEffect.
	costEff := func(key string, kind ir.EffectKind) {
		effReg(key, func(c *Ctx, n node) (*ir.Effect, error) {
			cost, err := c.Cost(rawArgs(n))
			if err != nil {
				return nil, err
			}
			return &ir.Effect{Kind: kind, Cost: cost}, nil
		})
	}
	costEff("CastEffect.AdditionalCastingCost", ir.EffAdditionalCost)
	costEff("CastEffect.AlternateCastingCost", ir.EffAlternativeCost)
	effReg("CastEffect.AlternateCastingCostIf", func(c *Ctx, n node) (*ir.Effect, error) {
		cost, err := c.Cost(argN(n, 0))
		if err != nil {
			return nil, err
		}
		cond, err := c.Cond(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffAlternativeCost, Cost: cost, Cond: cond}, nil
	})
	effReg("CastEffect.CantBeCastIf", func(c *Ctx, n node) (*ir.Effect, error) {
		cond, err := c.Cond(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffCantCast, Cond: cond}, nil
	})
	effReg("CastEffect.ReduceCastingCostForEach", func(c *Ctx, n node) (*ir.Effect, error) {
		r, err := reduction(c, list(argN(n, 0)))
		if err != nil {
			return nil, err
		}
		v, err := c.Value(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffSelfCostLess, N: &ir.Value{Kind: ir.VMul, Args: []*ir.Value{constV(r), v}}}, nil
	})
	effReg("CastEffect.ReduceCastingCostIf", func(c *Ctx, n node) (*ir.Effect, error) {
		r, err := reduction(c, list(argN(n, 0)))
		if err != nil {
			return nil, err
		}
		cond, err := c.Cond(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffSelfCostLess, N: constV(r), Cond: cond}, nil
	})
	effReg("CastEffect.ReduceCastingCostIfItTargetsAPermanent", func(c *Ctx, n node) (*ir.Effect, error) {
		r, err := reduction(c, list(argN(n, 0)))
		if err != nil {
			return nil, err
		}
		f, err := c.Filter(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffSelfCostLess, N: constV(r), Filter: f}, nil
	})
	effReg("CastEffect.ReduceCastingCostX", func(c *Ctx, n node) (*ir.Effect, error) {
		syms := list(argN(n, 0))
		if len(syms) != 1 {
			return nil, &RefusalError{Key: "CastEffect.ReduceCastingCostX:shape", Path: c.Src()}
		}
		v, err := c.lower(syms[0])
		if err != nil {
			return nil, err
		}
		if s, ok := v.(*ir.ManaSym); !ok || s.Kind != ir.ManaX {
			return nil, &RefusalError{Key: "CastEffect.ReduceCastingCostX:shape", Path: c.Src()}
		}
		nv, err := c.Value(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffSelfCostLess, N: nv}, nil
	})

	effFixed("SpellEffect.CantBeCountered", ir.EffCantBeCountered)
}
