package mtgish

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// Rule variants that make spell, activated and triggered abilities and
// keywords with arguments, plus the ActivateModifier, Protectable,
// ProtectableColor and DamageSources satellites.

// abMod is what an ActivateModifier lowers to: one change to an ability.
type abMod struct{ apply func(*ir.Ability) }

// andCond is first alone, or {CAll, [first, second]} when second is set.
func andCond(first, second *ir.Cond) *ir.Cond {
	if second == nil {
		return first
	}
	if first == nil {
		return second
	}
	return &ir.Cond{Kind: ir.CAll, Args: []*ir.Cond{first, second}}
}

func ruleSimple(key string, build func(c *Ctx, n node) (*ir.Ability, error)) {
	register(key, func(c *Ctx, n node) (any, error) { return build(c, n) })
}

// keywordRule registers a keyword with a cost and/or filter argument.
func keywordRule(variant, name string, cost, filter int) {
	register("Rule."+variant, func(c *Ctx, n node) (any, error) {
		kw := &ir.Keyword{Name: name}
		var err error
		if cost >= 0 {
			if kw.Cost, err = c.Cost(argN(n, cost)); err != nil {
				return nil, err
			}
		}
		if filter >= 0 {
			if kw.Filter, err = c.Filter(argN(n, filter)); err != nil {
				return nil, err
			}
		}
		src := c.Src()
		kw.Src = src
		return &ir.Ability{Kind: ir.AbKeyword, Keyword: kw, Src: src}, nil
	})
}

// keywordMana registers a keyword whose argument is a list of mana symbols.
func keywordMana(variant, name string) {
	register("Rule."+variant, func(c *Ctx, n node) (any, error) {
		var syms []ir.ManaSym
		for _, s := range list(rawArgs(n)) {
			v, err := c.lower(s)
			if err != nil {
				return nil, err
			}
			ms, ok := v.(*ir.ManaSym)
			if !ok {
				return nil, fmt.Errorf("%s lowers to %T, want *ir.ManaSym", qualified(s), v)
			}
			syms = append(syms, *ms)
		}
		src := c.Src()
		return &ir.Ability{Kind: ir.AbKeyword, Src: src,
			Keyword: &ir.Keyword{Name: name, Cost: &ir.Cost{Mana: syms, HasMana: true}, Src: src}}, nil
	})
}

func (c *Ctx) abMod(x any) (*abMod, error) { return lowerAs[abMod](c, x) }

func modFlag(variant string, f func(*ir.Ability)) {
	register("ActivateModifier."+variant, func(c *Ctx, n node) (any, error) { return &abMod{f}, nil })
}

// wrapZone registers a rule that is its inner rule restricted to one zone.
func wrapZone(variant string, z ir.Zone) {
	ruleSimple("Rule."+variant, func(c *Ctx, n node) (*ir.Ability, error) {
		ab, err := c.Ability(rawArgs(n))
		if err != nil {
			return nil, err
		}
		ab.Zones = ir.MakeZoneSet(z)
		return ab, nil
	})
}

func init() {
	ruleSimple("Rule.SpellActions", func(c *Ctx, n node) (*ir.Ability, error) {
		body, err := c.Program(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Ability{Kind: ir.AbSpell, Body: body}, nil
	})
	ruleSimple("Rule.SpellActions_Kicker", func(c *Ctx, n node) (*ir.Ability, error) {
		cost, err := c.Cost(argN(n, 0))
		if err != nil {
			return nil, err
		}
		kicked, err := c.Program(argN(n, 1))
		if err != nil {
			return nil, err
		}
		unkicked, err := c.Program(argN(n, 2))
		if err != nil {
			return nil, err
		}
		return &ir.Ability{Kind: ir.AbSpell, Keyword: &ir.Keyword{Name: "Kicker", Cost: cost},
			Body: &ir.Program{Steps: []*ir.Step{{Verb: ir.VerbIf, Cond: &ir.Cond{Kind: ir.CKicked}, Then: kicked, Else: unkicked}}}}, nil
	})
	activated := func(c *Ctx, n node) (*ir.Ability, error) {
		cost, err := c.Cost(argN(n, 0))
		if err != nil {
			return nil, err
		}
		body, err := c.Program(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Ability{Kind: ir.AbActivated, Cost: cost, Body: body}, nil
	}
	ruleSimple("Rule.Activated", activated)
	ruleSimple("Rule.ActivatedWithModifiers", func(c *Ctx, n node) (*ir.Ability, error) {
		ab, err := activated(c, n)
		if err != nil {
			return nil, err
		}
		m, err := c.abMod(argN(n, 2))
		if err != nil {
			return nil, err
		}
		m.apply(ab)
		return ab, nil
	})
	triggered := func(once bool, hasCond bool) func(c *Ctx, n node) (*ir.Ability, error) {
		return func(c *Ctx, n node) (*ir.Ability, error) {
			trig, err := c.Trigger(argN(n, 0))
			if err != nil {
				return nil, err
			}
			ab := &ir.Ability{Kind: ir.AbTriggered, Trigger: trig}
			bodyIdx := 1
			if hasCond {
				if ab.Cond, err = c.Cond(argN(n, 1)); err != nil {
					return nil, err
				}
				bodyIdx = 2
			}
			if ab.Body, err = c.Program(argN(n, bodyIdx)); err != nil {
				return nil, err
			}
			if once {
				trig.OncePerTurn = true
			}
			return ab, nil
		}
	}
	ruleSimple("Rule.TriggerA", triggered(false, false))
	ruleSimple("Rule.TriggerI", triggered(false, true))
	ruleSimple("Rule.TriggerOnceEachTurn", triggered(true, false))
	ruleSimple("Rule.If", func(c *Ctx, n node) (*ir.Ability, error) {
		cond, err := c.Cond(argN(n, 0))
		if err != nil {
			return nil, err
		}
		rules := list(argN(n, 1))
		if len(rules) != 1 {
			return nil, &RefusalError{Key: "Rule.If:multi", Path: c.Src()}
		}
		ab, err := c.Ability(rules[0])
		if err != nil {
			return nil, err
		}
		ab.Cond = andCond(cond, ab.Cond)
		return ab, nil
	})
	wrapZone("FromGraveyard", ir.ZoneGraveyard)
	wrapZone("FromStack", ir.ZoneStack)

	modFlag("ActivateOnlyAsASorcery", func(a *ir.Ability) { a.SorceryOnly = true })
	modFlag("ActivateOnlyAsAnInstant", func(a *ir.Ability) { a.InstantOnly = true })
	modFlag("ActivateOnlyOnce", func(a *ir.Ability) { a.Once = true })
	register("ActivateModifier.ActivateOnlyIf", func(c *Ctx, n node) (any, error) {
		cond, err := c.Cond(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &abMod{func(a *ir.Ability) { a.Cond = andCond(a.Cond, cond) }}, nil
	})

	keywordRule("Ward", "Ward", 0, -1)
	keywordRule("Equip", "Equip", 1, 0)
	keywordRule("Flashback", "Flashback", 0, -1)
	keywordRule("Kicker", "Kicker", 0, -1)
	keywordRule("Miracle", "Miracle", 0, -1)
	keywordMana("MiracleX", "Miracle")
	keywordMana("Surge", "Surge")
	keywordRule("Affinity", "Affinity", -1, 0)
	keywordRule("EnchantPermanent", "Enchant", -1, 0)
	keywordRule("Protection", "Protection", -1, 0)
	keywordRule("HexproofFrom", "Hexproof", -1, 0)

	register("Protectable.FromColor", func(c *Ctx, n node) (any, error) {
		v, err := c.lower(rawArgs(n))
		if err != nil {
			return nil, err
		}
		cs, ok := v.(ir.ColorSet)
		if !ok {
			return nil, fmt.Errorf("%s lowers to %T, want ir.ColorSet", qualified(rawArgs(n)), v)
		}
		return &ir.Filter{Subject: ir.SubjObject, Op: ir.FPred, Pred: ir.PColor, Colors: cs}, nil
	})
	register("Protectable.FromEverything", func(c *Ctx, n node) (any, error) {
		return &ir.Filter{Subject: ir.SubjObject, Op: ir.FPred, Pred: ir.PAny}, nil
	})
	register("Protectable.FromTypes", func(c *Ctx, n node) (any, error) { return c.Filter(rawArgs(n)) })
	register("ProtectableColor.Colors", func(c *Ctx, n node) (any, error) {
		var out ir.ColorSet
		for _, x := range list(rawArgs(n)) {
			cs, err := filtColor(c, x)
			if err != nil {
				return nil, err
			}
			out |= cs
		}
		return out, nil
	})
	register("DamageSources.ControlledByAPlayer", func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Filter{Subject: ir.SubjObject, Op: ir.FPred, Pred: ir.PControlledBy,
			Ref: &ir.Ref{Kind: ir.RefAll, Filter: f}}, nil
	})
}

func init() {
	// "If this card is in your opening hand, you may begin the game with it on
	// the battlefield": a keyword, the leyline ability. Any other opening-hand
	// action stays refused.
	register("Rule.IfCardIsInOpeningHand", func(c *Ctx, n node) (any, error) {
		key := "Rule.IfCardIsInOpeningHand"
		acts := list(rawArgs(n))
		if len(acts) != 1 || variantOf(acts[0]) != "MayAction" {
			return nil, &RefusalError{Key: key, Path: c.Src()}
		}
		inner := rawArgs(acts[0])
		flags := list(argN(inner, 1))
		if variantOf(inner) != "BeginGameWithCardOnBattlefield" || variantOf(argN(inner, 0)) != "ThisPregameCard" ||
			len(flags) != 1 || variantOf(flags[0]) != "EntersNormally" {
			return nil, &RefusalError{Key: key, Path: c.Src()}
		}
		src := c.Src()
		return &ir.Ability{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: "MayEffectFromOpeningHand", Src: src}, Src: src}, nil
	})
}

func init() {
	// "A deck can have any number of cards named ...": a keyword of that name.
	register("Rule.DeckConstruction", func(c *Ctx, n node) (any, error) {
		if variantOf(rawArgs(n)) != "CanHaveAnyNumberOfThisCard" {
			return nil, &RefusalError{Key: "Rule.DeckConstruction", Path: c.Src()}
		}
		src := c.Src()
		return &ir.Ability{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: anyNumberKeyword, Src: src}, Src: src}, nil
	})
}

// anyNumberKeyword is the keyword of a card a deck may hold any number of.
const anyNumberKeyword = "A deck can have any number of cards named CARDNAME."
