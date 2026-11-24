package gorgebe

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { registerStep(ir.VerbApplyEffect, printApplyEffect) }

func isPumpEffect(k ir.EffectKind) bool { return k == ir.EffAdjustPT || k == ir.EffAddAbility }

func isAnimateEffect(k ir.EffectKind) bool {
	switch k {
	case ir.EffSetPT, ir.EffAddCardType, ir.EffAddSubtype, ir.EffSetSubtype, ir.EffSetColor, ir.EffAddAbility, ir.EffAllCreatureTypes:
		return true
	}
	return false
}

// bareKeywords lists the keyword names of e's abilities; ok is false when any
// ability is not a bare keyword.
func bareKeywords(e *ir.Effect) (kws []string, ok bool) {
	if len(e.Abilities) == 0 {
		return nil, false
	}
	for _, a := range e.Abilities {
		k := a.Keyword
		if a.Kind != ir.AbKeyword || k == nil || k.Name == "" || k.N != nil || k.Cost != nil || k.Filter != nil {
			return nil, false
		}
		kws = append(kws, k.Name)
	}
	return kws, true
}

// grantedKeywords is bareKeywords, plus protection from colors, spelled the
// way a static's AddKeyword$ writes it.
func grantedKeywords(e *ir.Effect) (kws []string, ok bool) {
	if len(e.Abilities) == 0 {
		return nil, false
	}
	for _, a := range e.Abilities {
		k := a.Keyword
		if a.Kind != ir.AbKeyword || k == nil || k.Name == "" || k.N != nil || k.Cost != nil {
			return nil, false
		}
		if k.Filter == nil {
			kws = append(kws, k.Name)
			continue
		}
		w, ok := protectionWords(k)
		if !ok {
			return nil, false
		}
		kws = append(kws, w...)
	}
	return kws, true
}

// protectionWords spells "protection from <colors>" as one keyword per color.
func protectionWords(k *ir.Keyword) ([]string, bool) {
	f := k.Filter
	if k.Name != "Protection" || f == nil || f.Op != ir.FPred || f.Pred != ir.PColor || f.Colors == 0 {
		return nil, false
	}
	var out []string
	for _, c := range colourNames {
		if f.Colors&c.c != 0 {
			out = append(out, "Protection from "+lower(c.name))
		}
	}
	return out, true
}

func printApplyEffect(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	o := st.Obj
	if o == nil || (o.Kind != ir.RefSelf && o.Kind != ir.RefTarget) {
		return nil, 0, unprintable("effect:obj", st.Src)
	}
	var dur []kv
	switch st.Dur {
	case ir.DurEndOfTurn:
	case ir.DurPermanent:
		dur = []kv{{"Duration", "Permanent"}}
	case ir.DurUntilYourNextTurn:
		dur = []kv{{"Duration", "UntilYourNextTurn"}}
	case ir.DurEndOfCombat:
		dur = []kv{{"Duration", "UntilEndOfCombat"}}
	default:
		return nil, 0, unprintable("effect:dur-"+st.Dur.String(), st.Src)
	}
	if len(st.Effects) == 0 {
		return nil, 0, unprintable("effect:none", st.Src)
	}
	pump, anim, mixed := true, true, false
	for _, e := range st.Effects {
		pump = pump && isPumpEffect(e.Kind)
		anim = anim && isAnimateEffect(e.Kind)
		if !isPumpEffect(e.Kind) && !isAnimateEffect(e.Kind) {
			mixed = true
		}
	}
	var n *node
	var err error
	switch {
	case pump:
		n, err = pumpNode(p, st)
	case anim:
		n, err = animateNode(p, st)
	default:
		bad := st.Effects[0]
		for _, e := range st.Effects {
			if !isPumpEffect(e.Kind) && !isAnimateEffect(e.Kind) {
				bad = e
				break
			}
		}
		if !mixed {
			for _, e := range st.Effects {
				if !isAnimateEffect(e.Kind) {
					bad = e
					break
				}
			}
		}
		return nil, 0, unprintable("effect:"+bad.Kind.String(), bad.Src)
	}
	if err != nil {
		return nil, 0, err
	}
	if err := p.operand(n, "Defined", o); err != nil {
		return nil, 0, err
	}
	for _, x := range dur {
		n.set(x.K, x.V)
	}
	if err := zoptional(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

// signedSum is the summed amount of one pump axis, printed signed; omit is
// true when the sum is the constant 0.
func signedSum(p *pr, effs []*ir.Effect, pick func(*ir.Effect) *ir.Value) (s string, omit bool, err error) {
	c := 0
	var rest []*ir.Value
	for _, e := range effs {
		if e.Kind != ir.EffAdjustPT {
			continue
		}
		v := pick(e)
		switch {
		case v == nil:
		case v.Kind == ir.VConst:
			c += int(v.N)
		default:
			rest = append(rest, v)
		}
	}
	switch {
	case len(rest) == 0 && c == 0:
		return "", true, nil
	case len(rest) == 0 && c > 0:
		return "+" + strconv.Itoa(c), false, nil
	case len(rest) == 0:
		return strconv.Itoa(c), false, nil
	case len(rest) == 1 && c == 0:
		a, err := p.amount(rest[0])
		if err != nil {
			return "", false, err
		}
		return "+" + a, false, nil
	}
	return "", false, unprintable("effect:adjust-pt-sum", rest[0].Src)
}

func pumpNode(p *pr, st *ir.Step) (*node, error) {
	n := &node{API: "Pump"}
	att, omit, err := signedSum(p, st.Effects, func(e *ir.Effect) *ir.Value { return e.P })
	if err != nil {
		return nil, err
	}
	if !omit {
		n.set("NumAtt", att)
	}
	def, omit, err := signedSum(p, st.Effects, func(e *ir.Effect) *ir.Value { return e.T })
	if err != nil {
		return nil, err
	}
	if !omit {
		n.set("NumDef", def)
	}
	var kws []string
	for _, e := range st.Effects {
		if e.Kind != ir.EffAddAbility {
			continue
		}
		k, ok := bareKeywords(e)
		if !ok {
			return nil, unprintable("effect:add-ability", e.Src)
		}
		kws = append(kws, k...)
	}
	if len(kws) > 0 {
		n.set("KW", strings.Join(kws, " & "))
	}
	return n, nil
}

func animateNode(p *pr, st *ir.Step) (*node, error) {
	n := &node{API: "Animate"}
	var ctypes, subs, kws []string
	var colors ir.ColorSet
	setPT, removeTypes, allTypes, setColor := false, false, false, false
	for _, e := range st.Effects {
		switch e.Kind {
		case ir.EffSetPT:
			if setPT || e.P == nil || e.T == nil {
				return nil, unprintable("effect:set-pt", e.Src)
			}
			setPT = true
			pw, err := p.amount(e.P)
			if err != nil {
				return nil, err
			}
			to, err := p.amount(e.T)
			if err != nil {
				return nil, err
			}
			n.set("Power", pw)
			n.set("Toughness", to)
		case ir.EffAddCardType:
			for _, w := range e.Words {
				ctypes = append(ctypes, Respell(w))
			}
		case ir.EffAddSubtype, ir.EffSetSubtype:
			for _, w := range e.Words {
				subs = append(subs, Respell(w))
			}
			removeTypes = removeTypes || e.Kind == ir.EffSetSubtype
		case ir.EffSetColor:
			colors |= e.Colors
			setColor = true
		case ir.EffAddAbility:
			k, ok := bareKeywords(e)
			if !ok {
				return nil, unprintable("effect:add-ability", e.Src)
			}
			kws = append(kws, k...)
		case ir.EffAllCreatureTypes:
			allTypes = true
		}
	}
	if ts := append(ctypes, subs...); len(ts) > 0 {
		n.set("Types", strings.Join(ts, ","))
	}
	if removeTypes {
		n.set("RemoveCreatureTypes", "True")
	}
	if setColor {
		var cs []string
		for _, c := range colourNames {
			if colors&c.c != 0 {
				cs = append(cs, c.name)
			}
		}
		n.set("Colors", strings.Join(cs, ","))
		n.set("OverwriteColors", "True")
	}
	if len(kws) > 0 {
		n.set("Keywords", strings.Join(kws, " & "))
	}
	if allTypes {
		n.set("AddAllCreatureTypes", "True")
	}
	return n, nil
}
