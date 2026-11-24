package gorgebe

import "github.com/adams-shaun/gorge/engine/cardc/ir"

func init() {
	registerStep(ir.VerbDraw, playerAmount("Draw", "NumCards"))
	registerStep(ir.VerbGainLife, playerAmount("GainLife", "LifeAmount"))
	registerStep(ir.VerbLoseLife, playerAmount("LoseLife", "LifeAmount"))
	registerStep(ir.VerbMill, playerAmount("Mill", "NumCards"))
	registerStep(ir.VerbSurveil, playerAmount("Surveil", "Amount"))
	registerStep(ir.VerbExtraTurn, printExtraTurn)
	registerStep(ir.VerbEndTurn, printEndTurn)
	registerStep(ir.VerbDiscard, printDiscard)
}

// who puts st.Who on n as Defined$ (declaring a target player there) and
// applies st.Optional: OptionalDecider$ You for the controller, else refused.
func (p *pr) who(n *node, st *ir.Step) error {
	if st.Who != nil {
		if err := p.operand(n, "Defined", st.Who); err != nil {
			return err
		}
	}
	return optionalDecider(n, st)
}

func optionalDecider(n *node, st *ir.Step) error {
	if !st.Optional {
		return nil
	}
	if st.Who != nil {
		return unprintable("optional:"+st.Verb.String(), st.Src)
	}
	n.set("OptionalDecider", "You")
	return nil
}

// playerAmount prints a player-acting verb with one amount parameter.
func playerAmount(api, key string) stepFunc {
	return func(p *pr, steps []*ir.Step) (*node, int, error) {
		st := steps[0]
		amt, err := p.amount(st.N)
		if err != nil {
			return nil, 0, err
		}
		n := &node{API: api, Params: []kv{{key, amt}}}
		if err := p.who(n, st); err != nil {
			return nil, 0, err
		}
		return n, 1, nil
	}
}

func printExtraTurn(p *pr, steps []*ir.Step) (*node, int, error) {
	n := &node{API: "AddTurn", Params: []kv{{"NumTurns", "1"}}}
	if err := p.who(n, steps[0]); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

func printEndTurn(p *pr, steps []*ir.Step) (*node, int, error) {
	n := &node{API: "EndTurn"}
	if err := optionalDecider(n, steps[0]); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

// isHandFilter reports whether f selects cards in the hand and nothing more.
func isHandFilter(f *ir.Filter) bool {
	if f == nil || f.Zones != ir.MakeZoneSet(ir.ZoneHand) {
		return false
	}
	return f.Op == ir.FNone || (f.Op == ir.FPred && f.Pred == ir.PAny)
}

func printDiscard(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	o := st.Obj
	bad := unprintable("verb:discard", st.Src)
	if o == nil || !isHandFilter(o.Filter) {
		return nil, 0, bad
	}
	n := &node{API: "Discard"}
	switch {
	case o.Kind == ir.RefChoose && o.Of == nil:
		amt, err := p.amount(o.N)
		if err != nil {
			return nil, 0, err
		}
		n.Params = []kv{{"NumCards", amt}, {"Mode", "TgtChoose"}}
	case o.Kind == ir.RefAll:
		n.Params = []kv{{"Mode", "Hand"}}
	default:
		return nil, 0, bad
	}
	if err := p.who(n, st); err != nil {
		return nil, 0, err
	}
	return n, 1, nil
}

func init() { registerStep(ir.VerbChooseNumber, printChooseNumber) }

// printChooseNumber prints ChooseNumber$ for an unbounded "choose a number".
func printChooseNumber(p *pr, steps []*ir.Step) (*node, int, error) {
	st := steps[0]
	if st.Optional || st.Who != nil || (st.N != nil && !(st.N.Kind == ir.VConst && st.N.N == 0)) {
		return nil, 0, unprintable("verb:choose-number", st.Src)
	}
	return &node{API: "ChooseNumber", Params: []kv{{"Defined", "You"}}}, 1, nil
}
