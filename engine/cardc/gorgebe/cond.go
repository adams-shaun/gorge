package gorgebe

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func init() { condFn = printCond }

// cmpWord is op upper-cased ("GE"), or "" for CmpNone.
func cmpWord(op ir.CmpOp) string {
	if op == ir.CmpNone {
		return ""
	}
	return strings.ToUpper(op.String())
}

// cmpAgainst is the "<CMP><n>" comparison of op against v.
func (p *pr) cmpAgainst(op ir.CmpOp, v *ir.Value, src string) (string, error) {
	w := cmpWord(op)
	if w == "" {
		return "", unprintable("cond:compare", src)
	}
	n, err := p.amount(v)
	if err != nil {
		return "", err
	}
	return w + n, nil
}

// isPred reports whether f is exactly the predicate pred with no operands.
func isPred(f *ir.Filter, pred ir.Pred) bool {
	return f != nil && f.Op == ir.FPred && f.Pred == pred && len(f.Args) == 0 && f.Ref == nil && f.Word == "" && f.Colors == 0
}

// isCmpPred is isPred for a numeric predicate: Cmp and Value set.
func isCmpPred(f *ir.Filter, pred ir.Pred) bool {
	return isPred(f, pred) && f.Cmp != ir.CmpNone && f.Value != nil
}

func isOpponentOfYou(f *ir.Filter) bool {
	return f != nil && f.Op == ir.FPred && f.Pred == ir.POpponentOf && len(f.Args) == 0 && isYou(f.Ref) && f.Word == "" && f.Colors == 0
}

// playerRefs are the reference kinds that denote a player.
var playerRefs = map[ir.RefKind]bool{
	ir.RefYou: true, ir.RefOwnerOf: true, ir.RefControllerOf: true, ir.RefActivePlayer: true, ir.RefDefendingPlayer: true,
	ir.RefEventPlayer: true, ir.RefIterPlayer: true, ir.RefPlayerOrControllerOf: true,
}

func printCond(p *pr, c *ir.Cond) (condForm, error) {
	no := func() (condForm, error) { return condForm{}, unprintable("cond:"+c.Kind.String(), c.Src) }
	sv := func(body, cmp string) (condForm, error) { return condForm{SVar: p.countVar(body), Cmp: cmp}, nil }
	switch c.Kind {
	case ir.CNot:
		if len(c.Args) != 1 {
			return no()
		}
		f, err := p.cond(c.Args[0])
		if err != nil {
			return condForm{}, err
		}
		return f.not()
	case ir.CKicked:
		return sv("Count$Kicked.1.0", "GE1")
	case ir.CIsTurnOf:
		switch {
		case c.Ref != nil && c.Filter == nil && isYou(c.Ref):
			return condForm{Turn: "You"}, nil
		case c.Ref == nil && isOpponentOfYou(c.Filter):
			return condForm{Turn: "Opponent"}, nil
		}
		return no()
	case ir.CCompare:
		if c.L == nil || isConst(c.L) {
			return no()
		}
		l, err := p.amount(c.L)
		if err != nil {
			return condForm{}, err
		}
		if isLiteral(l) {
			return no()
		}
		cmp, err := p.cmpAgainst(c.Cmp, c.R, c.Src)
		if err != nil {
			return condForm{}, err
		}
		return condForm{SVar: l, Cmp: cmp}, nil
	case ir.CCountCompare:
		if c.Filter == nil {
			return no()
		}
		b, err := printValue(p, &ir.Value{Kind: ir.VCount, Filter: c.Filter, Src: c.Src})
		if err != nil {
			return condForm{}, err
		}
		cmp, err := p.cmpAgainst(c.Cmp, c.R, c.Src)
		if err != nil {
			return condForm{}, err
		}
		return sv(b, cmp)
	case ir.CExistsPlayer:
		f := c.Filter
		if f == nil || f.Op != ir.FAll || len(f.Args) != 2 || !isOpponentOfYou(f.Args[0]) || !isCmpPred(f.Args[1], ir.PLifeTotal) {
			return no()
		}
		var body string
		switch f.Args[1].Cmp {
		case ir.CmpLE, ir.CmpLT:
			body = "PlayerCountOpponents$LowestLifeTotal"
		case ir.CmpGE, ir.CmpGT:
			body = "PlayerCountOpponents$HighestLifeTotal"
		default:
			return no()
		}
		cmp, err := p.cmpAgainst(f.Args[1].Cmp, f.Args[1].Value, c.Src)
		if err != nil {
			return condForm{}, err
		}
		return sv(body, cmp)
	case ir.CRefMatches:
		return printRefMatches(p, c)
	}
	return no()
}

func printRefMatches(p *pr, c *ir.Cond) (condForm, error) {
	no := func() (condForm, error) { return condForm{}, unprintable("cond:ref-matches", c.Src) }
	f := c.Filter
	if c.Ref == nil || f == nil {
		return no()
	}
	if isYou(c.Ref) {
		if isPred(f, ir.PAttackedThisTurn) {
			return condForm{SVar: p.countVar("Count$AttackersDeclared"), Cmp: "GE1"}, nil
		}
		var body string
		switch {
		case isCmpPred(f, ir.PCardsInGraveyard):
			body = "Count$ValidGraveyard Card.YouOwn"
		case isCmpPred(f, ir.PCardsInHand):
			body = "Count$CardsInYourHand"
		case isCmpPred(f, ir.PLifeTotal):
			body = "Count$YourLifeTotal"
		default:
			return no()
		}
		cmp, err := p.cmpAgainst(f.Cmp, f.Value, c.Src)
		if err != nil {
			return condForm{}, err
		}
		return condForm{SVar: p.countVar(body), Cmp: cmp}, nil
	}
	if p.isPlayerTarget(c.Ref) {
		if !isCmpPred(f, ir.PLifeTotal) {
			return no()
		}
		cmp, err := p.cmpAgainst(f.Cmp, f.Value, c.Src)
		if err != nil {
			return condForm{}, err
		}
		return condForm{SVar: p.countVar("TargetedPlayer$LifeTotal"), Cmp: cmp}, nil
	}
	if playerRefs[c.Ref.Kind] || f.Subject == ir.SubjPlayer {
		return no()
	}
	present, err := p.valid(f)
	if err != nil {
		return condForm{}, err
	}
	def := ""
	if c.Ref.Kind != ir.RefSelf {
		if def, err = p.defined(c.Ref); err != nil {
			return condForm{}, err
		}
	}
	return condForm{Present: present, Defined: def, Cmp: "GE1"}, nil
}
