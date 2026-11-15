package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// lower_maxspeedrepl.go: a max-speed static's AddReplacementEffect$ grant to the
// source itself (Far Fortune, End Boss). gorge's
// maxSpeedAbilities adds the replacement to the source while the
// controller's speed is 4, so the replacement lowers as the ordinary R: line
// with the speed gate ANDed into its Cond. Only the plus-N damage
// replacement has a kernel reader that evaluates Repl.Cond (replPlus).
func (lc *lowerCtx) maxSpeedGrantedRepl(sp *params, name string, gy bool) error {
	if gy {
		return refuse("MaxSpeed granted replacement from the graveyard")
	}
	if left := sp.unread(); len(left) > 0 {
		return refuse("MaxSpeed static unread params %v", left)
	}
	if _, ok := sp.m["AddAbility"]; ok {
		return refuse("MaxSpeed granted replacement %s beside AddAbility$", name)
	}
	for _, k := range []string{"AddTrigger", "AddStaticAbility"} {
		if _, ok := sp.m[k]; ok {
			return refuse("MaxSpeed granted replacement %s beside %s$", name, k)
		}
	}
	r, ok := cards.ParseReplacementLine(lc.face.SVars[name])
	if !ok {
		return refuse("MaxSpeed granted replacement %s", name)
	}
	r.With = cards.ResolveSVar(lc.face.SVars, r.Params["ReplaceWith"]) // link.go does this for a printed R: line
	idx := len(lc.out.Repls)
	if err := lc.lowerRepl(r); err != nil {
		return err
	}
	if len(lc.out.Repls) != idx+1 || (lc.out.Repls[idx].Kind != "DamagePlus" && lc.out.Repls[idx].Kind != "DrawMore") || lc.out.Repls[idx].Cond.Kind != cNone {
		return refuse("MaxSpeed granted replacement %s", name)
	}
	lc.out.Repls[idx].Cond = lCond{Kind: cValue, A: lValue{Kind: vYourSpeed}, Op: opGE, B: cst(4)}
	return nil
}
