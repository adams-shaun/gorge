package kgen

import "strings"

// pumpAllEachChoice lowers a mass Pump (Defined$ Valid <creatures>) with
// KWChoice$ ("choose first strike, vigilance or lifelink; creatures you
// control gain it"): gorge effPump asks the controller one KModes decision
// over the candidate list first (the same ask pumpChoice lowers), then
// registers the one chosen keyword on every object of the set. Each branch
// is the full mass grant for one candidate. base is the already-built
// PumpAllEach for the set.
func (lc *lowerCtx) pumpAllEachChoice(ps *params, base lInstr, kwc string) error {
	if _, ok := ps.get("KW"); ok {
		return refuse("Pump with KW$ and KWChoice$")
	}
	choices := strings.Split(kwc, ",")
	kws := make([][]string, len(choices))
	for i, name := range choices {
		ks, err := pumpKeywords(strings.TrimSpace(name))
		if err != nil {
			return err
		}
		kws[i] = ks
	}
	lc.emit(lInstr{Op: "GenericChoice", Obj: lRef{Kind: rYou}, A: cst(len(choices))})
	for i, ks := range kws {
		bat := len(lc.ab.Prog)
		lc.emit(lInstr{Op: "Branch", C: i})
		in := base
		in.CS = strings.Join(ks, "&")
		lc.emit(in)
		lc.ab.Prog[bat].Skip = len(lc.ab.Prog) - bat - 1
	}
	return nil
}
