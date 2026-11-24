package mtgish

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// CreatableToken and its leaves (TokenColorList, TokenSubtypes, TokenFlag,
// TokenCopyEffects, TokenCopyEffect). Each CreatableToken lowers to one
// *ir.Step with N the count.

func strList(x any) []string {
	var out []string
	for _, e := range list(x) {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// tokenFace builds the token face of a TokenWithPT-shaped node whose
// arguments start at index i: pt, colors, supertypes, types, subtypes, rules.
func tokenFace(c *Ctx, n node, key string, i int) (*ir.Face, error) {
	pt, err := lowerPT(c, argN(n, i))
	if err != nil {
		return nil, err
	}
	if pt.manual {
		return nil, &RefusalError{Key: key + ":manual-pt", Path: c.Src()}
	}
	cv, err := c.lower(argN(n, i+1))
	if err != nil {
		return nil, err
	}
	colors, ok := cv.(ir.ColorSet)
	if !ok {
		return nil, fmt.Errorf("%s lowers to %T, want a color set", qualified(argN(n, i+1)), cv)
	}
	sv, err := c.lower(argN(n, i+4))
	if err != nil {
		return nil, err
	}
	subs, ok := sv.([]string)
	if !ok {
		return nil, fmt.Errorf("%s lowers to %T, want subtypes", qualified(argN(n, i+4)), sv)
	}
	abs, err := c.NestedAbilities(list(argN(n, i+5)))
	if err != nil {
		return nil, err
	}
	return &ir.Face{
		ColorIndicator: colors,
		Supertypes:     strList(argN(n, i+2)),
		Types:          strList(argN(n, i+3)),
		Subtypes:       subs,
		Power:          &ir.PT{N: pt.p.N},
		Toughness:      &ir.PT{N: pt.t.N},
		Abilities:      abs,
	}, nil
}

func tokenStep(face *ir.Face) *ir.Step {
	return &ir.Step{Verb: ir.VerbCreateToken, N: refConst1(), Token: face}
}

func init() {
	register("CreatableToken.TokenWithPT", func(c *Ctx, n node) (any, error) {
		f, err := tokenFace(c, n, "CreatableToken.TokenWithPT", 0)
		if err != nil {
			return nil, err
		}
		return tokenStep(f), nil
	})
	register("CreatableToken.NamedTokenWithPT", func(c *Ctx, n node) (any, error) {
		f, err := tokenFace(c, n, "CreatableToken.NamedTokenWithPT", 1)
		if err != nil {
			return nil, err
		}
		f.Name, _ = argN(n, 0).(string)
		return tokenStep(f), nil
	})
	register("CreatableToken.NumberTokens", func(c *Ctx, n node) (any, error) {
		v, err := c.Value(argN(n, 0))
		if err != nil {
			return nil, err
		}
		ss, err := c.Steps(argN(n, 1))
		if err != nil {
			return nil, err
		}
		if len(ss) != 1 {
			return nil, fmt.Errorf("%s: want one token step, got %d", qualified(argN(n, 1)), len(ss))
		}
		ss[0].N = v
		return ss[0], nil
	})
	predefined := func(name string) lowerFunc {
		return func(*Ctx, node) (any, error) {
			return tokenStep(&ir.Face{Name: name, Types: []string{"Artifact"}, Subtypes: []string{name}}), nil
		}
	}
	register("CreatableToken.FoodToken", predefined("Food"))
	register("CreatableToken.TreasureToken", predefined("Treasure"))
	copyOf := func(c *Ctx, n node) (any, error) {
		r, err := c.Ref(argN(n, 0))
		if err != nil {
			return nil, err
		}
		v, err := c.lower(argN(n, 1))
		if err != nil {
			return nil, err
		}
		effs, ok := v.([]*ir.Effect)
		if !ok {
			return nil, fmt.Errorf("%s lowers to %T, want effects", qualified(argN(n, 1)), v)
		}
		return &ir.Step{Verb: ir.VerbCreateTokenCopy, N: refConst1(), Obj: r, Effects: effs}, nil
	}
	register("CreatableToken.TokenCopyOfPermanent", copyOf)
	register("CreatableToken.TokenCopyOfGraveyardCard", copyOf)

	register("TokenColorList.Colors", func(c *Ctx, n node) (any, error) {
		var cs ir.ColorSet
		for _, s := range strList(rawArgs(n)) {
			bit, ok := charsColorNames[s]
			if !ok {
				return nil, fmt.Errorf("TokenColorList.Colors: unknown color %q", s)
			}
			cs |= bit
		}
		return cs, nil
	})
	register("TokenColorList.Colorless", func(*Ctx, node) (any, error) { return ir.ColorSet(0), nil })
	register("TokenSubtypes.SubtypesList", func(c *Ctx, n node) (any, error) {
		return strList(rawArgs(n)), nil
	})
	register("TokenFlag.EntersTapped", func(*Ctx, node) (any, error) { return &ir.Enter{Tapped: true}, nil })

	register("TokenCopyEffects.NoTokenCopyEffects", func(*Ctx, node) (any, error) { return []*ir.Effect(nil), nil })
	register("TokenCopyEffects.TokenCopyEffects", func(c *Ctx, n node) (any, error) {
		return c.Effects(list(rawArgs(n)))
	})
	register("TokenCopyEffect.AddAbility", func(c *Ctx, n node) (any, error) {
		abs, err := c.NestedAbilities(list(rawArgs(n)))
		if err != nil {
			return nil, err
		}
		return &ir.Effect{Kind: ir.EffAddAbility, Abilities: abs}, nil
	})
	register("TokenCopyEffect.AddCreatureTypes", func(c *Ctx, n node) (any, error) {
		return &ir.Effect{Kind: ir.EffAddSubtype, Words: strList(rawArgs(n))}, nil
	})
}
