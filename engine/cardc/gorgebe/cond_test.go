package gorgebe

import (
	"errors"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

type condTestCase struct {
	name  string
	c     *ir.Cond
	frame bool
	want  condForm
	what  string // the refusal's What; "" when the condition prints
	svars []kv   // the SVars it adds, when it prints
}

func ctYou() *ir.Ref { return &ir.Ref{Kind: ir.RefYou} }

func ctPred(p ir.Pred) *ir.Filter { return &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: p} }

func ctCmpPred(p ir.Pred, op ir.CmpOp, n int32) *ir.Filter {
	f := ctPred(p)
	f.Cmp, f.Value = op, vc(n)
	return f
}

func ctOpp() *ir.Filter {
	f := ctPred(ir.POpponentOf)
	f.Ref = ctYou()
	return f
}

func ctRef(r *ir.Ref, f *ir.Filter) *ir.Cond {
	return &ir.Cond{Kind: ir.CRefMatches, Ref: r, Filter: f}
}

func ctNot(c *ir.Cond) *ir.Cond { return &ir.Cond{Kind: ir.CNot, Args: []*ir.Cond{c}} }

func condTestCases() []condTestCase {
	raid := ctRef(ctYou(), ctPred(ir.PAttackedThisTurn))
	thresh := ctRef(ctYou(), ctCmpPred(ir.PCardsInGraveyard, ir.CmpGE, 7))
	hell := ctRef(ctYou(), ctCmpPred(ir.PCardsInHand, ir.CmpEQ, 0))
	life := ctRef(ctYou(), ctCmpPred(ir.PLifeTotal, ir.CmpLE, 5))
	tlife := ctRef(&ir.Ref{Kind: ir.RefTarget}, ctCmpPred(ir.PLifeTotal, ir.CmpEQ, 10))
	low := &ir.Cond{Kind: ir.CExistsPlayer, Filter: &ir.Filter{Op: ir.FAll, Args: []*ir.Filter{ctOpp(), ctCmpPred(ir.PLifeTotal, ir.CmpLE, 5)}}}
	high := &ir.Cond{Kind: ir.CExistsPlayer, Filter: &ir.Filter{Op: ir.FAll, Args: []*ir.Filter{ctOpp(), ctCmpPred(ir.PLifeTotal, ir.CmpGE, 40)}}}
	kicked := &ir.Cond{Kind: ir.CKicked}
	you := &ir.Cond{Kind: ir.CIsTurnOf, Ref: ctYou()}
	opp := &ir.Cond{Kind: ir.CIsTurnOf, Filter: ctOpp()}
	cmpX := &ir.Cond{Kind: ir.CCompare, L: vX, Cmp: ir.CmpGE, R: vc(3)}
	return []condTestCase{
		{name: "raid", c: raid, want: condForm{SVar: "N", Cmp: "GE1"}, svars: []kv{{"N", "Count$AttackersDeclared"}}},
		{name: "threshold", c: thresh, want: condForm{Named: "Threshold", Body: "Count$ValidGraveyard Card.YouOwn", Choose: "Count$Threshold", Cmp: "GE7"}},
		{name: "hellbent", c: hell, want: condForm{Named: "Hellbent", Body: "Count$CardsInYourHand", Choose: "Count$Hellbent", Cmp: "EQ0"}},
		{name: "life", c: life, want: condForm{SVar: "N", Cmp: "LE5"}, svars: []kv{{"N", "Count$YourLifeTotal"}}},
		{name: "target life", c: tlife, frame: true, want: condForm{SVar: "N", Cmp: "EQ10"}, svars: []kv{{"N", "TargetedPlayer$LifeTotal"}}},
		{name: "target life without frame", c: tlife, what: "cond:ref-matches"},
		{name: "lowest opponent life", c: low, want: condForm{SVar: "N", Cmp: "LE5"}, svars: []kv{{"N", "PlayerCountOpponents$LowestLifeTotal"}}},
		{name: "highest opponent life", c: high, want: condForm{SVar: "N", Cmp: "GE40"}, svars: []kv{{"N", "PlayerCountOpponents$HighestLifeTotal"}}},
		{name: "not raid", c: ctNot(raid), want: condForm{SVar: "N", Cmp: "LT1"}, svars: []kv{{"N", "Count$AttackersDeclared"}}},
		{name: "not threshold", c: ctNot(thresh), want: condForm{SVar: "N", Cmp: "LT7"}, svars: []kv{{"N", "Count$ValidGraveyard Card.YouOwn"}}},
		{name: "not hellbent", c: ctNot(hell), want: condForm{SVar: "N", Cmp: "NE0"}, svars: []kv{{"N", "Count$CardsInYourHand"}}},
		{name: "not life", c: ctNot(life), want: condForm{SVar: "N", Cmp: "GE6"}, svars: []kv{{"N", "Count$YourLifeTotal"}}},
		{name: "not kicked", c: ctNot(kicked), want: condForm{SVar: "N", Cmp: "LT1"}, svars: []kv{{"N", "Count$Kicked.1.0"}}},
		{name: "not you turn", c: ctNot(you), want: condForm{Turn: "Opponent"}},
		{name: "not opponent turn", c: ctNot(opp), want: condForm{Turn: "You"}},
		{name: "kicked", c: kicked, want: condForm{SVar: "N", Cmp: "GE1"}, svars: []kv{{"N", "Count$Kicked.1.0"}}},
		{name: "your turn", c: you, want: condForm{Turn: "You"}},
		{name: "opponent turn", c: opp, want: condForm{Turn: "Opponent"}},
		{name: "compare", c: cmpX, want: condForm{SVar: "X", Cmp: "GE3"}, svars: []kv{{"X", "Count$xPaid"}}},
		{name: "compare const left", c: &ir.Cond{Kind: ir.CCompare, L: vc(1), Cmp: ir.CmpGE, R: vX}, what: "cond:compare"},
		{name: "cost paid", c: &ir.Cond{Kind: ir.CCostPaid}, what: "cond:cost-paid"},
		{name: "unknown", c: &ir.Cond{Kind: ir.CNthTurn}, what: "cond:nth-turn"},
		{name: "unknown filter", c: ctRef(ctYou(), ctPred(ir.PTapped)), what: "cond:ref-matches"},
		{name: "extra predicate", c: ctRef(ctYou(), &ir.Filter{Op: ir.FAll, Args: []*ir.Filter{ctPred(ir.PAttackedThisTurn), ctPred(ir.PTapped)}}), what: "cond:ref-matches"},
		{name: "exists other", c: &ir.Cond{Kind: ir.CExistsPlayer, Filter: ctPred(ir.PAny)}, what: "cond:exists-player"},
	}
}

func condTestRun(c condTestCase) (*pr, condForm, error) {
	p := &pr{}
	if c.frame {
		p.frames = []*frame{valueTestPlayerFrame()}
	}
	f, err := p.cond(c.c)
	return p, f, err
}

func TestCondForms(t *testing.T) {
	for _, c := range condTestCases() {
		p, f, err := condTestRun(c)
		if c.what != "" {
			var u *Unprintable
			if !errors.As(err, &u) || u.What != c.what {
				t.Errorf("%s: got %+v, %v; want Unprintable %s", c.name, f, err, c.what)
			}
			continue
		}
		if err != nil || f != c.want {
			t.Errorf("%s: got %+v, %v; want %+v", c.name, f, err, c.want)
		}
		if len(p.svars) != len(c.svars) || (len(c.svars) > 0 && !reflect.DeepEqual(p.svars, c.svars)) {
			t.Errorf("%s: svars %v, want %v", c.name, p.svars, c.svars)
		}
	}
}

func TestCondRenderings(t *testing.T) {
	type pair struct {
		abil, trig []kv
		abilErr    bool
	}
	cases := []struct {
		name string
		f    condForm
		want pair
	}{
		{"svar", condForm{SVar: "N", Cmp: "GE7"},
			pair{abil: []kv{{"ConditionCheckSVar", "N"}, {"ConditionSVarCompare", "GE7"}}, trig: []kv{{"CheckSVar", "N"}, {"SVarCompare", "GE7"}}}},
		{"svar default cmp", condForm{SVar: "N"},
			pair{abil: []kv{{"ConditionCheckSVar", "N"}, {"ConditionSVarCompare", "GE1"}}, trig: []kv{{"CheckSVar", "N"}}}},
		{"present self", condForm{Present: "Card.Untapped"},
			pair{abil: []kv{{"ConditionDefined", "Self"}, {"ConditionPresent", "Card.Untapped"}, {"ConditionCompare", "GE1"}},
				trig: []kv{{"IsPresent", "Card.Untapped+Self"}}}},
		{"present targeted", condForm{Present: "Card.Untapped", Defined: "Targeted"},
			pair{abil: []kv{{"ConditionDefined", "Targeted"}, {"ConditionPresent", "Card.Untapped"}, {"ConditionCompare", "GE1"}},
				trig: []kv{{"IsPresent", "Card.Untapped"}, {"PresentDefined", "Targeted"}}}},
		{"threshold", condForm{Named: "Threshold", Body: "Count$ValidGraveyard Card.YouOwn", Choose: "Count$Threshold", Cmp: "GE7"},
			pair{trig: []kv{{"Threshold", "True"}}, abilErr: true}},
		{"your turn", condForm{Turn: "You"},
			pair{abil: []kv{{"ConditionPlayerTurn", "True"}}, trig: []kv{{"PlayerTurn", "True"}}}},
		{"opponent turn", condForm{Turn: "Opponent"},
			pair{trig: []kv{{"OpponentTurn", "True"}}, abilErr: true}},
	}
	for _, c := range cases {
		if got, err := c.f.onStatic(); c.name == "threshold" && (err != nil || !reflect.DeepEqual(got, []kv{{"Condition", "Threshold"}})) {
			t.Errorf("threshold: onStatic %v, %v", got, err)
		}
		if got, err := c.f.onActivation(); c.name == "threshold" && (err != nil || !reflect.DeepEqual(got, []kv{{"Activation", "Threshold"}})) {
			t.Errorf("threshold: onActivation %v, %v", got, err)
		}
		a, aerr := c.f.onAbility()
		if c.want.abilErr {
			var u *Unprintable
			if !errors.As(aerr, &u) {
				t.Errorf("%s: onAbility err %v, want Unprintable", c.name, aerr)
			}
		} else if aerr != nil || !reflect.DeepEqual(a, c.want.abil) {
			t.Errorf("%s: onAbility %v, %v; want %v", c.name, a, aerr, c.want.abil)
		}
		tr, terr := c.f.onTrigger()
		if terr != nil || !reflect.DeepEqual(tr, c.want.trig) {
			t.Errorf("%s: onTrigger %v, %v; want %v", c.name, tr, terr, c.want.trig)
		}
	}
}
