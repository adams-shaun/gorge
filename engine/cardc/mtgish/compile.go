package mtgish

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// Compile lowers one mtgish card to IR. It never panics and never returns
// nil: a construct without a lowering is a Refusal on its face, and a layout
// cardc does not play is one face holding the single refusal.
func Compile(card *Card) *ir.Card {
	out := &ir.Card{Name: card.Name}
	if card.Shared == nil {
		out.Faces = []*ir.Face{{Name: card.Name, Refusals: []ir.Refusal{{Key: "OracleCard." + card.Layout}}}}
		return out
	}
	layoutKey := "OracleCard." + card.Layout
	c := &Ctx{Card: card.Name}
	var layout *ir.Layout
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()
		var v any
		if v, err = c.lower(card.Shared); err == nil {
			layout, _ = v.(*ir.Layout)
		}
	}()
	if err != nil || layout == nil {
		key := layoutKey
		if _, isRef := err.(*RefusalError); err != nil && !isRef {
			key = "error:" + layoutKey
		}
		out.Faces = []*ir.Face{{Name: card.Name, Refusals: []ir.Refusal{{Key: key, Path: layoutKey}}}}
		out.Src = layoutKey
		return out
	}
	out.Layout = *layout
	out.Src = layoutKey
	for i, fn := range card.Faces {
		out.Faces = append(out.Faces, charsFace(card, i, fn))
	}
	return out
}

// charsRefuse records err on f: a RefusalError under its own key, anything
// else under error:<key>.
func charsRefuse(f *ir.Face, key string, err error) {
	if re, ok := err.(*RefusalError); ok {
		f.Refusals = append(f.Refusals, ir.Refusal{Key: re.Key, Path: re.Path})
		return
	}
	f.Refusals = append(f.Refusals, ir.Refusal{Key: "error:" + key, Path: key})
}

// charsGuard runs f, turning a panic into an error.
func charsGuard(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return f()
}

func charsFace(card *Card, idx int, fn node) *ir.Face {
	f := &ir.Face{}
	f.Name, _ = fn["Name"].(string)
	if f.Name == "" {
		f.Name = card.Name
	}
	c := &Ctx{Card: card.Name, Face: idx}

	tl, _ := fn["Typeline"].(map[string]any)
	if tl == nil && card.Shared != nil {
		tl, _ = card.Shared["Typeline"].(map[string]any)
	}
	f.Supertypes = charsStrings(tl["Supertypes"])
	f.Types = charsStrings(tl["Cardtypes"])
	f.Subtypes = charsStrings(tl["Subtypes"])

	mc, has := fn["ManaCost"]
	if !has && idx == 0 && card.Layout == "Flip" && card.Shared != nil {
		mc, has = card.Shared["ManaCost"]
	}
	if has {
		var syms []ir.ManaSym
		bad := false
		for _, s := range list(mc) {
			var sym *ir.ManaSym
			err := charsGuard(func() error {
				v, err := c.lower(s)
				if err != nil {
					return err
				}
				var ok bool
				if sym, ok = v.(*ir.ManaSym); !ok {
					return fmt.Errorf("%s lowers to %T", qualified(s), v)
				}
				return nil
			})
			if err != nil {
				bad = true
				charsRefuse(f, qualified(s), err)
				continue
			}
			syms = append(syms, *sym)
		}
		if !bad {
			f.ManaCost = syms
			f.HasManaCost = true
		}
	}

	for _, s := range list(fn["ColorIndicator"]) {
		name, _ := s.(string)
		bit, ok := charsColorNames[name]
		if !ok {
			f.Refusals = append(f.Refusals, ir.Refusal{Key: "ColorIndicatorColor." + name, Path: "ColorIndicator"})
			continue
		}
		f.ColorIndicator |= bit
	}

	rules := list(fn["Rules"])
	cdaP, cdaT := false, false
	for _, r := range rules {
		switch qualified(r) {
		case "Rule.CDA_Power":
			cdaP = true
		case "Rule.CDA_Toughness":
			cdaT = true
		}
	}
	if pt, ok := fn["CardPT"].(map[string]any); ok {
		if p, ok := intOf(pt["Power"]); ok {
			f.Power = &ir.PT{N: int32(p), Star: cdaP, Src: "CardPT/Power"}
		}
		if t, ok := intOf(pt["Toughness"]); ok {
			f.Toughness = &ir.PT{N: int32(t), Star: cdaT, Src: "CardPT/Toughness"}
		}
	}
	if l, ok := fn["Loyalty"]; ok {
		err := charsGuard(func() error {
			v, err := c.lower(l)
			if err != nil {
				return err
			}
			x, ok := v.(*int32)
			if !ok {
				return fmt.Errorf("%s lowers to %T", qualified(l), v)
			}
			f.Loyalty = x
			return nil
		})
		if err != nil {
			charsRefuse(f, qualified(l), err)
		}
	}
	if d, ok := intOf(fn["Defense"]); ok {
		x := int32(d)
		f.Defense = &x
	}

	for _, r := range rules {
		key := qualified(r)
		c.ResetTargets()
		var ab *ir.Ability
		err := charsGuard(func() (err error) {
			ab, err = c.Ability(r)
			return err
		})
		if err != nil {
			charsRefuse(f, key, err)
			continue
		}
		f.Abilities = append(f.Abilities, ab)
	}
	return f
}

func charsStrings(x any) []string {
	var out []string
	for _, e := range list(x) {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
