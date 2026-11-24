package gorgebe

import (
	"strconv"
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
	sv := func(body, cmp string) (condForm, error) { return p.svCond(body, cmp), nil }
	switch c.Kind {
	case ir.CNot:
		if len(c.Args) != 1 {
			return no()
		}
		f, err := p.cond(c.Args[0])
		if err != nil {
			return condForm{}, err
		}
		return p.plain(f).not()
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
	case ir.CDiedThisTurn:
		// Morbid: a creature died this turn.
		if f := c.Filter; f == nil || f.Op != ir.FPred || f.Pred != ir.PCardType || f.Word != "Creature" {
			return no()
		}
		return sv("Count$Morbid.1.0", "GE1")
	case ir.CLeftBattlefieldThisTurn:
		// Revolt: a permanent you controlled left the battlefield this turn.
		f := c.Filter
		if f == nil || f.Op != ir.FPred || f.Pred != ir.PControlledBy || !isYou(f.Ref) {
			return no()
		}
		return sv("Count$Revolt.1.0", "GE1")
	case ir.CReceiptNonEmpty:
		return printReceiptNonEmpty(p, c)
	case ir.CExistsPlayer:
		f := c.Filter
		if f != nil && f.Op == ir.FAll && len(f.Args) == 2 && isOpponentOfYou(f.Args[0]) && isPred(f.Args[1], ir.PLostLifeThisTurn) {
			return sv("Count$LifeOppsLostThisTurn", "GE1")
		}
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
	if (c.Ref.Kind == ir.RefSelf || (c.Ref.Kind == ir.RefEventObject && p.ev == ir.EvWouldEnter && p.evSelf)) && isPred(f, ir.PKicked) {
		return p.svCond("Count$Kicked.1.0", "GE1"), nil
	}
	if isYou(c.Ref) && isUrzaTrio(f) {
		return condForm{Choose: "Count$UrzaLands", Body: "Count$UrzaLands.1.0", Cmp: "GE1"}, nil
	}
	if isYou(c.Ref) && f.Op == ir.FPred && f.Pred == ir.PControls && len(f.Args) == 1 && f.Ref == nil && f.Word == "" {
		// "you control a ...": Forge counts the battlefield matches of
		// the filter with YouCtrl added.
		own := &ir.Filter{Subject: ir.SubjPermanent, Op: ir.FPred, Pred: ir.PControlledBy, Ref: &ir.Ref{Kind: ir.RefYou}}
		present, err := p.valid(&ir.Filter{Op: ir.FAll, Args: []*ir.Filter{own, f.Args[0]}, Src: f.Src})
		if err != nil {
			return condForm{}, err
		}
		cmp := "GE1"
		if f.Cmp != ir.CmpNone {
			if cmp, err = p.cmpAgainst(f.Cmp, f.Value, c.Src); err != nil {
				return condForm{}, err
			}
		}
		return condForm{Present: present, Field: true, Cmp: cmp}, nil
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
		return p.svCond(body, cmp), nil
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

// printReceiptNonEmpty prints "the card you exiled was a ...": the test of the
// targeted object the previous step exiled.
func printReceiptNonEmpty(p *pr, c *ir.Cond) (condForm, error) {
	no := func() (condForm, error) { return condForm{}, unprintable("cond:receipt-non-empty", c.Src) }
	prev := p.prev
	if c.Filter == nil || prev == nil || c.Receipt != ir.RcExiled || prev.Verb != ir.VerbExile || prev.Obj == nil || prev.Obj.Kind != ir.RefTarget {
		return no()
	}
	present, err := p.valid(c.Filter)
	if err != nil {
		return condForm{}, err
	}
	return condForm{Present: present, Defined: "Targeted", Cmp: "EQ1"}, nil
}

// mergeParams folds an if/else whose branches differ in literal amounts into
// one node: each differing parameter becomes a count that picks between the
// two literals by c. Forge's own count heads (Morbid, Revolt) pick directly;
// others compare through Count$Compare. Parameters that differ must agree on
// the pair of magnitudes and keep one sign each ("-X" for a negative pair).
func (p *pr) mergeParams(c condForm, then, els *node, diff []string) bool {
	if len(diff) == 0 {
		return false
	}
	type pair struct{ a, b int }
	var want pair
	neg := make([]bool, len(diff))
	for i, k := range diff {
		av, _ := then.get(k)
		bv, _ := els.get(k)
		a, aerr := strconv.Atoi(av)
		b, berr := strconv.Atoi(bv)
		if aerr != nil || berr != nil || (a < 0) != (b < 0) {
			return false
		}
		if a < 0 {
			neg[i], a, b = true, -a, -b
		}
		if i == 0 {
			want = pair{a, b}
		} else if want != (pair{a, b}) {
			return false
		}
	}
	var v string
	if c.Choose != "" {
		v = p.countVarAs("X", c.Choose+"."+strconv.Itoa(want.a)+"."+strconv.Itoa(want.b))
	} else {
		c = p.plain(c)
		v = p.countVarAs("X", "Count$Compare "+c.SVar+" "+c.cmp()+"."+strconv.Itoa(want.a)+"."+strconv.Itoa(want.b))
	}
	for i, k := range diff {
		if neg[i] {
			then.set(k, "-"+v)
		} else {
			then.set(k, v)
		}
	}
	return true
}

// battlefieldCount renders "there are n or more X on the battlefield" as the
// IsPresent$/PresentCompare$ pair Forge puts on a static.
func (p *pr) battlefieldCount(c *ir.Cond) ([]kv, error) {
	if c.Kind != ir.CCountCompare || c.Filter == nil || c.Cmp == ir.CmpNone || !isConst(c.R) {
		return nil, unprintable("cond:"+c.Kind.String(), c.Src)
	}
	v, err := p.valid(c.Filter)
	if err != nil {
		return nil, err
	}
	return withCmp([]kv{{"IsPresent", v}}, "PresentCompare", cmpWord(c.Cmp)+itoa(c.R.N)), nil
}

// isUrzaTrio reports whether f is "you control the two other Urza's lands":
// two controls tests, each of the Urza's subtype and one of Mine, Power-Plant
// and Tower, naming two different ones.
func isUrzaTrio(f *ir.Filter) bool {
	if f == nil || f.Op != ir.FAll || len(f.Args) != 2 {
		return false
	}
	seen := map[string]bool{}
	for _, a := range f.Args {
		if a == nil || a.Op != ir.FPred || a.Pred != ir.PControls || len(a.Args) != 1 || a.Args[0] == nil {
			return false
		}
		g := a.Args[0]
		if g.Op != ir.FAll || len(g.Args) != 2 || g.Args[0].Pred != ir.PSubtype || g.Args[0].Word != "Urzas" || g.Args[1].Pred != ir.PSubtype {
			return false
		}
		w := g.Args[1].Word
		if w != "Mine" && w != "PowerPlant" && w != "Tower" {
			return false
		}
		seen[w] = true
	}
	return len(seen) == 2
}

// mergeBranches folds the two branches of an if/else into then when they
// differ only in literal amounts. A mana branch without an Amount$ makes one.
func (p *pr) mergeBranches(c condForm, then, els *node) bool {
	if diff, ok := sameParams(then, els); ok {
		return p.mergeParams(c, then, els, diff)
	}
	if then.API != "Mana" || els.API != "Mana" {
		return false
	}
	t2, e2 := &node{API: then.API, Params: append([]kv(nil), then.Params...)}, &node{API: els.API, Params: append([]kv(nil), els.Params...)}
	for _, n := range []*node{t2, e2} {
		if _, ok := n.get("Amount"); !ok {
			n.Params = append(n.Params, kv{"Amount", "1"})
		}
	}
	diff, ok := sameParams(t2, e2)
	if !ok || !p.mergeParams(c, t2, e2, diff) {
		return false
	}
	then.Params = t2.Params
	return true
}
