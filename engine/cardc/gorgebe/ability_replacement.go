package gorgebe

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { registerAbility(ir.AbReplacement, printReplacement) }

func printReplacement(p *pr, a *ir.Ability) error {
	tr := a.Trigger
	if tr == nil {
		return unprintable("repl:none", a.Src)
	}
	defer func() { p.ev, p.evSelf = ir.EvNone, false }()
	p.ev, p.evSelf = tr.Event, tr.Ref != nil && tr.Ref.Kind == ir.RefSelf
	if tr.Event == ir.EvWouldCreateTokens {
		return printTokenReplacement(p, a)
	}
	if tr.Event == ir.EvWouldPutCounters {
		return p.printCounterReplacement(a)
	}
	if tr.Event != ir.EvWouldEnter && tr.Event != ir.EvWouldDealDamage {
		return unprintable("repl:"+tr.Event.String(), a.Src)
	}
	if tr.Event == ir.EvWouldEnter && !p.evSelf && tr.Subject == nil {
		return unprintable("repl:would-enter", a.Src)
	}
	if tr.Event == ir.EvWouldEnter && p.evSelf && a.Body != nil && len(a.Body.Steps) == 1 {
		st := a.Body.Steps[0]
		cond, enter := a.Cond, (*ir.Enter)(nil)
		switch {
		case st.Verb == ir.VerbModifyEnter && enterCounterOnly(st.Enter):
			enter = st.Enter
		case st.Verb == ir.VerbIf && a.Cond == nil && st.Cond != nil && st.Else == nil && st.Then != nil && len(st.Then.Steps) == 1 &&
			st.Then.Steps[0].Verb == ir.VerbModifyEnter && enterCounterOnly(st.Then.Steps[0].Enter):
			cond, enter = st.Cond, st.Then.Steps[0].Enter
		}
		if enter != nil {
			n, err := p.amount(enter.N)
			if err != nil {
				return err
			}
			line := "K:etbCounter:" + enter.Counter + ":" + n
			if cond != nil {
				// Forge's line carries a description after the condition that no IR
				// fact holds, so the static diff cannot key it without Forge's text
				// in the ledger. Refused until the diff canonicalises it (CC-39).
				return unprintable("repl:etb-counter-cond", a.Src)
			}
			p.lines = append(p.lines, line)
			return nil
		}
	}
	if tr.Event == ir.EvWouldDealDamage && a.Body != nil && len(a.Body.Steps) == 1 && a.Body.Steps[0].Verb == ir.VerbPrevent &&
		a.Body.Steps[0].Obj == nil && len(a.Body.Targets) == 0 && a.Cond == nil {
		ps := params{{"Event", "DamageDone"}, {"ActiveZones", "Battlefield"}}
		if tr.Subject != nil {
			v, err := p.valid(tr.Subject)
			if err != nil {
				return err
			}
			ps = append(ps, kv{"ValidSource", v})
		}
		if tr.Object != nil {
			v, err := p.valid(tr.Object)
			if err != nil {
				return err
			}
			ps = append(ps, kv{"ValidTarget", v})
		}
		ps = append(ps, kv{"Prevent", "True"})
		switch tr.Word {
		case "":
		case "noncombat":
			ps = append(ps, kv{"IsCombat", "False"})
		case "combat":
			ps = append(ps, kv{"IsCombat", "True"})
		default:
			return unprintable("repl:damage-word", a.Src)
		}
		p.lines = append(p.lines, "R:"+ps.join())
		return nil
	}
	head, err := p.program(a.Body)
	if err != nil {
		return err
	}
	if tr.Event == ir.EvWouldEnter && p.evSelf && a.Cond == nil && strings.HasPrefix(head.API, "Choose") {
		// "as this enters, choose ...": Forge's keyword form
		p.lines = append(p.lines, "K:ETBReplacement:Other:"+p.sub(head.API, head))
		return nil
	}
	var ps params
	if tr.Event == ir.EvWouldEnter {
		ps = params{{"Event", "Moved"}, {"ValidCard", "Card.Self"}, {"Destination", "Battlefield"}, {"ReplacementResult", "Updated"}}
		if a.Cond != nil {
			return unprintable("repl:cond", a.Src)
		}
		if !p.evSelf {
			v, err := p.valid(tr.Subject)
			if err != nil {
				return err
			}
			ps = params{{"Event", "Moved"}, {"ValidCard", v}, {"Destination", "Battlefield"}, {"ReplaceWith", ""}, {"ReplacementResult", "Updated"}, {"ActiveZones", "Battlefield"}}
		}
		ps.set("ReplaceWith", p.sub("DB"+head.API, head))
	} else {
		src, err := p.valid(tr.Subject)
		if err != nil {
			return err
		}
		tgt, err := p.valid(tr.Object)
		if err != nil {
			return err
		}
		if a.Cond != nil {
			return unprintable("repl:cond", a.Src)
		}
		ps = params{{"Event", "DamageDone"}, {"ActiveZones", "Battlefield"}, {"ValidSource", src}, {"ValidTarget", tgt}}
		ps.set("ReplaceWith", p.sub("DB"+head.API, head))
	}
	p.lines = append(p.lines, "R:"+ps.join())
	return nil
}

// join spells the parameters as "K$ V | K$ V".
func (ps params) join() string {
	var b strings.Builder
	for i, x := range ps {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(x.K + "$ " + x.V)
	}
	return b.String()
}

// printCounterReplacement prints "if an effect would put counters on a
// permanent you control" as Event$ AddCounter.
func (p *pr) printCounterReplacement(a *ir.Ability) error {
	tr := a.Trigger
	if a.Cond != nil || tr.Object == nil || tr.Subject != nil || tr.Word != "" {
		return unprintable("repl:would-put-counters", a.Src)
	}
	v, err := p.valid(tr.Object)
	if err != nil {
		return err
	}
	head, err := p.program(a.Body)
	if err != nil {
		return err
	}
	ps := params{{"Event", "AddCounter"}, {"ActiveZones", "Battlefield"}, {"ValidCard", v + "+inZoneBattlefield"}, {"EffectOnly", "True"}}
	ps.set("ReplaceWith", p.sub("DB"+head.API, head))
	p.lines = append(p.lines, "R:"+ps.join())
	return nil
}
