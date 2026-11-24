package gorgebe

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { registerStep(ir.VerbApplyEffect, printApplyEffect) }

func isPumpEffect(k ir.EffectKind) bool {
	return k == ir.EffAdjustPT || k == ir.EffAddAbility || k == ir.EffCantBlock
}

// cantBlockKW is the keyword Pump$ grants for "can't block".
const cantBlockKW = "HIDDEN CARDNAME can't block."

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

func printApplyEffect(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	o := slotRef(st.Obj)
	if o == nil {
		return nil, 0, unprintable("effect:obj", st.Src)
	}
	if o.Kind == ir.RefAll && o.Filter != nil && o.Filter.Subject != ir.SubjPlayer {
		return printApplyEffectAll(p, st)
	}
	switch o.Kind {
	case ir.RefSelf, ir.RefTarget, ir.RefAttachedTo, ir.RefEventObject:
	default:
		return nil, 0, unprintable("effect:obj", st.Src)
	}
	dur, err := effectDur(st)
	if err != nil {
		return nil, 0, err
	}
	if len(st.Effects) == 1 && st.Effects[0].Kind == ir.EffCantBeBlocked {
		return printUnblockable(p, st, dur)
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
	if err := p.operand(n, "Defined", st.Obj); err != nil {
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
		case v == nil || zeroProduct(v):
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
		a, err := p.amountAs("X", rest[0])
		if err != nil {
			return "", false, err
		}
		return "+" + a, false, nil
	}
	return "", false, unprintable("effect:adjust-pt-sum", rest[0].Src)
}

// zeroProduct reports whether v is a product with a constant 0 factor, which
// Forge's script leaves out.
func zeroProduct(v *ir.Value) bool {
	if v.Kind != ir.VMul {
		return false
	}
	for _, a := range v.Args {
		if a != nil && a.Kind == ir.VConst && a.N == 0 {
			return true
		}
	}
	return false
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
		if e.Kind == ir.EffCantBlock {
			kws = append(kws, cantBlockKW)
			continue
		}
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

// printApplyEffectAll prints an effect on each permanent a filter selects:
// PumpAll$ with ValidCards$. Only power, toughness and keyword changes have
// that form here.
func printApplyEffectAll(p *pr, st *ir.Step) (*node, int, error) {
	if len(st.Effects) == 0 {
		return nil, 0, unprintable("effect:none", st.Src)
	}
	for _, e := range st.Effects {
		if !isPumpEffect(e.Kind) {
			return nil, 0, unprintable("effect:all-"+e.Kind.String(), e.Src)
		}
	}
	dur, err := effectDur(st)
	if err != nil {
		return nil, 0, err
	}
	n, err := pumpNode(p, st)
	if err != nil {
		return nil, 0, err
	}
	n.API = "PumpAll"
	v, err := p.valid(st.Obj.Filter)
	if err != nil {
		return nil, 0, err
	}
	n.set("ValidCards", v)
	for _, x := range dur {
		n.set(x.K, x.V)
	}
	if err := zoptional(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

// effectDur is the Duration$ parameters of st's duration; end of turn is Forge's default.
func effectDur(st *ir.Step) ([]kv, error) {
	switch st.Dur {
	case ir.DurEndOfTurn:
		return nil, nil
	case ir.DurPermanent:
		return []kv{{"Duration", "Permanent"}}, nil
	case ir.DurUntilYourNextTurn:
		return []kv{{"Duration", "UntilYourNextTurn"}}, nil
	case ir.DurEndOfCombat:
		return []kv{{"Duration", "UntilEndOfCombat"}}, nil
	}
	return nil, unprintable("effect:dur-"+st.Dur.String(), st.Src)
}

// mergeTargetCount folds "if kicked: any number of targets, else: one target"
// into then, the way Forge writes it: TargetMin$ and TargetMax$ read counts of
// the kicker. The two branches are the same node but for that target count.
func (p *pr) mergeTargetCount(c condForm, then, els *node, before int) bool {
	if then.API != els.API || then.Sub != nil || els.Sub != nil || c.SVar == "" {
		return false
	}
	body := ""
	idx := -1
	for i, s := range p.svars {
		if s.K == c.SVar {
			body, idx = s.V, i
		}
	}
	if body != "Count$Kicked.1.0" || idx < before {
		return false
	}
	if _, ok := els.get("TargetMin"); ok {
		return false
	}
	if mn, _ := then.get("TargetMin"); mn != "0" {
		return false
	}
	if mx, _ := then.get("TargetMax"); mx != "-1" {
		return false
	}
	var rest []kv
	for _, x := range then.Params {
		if x.K != "TargetMin" && x.K != "TargetMax" {
			rest = append(rest, x)
		}
	}
	if len(rest) != len(els.Params) {
		return false
	}
	for i := range rest {
		if rest[i] != els.Params[i] {
			return false
		}
	}
	v, ok := then.get("ValidTgts")
	if !ok {
		return false
	}
	p.svars = append(p.svars[:idx], p.svars[idx+1:]...)
	lo := p.countVarAs("X", "Count$Kicked.0.1")
	all := p.countVarAs("Z", "Count$Valid "+v)
	hi := p.countVarAs("Y", "Count$Kicked."+all+".1")
	then.Params = append(rest, kv{"TargetMin", lo}, kv{"TargetMax", hi})
	return true
}

// rememberOperand puts the object an Effect$ node's continuous effect applies
// to on n as RememberObjects$: Targeted (declaring the slot the first time),
// or Self.
func (p *pr) rememberOperand(n *node, r *ir.Ref) error {
	r = slotRef(r)
	switch {
	case r != nil && r.Kind == ir.RefSelf:
		n.set("RememberObjects", "Self")
		return nil
	case r != nil && r.Kind == ir.RefTarget:
		ps, err := p.declare(r.Slot, r.Src)
		if err != nil {
			return err
		}
		if ps == nil {
			return unprintable("effect:effect-obj", r.Src)
		}
		for _, x := range ps {
			n.set(x.K, x.V)
		}
		n.set("RememberObjects", "Targeted")
		return nil
	}
	return unprintable("effect:effect-obj", "")
}

// effectStatic adds a static-ability SVar to the Effect$ node n, naming it base.
func (p *pr) effectStatic(n *node, base, body string) {
	name := p.fresh(base)
	p.svars = append(p.svars, kv{name, body})
	n.set("StaticAbilities", name)
}

// printUnblockable prints "can't be blocked this turn" as Effect$: a static
// of the remembered object.
func printUnblockable(p *pr, st *ir.Step, dur []kv) (*node, int, error) {
	if len(dur) > 0 {
		return nil, 0, unprintable("effect:cant-be-blocked-dur", st.Src)
	}
	n := &node{API: "Effect"}
	if err := p.rememberOperand(n, st.Obj); err != nil {
		return nil, 0, err
	}
	n.set("ExileOnMoved", "Battlefield")
	p.effectStatic(n, "Unblockable", "Mode$ CantBlockBy | ValidAttacker$ Card.IsRemembered")
	if err := zoptional(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}
