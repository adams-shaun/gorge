package gorgebe

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { registerAbility(ir.AbStatic, printStatic) }

func isSelfRef(r *ir.Ref) bool { return r != nil && r.Kind == ir.RefSelf }

// playersOf spells a player object: you, or every player a filter selects.
func (p *pr) playersOf(r *ir.Ref) (string, bool, error) {
	switch {
	case r == nil:
		return "", false, nil
	case r.Kind == ir.RefYou:
		return "You", true, nil
	case r.Kind == ir.RefAll && r.Filter != nil && r.Filter.Subject == ir.SubjPlayer:
		v, err := p.valid(r.Filter)
		return v, err == nil, err
	}
	return "", false, nil
}

// genericCost is the amount of a cost of generic mana alone.
func genericCost(c *ir.Cost) (int32, bool) {
	if c == nil || !c.HasMana || len(c.Parts) != 0 || len(c.Mana) != 1 || c.Mana[0].Kind != ir.ManaGeneric {
		return 0, false
	}
	return c.Mana[0].N, true
}

// ptSum is the sum of one adjust-pt axis: "" when it is the constant 0.
func ptSum(p *pr, effs []*ir.Effect, pick func(*ir.Effect) *ir.Value) (string, error) {
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
		return "", nil
	case len(rest) == 0:
		return strconv.Itoa(c), nil
	case len(rest) == 1 && c == 0:
		return p.amount(rest[0])
	}
	return "", unprintable("static:adjust-pt-sum", rest[0].Src)
}

func printStatic(p *pr, a *ir.Ability) error {
	body := a.Body
	if body == nil || len(body.Steps) != 1 || len(body.Targets) != 0 {
		return unprintable("static:body", a.Src)
	}
	st := body.Steps[0]
	if st.Verb != ir.VerbApplyEffect || st.Dur != ir.DurWhileActive || st.Optional || len(st.Effects) == 0 {
		return unprintable("static:body", a.Src)
	}
	o := st.Obj
	var ps params
	var line string // "S:" or "R:"
	e := st.Effects[0]
	bad := func(e *ir.Effect) error { return unprintable("static:"+e.Kind.String(), e.Src) }
	continuous := true
	for _, x := range st.Effects {
		continuous = continuous && (x.Kind == ir.EffAdjustPT || x.Kind == ir.EffAddAbility)
	}
	switch {
	case continuous:
		var aff string
		switch {
		case isSelfRef(o):
			aff = "Card.Self"
		case o != nil && o.Kind == ir.RefAll && o.Filter != nil && o.Filter.Subject != ir.SubjPlayer:
			v, err := p.valid(o.Filter)
			if err != nil {
				return err
			}
			aff = v
		default:
			return bad(e)
		}
		ps = params{{"Mode", "Continuous"}, {"Affected", aff}}
		pw, err := ptSum(p, st.Effects, func(e *ir.Effect) *ir.Value { return e.P })
		if err != nil {
			return err
		}
		if pw != "" {
			ps.set("AddPower", pw)
		}
		tg, err := ptSum(p, st.Effects, func(e *ir.Effect) *ir.Value { return e.T })
		if err != nil {
			return err
		}
		if tg != "" {
			ps.set("AddToughness", tg)
		}
		var kws []string
		for _, x := range st.Effects {
			if x.Kind != ir.EffAddAbility {
				continue
			}
			k, ok := bareKeywords(x)
			if !ok {
				return bad(x)
			}
			kws = append(kws, k...)
		}
		if len(kws) > 0 {
			ps.set("AddKeyword", strings.Join(kws, " & "))
		}
		line = "S:"
	case len(st.Effects) != 1:
		return unprintable("static:body", a.Src)
	default:
		var err error
		if ps, line, err = p.staticEffect(a, st, e); err != nil {
			return err
		}
	}
	if a.Cond != nil {
		f, err := p.cond(a.Cond)
		if err != nil {
			return err
		}
		f = p.settle(f)
		if aff, ok := ps.selfTest(f); ok {
			// Forge tests the source's own state in the Affected filter.
			ps = aff
		} else {
			cs, err := f.onStatic()
			if err != nil {
				return err
			}
			for _, c := range cs {
				ps.set(c.K, c.V)
			}
		}
	}
	var b strings.Builder
	b.WriteString(line)
	for i, x := range ps {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(x.K + "$ " + x.V)
	}
	p.lines = append(p.lines, b.String())
	return nil
}

// staticEffect prints the single non-continuous effect e of st.
func (p *pr) staticEffect(a *ir.Ability, st *ir.Step, e *ir.Effect) (params, string, error) {
	o := st.Obj
	bad := unprintable("static:"+e.Kind.String(), e.Src)
	anySpell := func() (bool, string, error) {
		if e.Filter == nil {
			return true, "", nil
		}
		v, err := p.valid(e.Filter)
		return v == "Card", v, err
	}
	switch e.Kind {
	case ir.EffCantBlock:
		if isSelfRef(o) {
			return params{{"Mode", "CantBlock"}, {"ValidCard", "Card.Self"}}, "S:", nil
		}
	case ir.EffCantBeBlocked:
		if isSelfRef(o) {
			return params{{"Mode", "CantBlockBy"}, {"ValidAttacker", "Card.Self"}}, "S:", nil
		}
	case ir.EffNoMaxHandSize:
		if o != nil && o.Kind == ir.RefYou {
			return params{{"Mode", "Continuous"}, {"Affected", "You"}, {"SetMaxHandSize", "Unlimited"}}, "S:", nil
		}
	case ir.EffFlash:
		if o != nil && o.Kind == ir.RefYou && e.Filter != nil {
			v, err := p.valid(e.Filter)
			if err != nil {
				return nil, "", err
			}
			return params{{"Mode", "CastWithFlash"}, {"ValidCard", v}, {"ValidSA", "Spell"}, {"Caster", "You"}}, "S:", nil
		}
	case ir.EffCastWithoutPaying:
		if o != nil && o.Kind == ir.RefYou {
			any, _, err := anySpell()
			if err != nil {
				return nil, "", err
			}
			if any {
				return params{{"Mode", "Continuous"}, {"Affected", "Card.nonLand+YouOwn"}, {"MayPlay", "True"},
					{"MayPlayWithoutManaCost", "True"}, {"AffectedZone", "Hand"}}, "S:", nil
			}
		}
	case ir.EffSpellsCostMore, ir.EffSpellsCostLess:
		n, ok := genericCost(e.Cost)
		if !ok || o == nil || o.Kind != ir.RefAll || o.Filter == nil || o.Filter.Subject != ir.SubjPlayer {
			return nil, "", bad
		}
		who, err := p.valid(o.Filter)
		if err != nil {
			return nil, "", err
		}
		mode := "RaiseCost"
		if e.Kind == ir.EffSpellsCostLess {
			mode = "ReduceCost"
		}
		ps := params{{"Mode", mode}, {"Activator", who}, {"Type", "Spell"}, {"Amount", strconv.Itoa(int(n))}}
		any, v, err := anySpell()
		if err != nil {
			return nil, "", err
		}
		if !any {
			ps.set("ValidCard", v)
		}
		return ps, "S:", nil
	case ir.EffAlternativeCost:
		if isSelfRef(o) {
			c, err := p.costText(e.Cost)
			if err != nil {
				return nil, "", err
			}
			ps := params{{"Mode", "AlternativeCost"}, {"ValidSA", "Spell.Self"}, {"EffectZone", "All"}, {"Cost", c}}
			if e.Cond != nil {
				gate, err := p.battlefieldCount(e.Cond)
				if err != nil {
					return nil, "", err
				}
				for _, g := range gate {
					ps.set(g.K, g.V)
				}
			}
			return ps, "S:", nil
		}
	case ir.EffCantLose, ir.EffCantWin:
		who, ok, err := p.playersOf(o)
		if err != nil {
			return nil, "", err
		}
		if ok {
			ev := "GameLoss"
			if e.Kind == ir.EffCantWin {
				ev = "GameWin"
			}
			return params{{"Event", ev}, {"ActiveZones", "Battlefield"}, {"ValidPlayer", who}, {"Layer", "CantHappen"}}, "R:", nil
		}
	case ir.EffCantBeCountered:
		if isSelfRef(o) && a.Zones == ir.MakeZoneSet(ir.ZoneStack) {
			return params{{"Event", "Counter"}, {"ValidCard", "Card.Self"}, {"ValidSA", "Spell"}, {"Layer", "CantHappen"}}, "R:", nil
		}
	}
	return nil, "", bad
}

// selfTest folds a test of the source's own state ("while this is untapped")
// into the Affected filter, the way Forge spells it: AffectedDefined$ Self and
// the test's qualifiers on Affected$. It reports false when c is not such a test.
func (ps params) selfTest(c condForm) (params, bool) {
	ok := false
	for _, x := range ps {
		ok = ok || (x.K == "Affected" && x.V == "Card.Self")
	}
	if !ok || c.Present == "" || (c.Defined != "" && c.Defined != "Self") || c.cmp() != "GE1" || strings.Contains(c.Present, ",") {
		return nil, false
	}
	q := c.Present
	switch {
	case strings.HasPrefix(q, "Permanent."):
		q = "Card." + q[len("Permanent."):]
	case q == "Permanent":
		q = "Card"
	default:
		return nil, false
	}
	var out params
	for _, x := range ps {
		if x.K == "Affected" {
			out = append(out, kv{"AffectedDefined", "Self"}, kv{"Affected", q})
			continue
		}
		out = append(out, x)
	}
	return out, true
}
