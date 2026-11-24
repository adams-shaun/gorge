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
	if tr.Event != ir.EvWouldEnter && tr.Event != ir.EvWouldDealDamage {
		return unprintable("repl:"+tr.Event.String(), a.Src)
	}
	if tr.Event == ir.EvWouldEnter && !p.evSelf {
		return unprintable("repl:would-enter", a.Src)
	}
	if tr.Event == ir.EvWouldEnter && a.Body != nil && len(a.Body.Steps) == 1 && a.Body.Steps[0].Verb == ir.VerbModifyEnter &&
		enterCounterOnly(a.Body.Steps[0].Enter) {
		if a.Cond != nil {
			return unprintable("repl:etb-counter-cond", a.Src)
		}
		e := a.Body.Steps[0].Enter
		n, err := p.amount(e.N)
		if err != nil {
			return err
		}
		p.lines = append(p.lines, "K:etbCounter:"+e.Counter+":"+n)
		return nil
	}
	if tr.Event == ir.EvWouldEnter && a.Body != nil && len(a.Body.Steps) == 1 && a.Body.Steps[0].Verb == ir.VerbIf && a.Body.Steps[0].Then != nil &&
		len(a.Body.Steps[0].Then.Steps) == 1 && a.Body.Steps[0].Then.Steps[0].Verb == ir.VerbModifyEnter &&
		enterCounterOnly(a.Body.Steps[0].Then.Steps[0].Enter) {
		return unprintable("repl:etb-counter-cond", a.Src)
	}
	head, err := p.program(a.Body)
	if err != nil {
		return err
	}
	var ps params
	if tr.Event == ir.EvWouldEnter {
		ps = params{{"Event", "Moved"}, {"ValidCard", "Card.Self"}, {"Destination", "Battlefield"}, {"ReplacementResult", "Updated"}}
		if a.Cond != nil {
			return unprintable("repl:cond", a.Src)
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
	var b strings.Builder
	b.WriteString("R:")
	for i, x := range ps {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(x.K + "$ " + x.V)
	}
	p.lines = append(p.lines, b.String())
	return nil
}
