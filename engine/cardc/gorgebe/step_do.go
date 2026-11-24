package gorgebe

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() { registerStep(ir.VerbDo, printDo) }

// printDo prints a do-block. The one shape it knows is a token replacement
// that creates its tokens twice ("those tokens, those tokens"), which is
// ReplaceToken$ Type$ Amount.
func printDo(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	bad := unprintable("verb:do", st.Src)
	if p.ev != ir.EvWouldCreateTokens || st.Optional || st.Who != nil || st.Then == nil || len(st.Then.Steps) != 2 {
		return nil, 0, bad
	}
	for _, s := range st.Then.Steps {
		if s.Verb != ir.VerbCreateToken || s.Token != nil || s.Optional || s.Obj == nil || s.Obj.Kind != ir.RefEventObject {
			return nil, 0, bad
		}
	}
	return &node{API: "ReplaceToken", Params: []kv{{"Type", "Amount"}}}, 1, nil
}

// printTokenReplacement prints "if an effect would create tokens under your
// control" as Event$ CreateToken. Only your own tokens, by any effect, print.
func printTokenReplacement(p *pr, a *ir.Ability) error {
	tr := a.Trigger
	bad := unprintable("repl:would-create-tokens", a.Src)
	s, o := tr.Subject, tr.Object
	if a.Cond != nil || s == nil || s.Subject != ir.SubjPermanent || s.Op != ir.FPred || s.Pred != ir.PAny ||
		o == nil || o.Subject != ir.SubjPlayer || o.Op != ir.FPred || o.Pred != ir.PPlayerIs || o.Ref == nil || o.Ref.Kind != ir.RefYou {
		return bad
	}
	head, err := p.program(a.Body)
	if err != nil {
		return err
	}
	ps := params{{"Event", "CreateToken"}, {"ActiveZones", "Battlefield"}, {"ValidToken", "Card.YouCtrl"}}
	ps.set("ReplaceWith", p.sub("DB"+head.API, head))
	ps.set("EffectOnly", "True")
	line := "R:"
	for i, x := range ps {
		if i > 0 {
			line += " | "
		}
		line += x.K + "$ " + x.V
	}
	p.lines = append(p.lines, line)
	return nil
}
