package mtgish

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// Action variants A-L, the satellite enums they use (ManaProduce,
// ChoosableColor, Expiration, FutureTrigger, LayerEffect,
// GraveyardCardEffect, ModX, PT, ExileFlag, LookAtTopOfLibraryAction), and
// the TokenColorList/TokenSubtypes/TokenFlag/TokenCopyEffect leaves.

const allFive = ir.ColorW | ir.ColorU | ir.ColorB | ir.ColorR | ir.ColorG

func constV(n int32) *ir.Value { return &ir.Value{Kind: ir.VConst, N: n} }

// manaList is what ManaProduce lowers to.
type manaList struct {
	syms []ir.ManaSym
	// eachColorAmong is set instead of syms for "one mana of each color
	// among" those objects.
	eachColorAmong *ir.Filter
}

// mana lowers a ManaProduce to its symbols.
func mana(c *Ctx, x any) ([]ir.ManaSym, error) {
	v, err := c.lower(x)
	if err != nil {
		return nil, err
	}
	m, ok := v.(*manaList)
	if !ok {
		return nil, fmt.Errorf("%s lowers to %T, want mana", qualified(x), v)
	}
	if m.eachColorAmong != nil {
		return nil, &RefusalError{Key: "ManaProduce.EachManaColorAmongPermanents:combined", Path: c.Src()}
	}
	return m.syms, nil
}

// ptPair is what PT lowers to.
type ptPair struct {
	p, t   *ir.Value
	manual bool
}

// exileFlag is what ExileFlag lowers to.
type exileFlag struct {
	word   string
	dur    ir.Duration
	hasDur bool
}

// argRef lowers argument i as a reference.
func argRef(c *Ctx, n node, i int) (*ir.Ref, error) { return c.Ref(argN(n, i)) }

// refAllOf is every object matching the filter argument i.
func refAllOf(c *Ctx, n node, i int) (*ir.Ref, error) {
	f, err := c.Filter(argN(n, i))
	if err != nil {
		return nil, err
	}
	return &ir.Ref{Kind: ir.RefAll, Filter: f}, nil
}

// stepObj registers an action that is one verb on a reference argument.
func stepObj(key string, verb ir.Verb) {
	register(key, func(c *Ctx, n node) (any, error) {
		r, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: verb, Obj: r}, nil
	})
}

// stepAllObj registers an action that is one verb on every object matching
// its filter argument.
func stepAllObj(key string, verb ir.Verb) {
	register(key, func(c *Ctx, n node) (any, error) {
		r, err := refAllOf(c, n, 0)
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: verb, Obj: r}, nil
	})
}

// stepN registers an action that is one verb with a count argument.
func stepN(key string, verb ir.Verb) {
	register(key, func(c *Ctx, n node) (any, error) {
		v, err := c.Value(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: verb, N: v}, nil
	})
}

func stepFixed(key string, mk func() *ir.Step) {
	register(key, func(c *Ctx, n node) (any, error) { return mk(), nil })
}

// applyEffect registers CreateXLayerEffectUntil style actions:
// {VerbApplyEffect, Obj: arg0, Effects: arg1, Dur: arg2 or dur}.
func applyEffect(key string, obj func(c *Ctx, n node) (*ir.Ref, error), effs func(c *Ctx, x any) ([]*ir.Effect, error), hasExp bool, dur ir.Duration) {
	register(key, func(c *Ctx, n node) (any, error) {
		o, err := obj(c, n)
		if err != nil {
			return nil, err
		}
		es, err := effs(c, argN(n, 1))
		if err != nil {
			return nil, err
		}
		d := dur
		if hasExp {
			if d, err = c.Duration(argN(n, 2)); err != nil {
				return nil, err
			}
		}
		return &ir.Step{Verb: ir.VerbApplyEffect, Obj: o, Effects: es, Dur: d}, nil
	})
}

func effectList(c *Ctx, x any) ([]*ir.Effect, error) { return c.Effects(list(x)) }

func init() {
	register("Action.AddMana", func(c *Ctx, n node) (any, error) {
		v, err := c.lower(rawArgs(n))
		if err != nil {
			return nil, err
		}
		ml, ok := v.(*manaList)
		if !ok {
			return nil, fmt.Errorf("%s lowers to %T, want mana", qualified(rawArgs(n)), v)
		}
		if ml.eachColorAmong != nil {
			return &ir.Step{Verb: ir.VerbAddMana, Word: "each-color-among", Filter: ml.eachColorAmong}, nil
		}
		return &ir.Step{Verb: ir.VerbAddMana, Mana: ml.syms}, nil
	})
	register("Action.AddManaRepeated", func(c *Ctx, n node) (any, error) {
		m, err := mana(c, argN(n, 0))
		if err != nil {
			return nil, err
		}
		v, err := c.Value(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbAddMana, Mana: m, N: v}, nil
	})
	register("Action.AttachPermanentToPermanent", func(c *Ctx, n node) (any, error) {
		a, err := argRef(c, n, 0)
		if err != nil {
			return nil, err
		}
		b, err := argRef(c, n, 1)
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbAttach, Obj: a, Other: b}, nil
	})
	register("Action.ChooseAColor", func(c *Ctx, n node) (any, error) {
		v, err := c.lower(rawArgs(n))
		if err != nil {
			return nil, err
		}
		cs, ok := v.(ir.ColorSet)
		if !ok {
			return nil, fmt.Errorf("%s lowers to %T, want a color set", qualified(rawArgs(n)), v)
		}
		return &ir.Step{Verb: ir.VerbChooseColor, Colors: cs}, nil
	})
	register("Action.ChooseAnExiledCard", func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbChoose, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: f, N: refConst1()}}, nil
	})
	stepObj("Action.CopySpellAndMayChooseNewTargets", ir.VerbCopySpell)
	register("Action.CopySpellForEachAndMayChooseNewTargets", func(c *Ctx, n node) (any, error) {
		r, err := argRef(c, n, 0)
		if err != nil {
			return nil, err
		}
		v, err := c.Value(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbCopySpell, Obj: r, N: v}, nil
	})
	stepObj("Action.CounterSpell", ir.VerbCounter)

	applyEffect("Action.CreateEachPermanentLayerEffectUntil",
		func(c *Ctx, n node) (*ir.Ref, error) { return refAllOf(c, n, 0) }, effectList, true, 0)
	applyEffect("Action.CreateGraveyardCardEffectUntil", func(c *Ctx, n node) (*ir.Ref, error) { return argRef(c, n, 0) }, effectList, true, 0)
	applyEffect("Action.CreatePermanentLayerEffect", func(c *Ctx, n node) (*ir.Ref, error) { return argRef(c, n, 0) }, effectList, false, ir.DurPermanent)
	applyEffect("Action.CreatePermanentLayerEffectUntil", func(c *Ctx, n node) (*ir.Ref, error) { return argRef(c, n, 0) }, effectList, true, 0)
	applyEffect("Action.CreatePermanentRuleEffectUntil", func(c *Ctx, n node) (*ir.Ref, error) { return argRef(c, n, 0) }, effectList, true, 0)
	applyEffect("Action.CreatePlayerEffectUntil", func(c *Ctx, n node) (*ir.Ref, error) { return argRef(c, n, 0) }, effectList, true, 0)

	register("Action.CreateFutureTrigger", func(c *Ctx, n node) (any, error) {
		tr, err := c.Trigger(argN(n, 0))
		if err != nil {
			return nil, err
		}
		body, err := c.NestedBody(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbDelayedTrigger, Abilities: []*ir.Ability{{Kind: ir.AbTriggered, Trigger: tr, Body: body}}}, nil
	})
	for _, k := range []string{"Action.CreatePreventDamageUntil", "Action.CreateReplaceWouldPutIntoGraveyardUntil"} {
		register(k, func(c *Ctx, n node) (any, error) {
			tr, err := c.Trigger(argN(n, 0))
			if err != nil {
				return nil, err
			}
			body, err := c.StepList(list(argN(n, 1)))
			if err != nil {
				return nil, err
			}
			d, err := c.Duration(argN(n, 2))
			if err != nil {
				return nil, err
			}
			return &ir.Step{Verb: ir.VerbCreateReplacement, Dur: d,
				Abilities: []*ir.Ability{{Kind: ir.AbReplacement, Trigger: tr, Body: body}}}, nil
		})
	}

	register("Action.CreateTokens", func(c *Ctx, n node) (any, error) {
		var steps []*ir.Step
		for _, t := range list(argN(n, 0)) {
			ss, err := c.Steps(t)
			if err != nil {
				return nil, err
			}
			steps = append(steps, ss...)
		}
		en, err := c.Enters(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		if en != nil {
			for _, s := range steps {
				e := *en
				s.Enter = &e
			}
		}
		return &ir.Program{Steps: steps}, nil
	})

	stepAllObj("Action.DestroyEachPermanent", ir.VerbDestroy)
	stepObj("Action.DestroyPermanent", ir.VerbDestroy)
	stepFixed("Action.DiscardACard", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbDiscard, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: anyCardIn(ir.ZoneHand), N: refConst1()}}
	})
	stepAllObj("Action.DiscardCards", ir.VerbDiscard)
	register("Action.DiscardNumberCards", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbDiscard, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: anyCardIn(ir.ZoneHand), N: v}}, nil
	})
	register("Action.DoubleCreaturesPowerUntilEndOfTurn", func(c *Ctx, n node) (any, error) {
		r, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbApplyEffect, Obj: r, Dur: ir.DurEndOfTurn,
			Effects: []*ir.Effect{{Kind: ir.EffAdjustPT, P: &ir.Value{Kind: ir.VPower, Ref: r}, T: constV(0)}}}, nil
	})
	stepFixed("Action.DrawACard", func() *ir.Step { return &ir.Step{Verb: ir.VerbDraw, N: refConst1()} })
	stepN("Action.DrawNumberCards", ir.VerbDraw)
	register("Action.EachPlayerAction", func(c *Ctx, n node) (any, error) {
		who, err := refAllOf(c, n, 0)
		if err != nil {
			return nil, err
		}
		inner, err := c.Steps(argN(n, 1))
		if err != nil {
			return nil, err
		}
		for _, s := range inner {
			if s.Who == nil {
				s.Who = &ir.Ref{Kind: ir.RefIterPlayer}
			}
		}
		return &ir.Step{Verb: ir.VerbForEachPlayer, Obj: who, Then: &ir.Program{Steps: inner}}, nil
	})
	stepFixed("Action.EndTheTurn", func() *ir.Step { return &ir.Step{Verb: ir.VerbEndTurn} })

	register("Action.Exile", func(c *Ctx, n node) (any, error) {
		var word string
		var dur ir.Duration
		for _, f := range list(argN(n, 1)) {
			v, err := c.lower(f)
			if err != nil {
				return nil, err
			}
			ef, ok := v.(*exileFlag)
			if !ok {
				return nil, fmt.Errorf("%s lowers to %T, want an exile flag", qualified(f), v)
			}
			if ef.word != "" {
				if word != "" {
					return nil, fmt.Errorf("%s: conflicting exile flags", qualified(f))
				}
				word = ef.word
			}
			if ef.hasDur {
				if dur != ir.DurNone {
					return nil, fmt.Errorf("%s: conflicting exile flags", qualified(f))
				}
				dur = ef.dur
			}
		}
		p := &ir.Program{}
		for _, x := range list(argN(n, 0)) {
			r, err := c.Ref(x)
			if err != nil {
				return nil, err
			}
			p.Steps = append(p.Steps, &ir.Step{Verb: ir.VerbExile, Obj: r, Word: word, Dur: dur})
		}
		return p, nil
	})
	register("ExileFlag.ExilesWithACounter", func(c *Ctx, n node) (any, error) {
		v, err := c.lower(rawArgs(n))
		if err != nil {
			return nil, err
		}
		w, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s lowers to %T, want a counter word", qualified(rawArgs(n)), v)
		}
		return &exileFlag{word: w}, nil
	})
	register("ExileFlag.Limited", func(c *Ctx, n node) (any, error) {
		d, err := c.Duration(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &exileFlag{dur: d, hasDur: true}, nil
	})
	register("Action.Fight", func(c *Ctx, n node) (any, error) {
		a, err := argRef(c, n, 0)
		if err != nil {
			return nil, err
		}
		b, err := argRef(c, n, 1)
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbFight, Source: a, Obj: b}, nil
	})
	register("Action.GainControlOfPermanentUntil", func(c *Ctx, n node) (any, error) {
		r, err := argRef(c, n, 0)
		if err != nil {
			return nil, err
		}
		d, err := c.Duration(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbGainControl, Obj: r, Dur: d}, nil
	})
	stepN("Action.GainLife", ir.VerbGainLife)
	register("Action.GainLifeForEach", func(c *Ctx, n node) (any, error) {
		a, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		b, err := c.Value(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbGainLife, N: &ir.Value{Kind: ir.VMul, Args: []*ir.Value{a, b}}}, nil
	})
	register("Action.GetAnEmblem", func(c *Ctx, n node) (any, error) {
		abs, err := c.NestedAbilities(list(rawArgs(n)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbEmblem, Abilities: abs}, nil
	})
	register("Action.If", func(c *Ctx, n node) (any, error) {
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
	register("Action.IfElse", func(c *Ctx, n node) (any, error) {
		cond, err := c.Cond(argN(n, 0))
		if err != nil {
			return nil, err
		}
		then, err := c.StepList(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		els, err := c.StepList(list(argN(n, 2)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbIf, Cond: cond, Then: then, Else: els}, nil
	})
	stepObj("Action.LookAtPlayersHand", ir.VerbLookAtHand)
	register("Action.LookAtTheTopCardOfPlayersLibrary", func(c *Ctx, n node) (any, error) {
		r, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbLook, Obj: &ir.Ref{Kind: ir.RefTopOfLibrary, Of: r, N: refConst1()}}, nil
	})
	register("Action.LookAtTheTopNumberCardsOfLibrary", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		subs, err := c.StepList(list(argN(n, 1)))
		if err != nil {
			return nil, err
		}
		return &ir.Step{Verb: ir.VerbLook, Obj: &ir.Ref{Kind: ir.RefTopOfLibrary, N: v}, Then: subs}, nil
	})
	stepFixed("Action.LookAtTopOfLibrary", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbLook, Obj: &ir.Ref{Kind: ir.RefTopOfLibrary, N: refConst1()}}
	})
	stepN("Action.LoseLife", ir.VerbLoseLife)

	// ManaProduce.
	for v, cs := range map[string]ir.ColorSet{"W": ir.ColorW, "U": ir.ColorU, "B": ir.ColorB, "R": ir.ColorR, "G": ir.ColorG} {
		sym := ir.ManaSym{Kind: ir.ManaColored, Colors: cs}
		register("ManaProduce.ManaProduce"+v, func(*Ctx, node) (any, error) { return &manaList{syms: []ir.ManaSym{sym}}, nil })
	}
	register("ManaProduce.ManaProduceC", func(*Ctx, node) (any, error) {
		return &manaList{syms: []ir.ManaSym{{Kind: ir.ManaColorless}}}, nil
	})
	register("ManaProduce.AnyManaColor", func(*Ctx, node) (any, error) {
		return &manaList{syms: []ir.ManaSym{{Kind: ir.ManaColored, Colors: allFive}}}, nil
	})
	register("ManaProduce.EachManaColorAmongPermanents", func(c *Ctx, n node) (any, error) {
		f, err := c.Filter(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &manaList{eachColorAmong: f}, nil
	})
	register("ManaProduce.And", func(c *Ctx, n node) (any, error) {
		out := &manaList{}
		for _, x := range list(rawArgs(n)) {
			m, err := mana(c, x)
			if err != nil {
				return nil, err
			}
			out.syms = append(out.syms, m...)
		}
		return out, nil
	})
	register("ManaProduce.Or", func(c *Ctx, n node) (any, error) {
		var cs ir.ColorSet
		for _, x := range list(rawArgs(n)) {
			m, err := mana(c, x)
			if err != nil {
				return nil, err
			}
			if len(m) != 1 || m[0].Kind != ir.ManaColored {
				return nil, &RefusalError{Key: "ManaProduce.Or:shape", Path: c.Src()}
			}
			cs |= m[0].Colors
		}
		return &manaList{syms: []ir.ManaSym{{Kind: ir.ManaColored, Colors: cs}}}, nil
	})

	register("ChoosableColor.AnyColor", func(*Ctx, node) (any, error) { return allFive, nil })

	// Expiration.
	dur := func(d ir.Duration) lowerFunc {
		return func(*Ctx, node) (any, error) { return &d, nil }
	}
	register("Expiration.UntilEndOfTurn", dur(ir.DurEndOfTurn))
	playerExp := func(key string, d ir.Duration) {
		register(key, func(c *Ctx, n node) (any, error) {
			r, err := c.Ref(rawArgs(n))
			if err != nil {
				return nil, err
			}
			if r.Kind != ir.RefYou || r.Of != nil {
				return nil, &RefusalError{Key: key + ":player", Path: c.Src()}
			}
			return &d, nil
		})
	}
	playerExp("Expiration.UntilPlayersNextTurn", ir.DurUntilYourNextTurn)
	playerExp("Expiration.UntilTheEndOfPlayersNextTurn", ir.DurUntilEndOfYourNextTurn)
	register("Expiration.UntilPermanentLeavesBattlefield", func(c *Ctx, n node) (any, error) {
		r, err := c.Ref(rawArgs(n))
		if err != nil {
			return nil, err
		}
		if r.Kind != ir.RefSelf {
			return nil, &RefusalError{Key: "Expiration.UntilPermanentLeavesBattlefield:permanent", Path: c.Src()}
		}
		d := ir.DurWhileSourceOnBattlefield
		return &d, nil
	})

	register("FutureTrigger.AtTheBeginningOfTheNextEndStep", func(*Ctx, node) (any, error) {
		return &ir.Trigger{Event: ir.EvNextEndStep}, nil
	})

	// LayerEffect.
	layer := func(variant string, f func(c *Ctx, n node) (*ir.Effect, error)) {
		register("LayerEffect."+variant, func(c *Ctx, n node) (any, error) { return f(c, n) })
	}
	words := func(x any) []string {
		var out []string
		for _, e := range list(x) {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	word := func(x any) []string {
		s, _ := x.(string)
		return []string{s}
	}
	k := func(x any) *ir.Value {
		v, _ := intOf(x)
		return constV(int32(v))
	}
	layer("AddAbility", func(c *Ctx, n node) (*ir.Effect, error) {
		abs, err := c.NestedAbilities(list(rawArgs(n)))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffAddAbility, Abilities: abs}, nil
	})
	layer("AddCardtype", func(c *Ctx, n node) (*ir.Effect, error) {
		return &ir.Effect{Kind: ir.EffAddCardType, Words: word(rawArgs(n))}, nil
	})
	layer("AddCardtypes", func(c *Ctx, n node) (*ir.Effect, error) {
		return &ir.Effect{Kind: ir.EffAddCardType, Words: words(rawArgs(n))}, nil
	})
	layer("AddCreatureType", func(c *Ctx, n node) (*ir.Effect, error) {
		return &ir.Effect{Kind: ir.EffAddSubtype, Words: word(rawArgs(n))}, nil
	})
	layer("AdjustPT", func(c *Ctx, n node) (*ir.Effect, error) {
		return &ir.Effect{Kind: ir.EffAdjustPT, P: k(argN(n, 0)), T: k(argN(n, 1))}, nil
	})
	layer("AdjustPTForEach", func(c *Ctx, n node) (*ir.Effect, error) {
		v, err := c.Value(argN(n, 2))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffAdjustPT,
			P: &ir.Value{Kind: ir.VMul, Args: []*ir.Value{k(argN(n, 0)), v}},
			T: &ir.Value{Kind: ir.VMul, Args: []*ir.Value{k(argN(n, 1)), v}}}, nil
	})
	layer("AdjustPTX", func(c *Ctx, n node) (*ir.Effect, error) {
		bad := &RefusalError{Key: "LayerEffect.AdjustPTX:mod", Path: c.Src()}
		for i := 0; i < 2; i++ {
			v, err := c.lower(argN(n, i))
			if _, isRef := err.(*RefusalError); isRef {
				return nil, bad
			} else if err != nil {
				return nil, err
			}
			if x, ok := v.(*int32); !ok || *x != 1 {
				return nil, bad
			}
		}
		v, err := c.Value(argN(n, 2))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffAdjustPT, P: v, T: v}, nil
	})
	layer("HasAllCreatureTypes", func(c *Ctx, n node) (*ir.Effect, error) {
		return &ir.Effect{Kind: ir.EffAllCreatureTypes}, nil
	})
	layer("SetColor", func(c *Ctx, n node) (*ir.Effect, error) {
		v, err := c.lower(rawArgs(n))
		if err != nil {
			return nil, err
		}
		cs, ok := v.(ir.ColorSet)
		if !ok {
			return nil, fmt.Errorf("%s lowers to %T, want a color set", qualified(rawArgs(n)), v)
		}
		return &ir.Effect{Kind: ir.EffSetColor, Colors: cs}, nil
	})
	layer("SetCreatureType", func(c *Ctx, n node) (*ir.Effect, error) {
		return &ir.Effect{Kind: ir.EffSetSubtype, Words: word(rawArgs(n))}, nil
	})
	layer("SetCreatureTypes", func(c *Ctx, n node) (*ir.Effect, error) {
		return &ir.Effect{Kind: ir.EffSetSubtype, Words: words(rawArgs(n))}, nil
	})
	layer("SetPT", func(c *Ctx, n node) (*ir.Effect, error) {
		pt, err := lowerPT(c, rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffSetPT, P: pt.p, T: pt.t}, nil
	})
	layer("SetPowerAndToughnessBoth", func(c *Ctx, n node) (*ir.Effect, error) {
		v, err := c.Value(rawArgs(n))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffSetPT, P: v, T: v}, nil
	})

	register("GraveyardCardEffect.AddAbility", func(c *Ctx, n node) (any, error) {
		abs, err := c.NestedAbilities(list(rawArgs(n)))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffAddAbility, Abilities: abs}, nil
	})

	register("ModX.PlusX", func(*Ctx, node) (any, error) { one := int32(1); return &one, nil })

	register("PT.PT", func(c *Ctx, n node) (any, error) {
		return &ptPair{p: k(argN(n, 0)), t: k(argN(n, 1))}, nil
	})
	register("PT.ManualPT", func(c *Ctx, n node) (any, error) {
		p, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		t, err := c.Value(argN(n, 1))
		if err != nil {
			return nil, err
		}
		return &ptPair{p: p, t: t, manual: true}, nil
	})

	registerLookAt()
}

// lowerPT lowers a PT to its pair.
func lowerPT(c *Ctx, x any) (*ptPair, error) {
	v, err := c.lower(x)
	if err != nil {
		return nil, err
	}
	pt, ok := v.(*ptPair)
	if !ok {
		return nil, fmt.Errorf("%s lowers to %T, want a PT", qualified(x), v)
	}
	return pt, nil
}

// registerLookAt registers LookAtTopOfLibraryAction. Inside a VerbLook step's
// Then, a RefChoose chooses among the looked-at cards, and a nil Obj means
// "the remaining looked-at cards".
func registerLookAt() {
	reveal := func(to ir.Zone) lowerFunc {
		return func(c *Ctx, n node) (any, error) {
			f, err := c.Filter(argN(n, 0))
			if err != nil {
				return nil, err
			}
			st := &ir.Step{Verb: ir.VerbMove, Optional: true, Obj: &ir.Ref{Kind: ir.RefChoose, Filter: f, N: refConst1()}, To: to, Word: "reveal"}
			if to == ir.ZoneBattlefield {
				en, err := c.Enters(list(argN(n, 1)))
				if err != nil {
					return nil, err
				}
				st.Enter = en
			}
			return st, nil
		}
	}
	register("LookAtTopOfLibraryAction.MayRevealACardOfTypeAndPutIntoHand", reveal(ir.ZoneHand))
	register("LookAtTopOfLibraryAction.MayRevealAndPutACardOfTypeOntoTheBattlefield", reveal(ir.ZoneBattlefield))
	stepFixed("LookAtTopOfLibraryAction.MayPutAGenericCardOnTopOfLibrary", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbMove, Optional: true, Obj: &ir.Ref{Kind: ir.RefChoose, N: refConst1()}, To: ir.ZoneLibrary, Word: "top"}
	})
	stepFixed("LookAtTopOfLibraryAction.PutTheRemainingCardsIntoGraveyard", func() *ir.Step {
		return &ir.Step{Verb: ir.VerbMove, To: ir.ZoneGraveyard}
	})
	remaining := func(variant, word string) {
		stepFixed("LookAtTopOfLibraryAction."+variant, func() *ir.Step {
			return &ir.Step{Verb: ir.VerbMove, To: ir.ZoneLibrary, Word: word}
		})
	}
	remaining("PutTheRemainingCardsOnTheBottomOfLibraryInARandomOrder", "bottom-random")
	remaining("PutTheRemainingCardsOnTheBottomOfLibraryInAnyOrder", "bottom-any-order")
	remaining("PutTheRemainingCardsOnTopOfLibraryInAnyOrder", "top-any-order")
}
