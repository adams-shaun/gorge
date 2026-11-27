package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// rollPlanarDiceRepl lowers the planar-dice replacement class (gorge
// replacement_match.go's ReplRollPlanarDice arm): "if you would roll one or
// more planar dice, instead roll that many planar dice plus one and ignore
// one" (Ichor Elixir, the corpus's only carrier). ActiveZones$ Battlefield
// and ValidPlayer$ You are the scope the kernel's roll scan reads; the
// ReplaceEffect chain's two ReplaceCount$ bodies are the plus count and the
// ignored count (gorge's ReplaceEvent PlanarRoll arm). Every other planar
// roll replacement shape refuses.
func (lc *lowerCtx) rollPlanarDiceRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ActiveZones") != "Battlefield" || rp.str("ValidPlayer") != "You" {
		return refuse("repl-hard: RollPlanarDice replacement")
	}
	if w == nil || w.Sub == nil || w.Sub.Sub != nil {
		return refuse("repl-hard: RollPlanarDice replacement")
	}
	plus, err := lc.planarReplCount(w, "Number")
	if err != nil {
		return err
	}
	ignore, err := lc.planarReplCount(w.Sub, "Ignore")
	if err != nil {
		return err
	}
	rl.Kind, rl.Plus, rl.Ignore = "RollPlanarDice", plus, ignore
	return nil
}

// planarReplCount reads one ReplaceEffect body of the chain: VarName$ is the
// variable it rewrites, and VarValue$ is ReplaceCount$<same>/Plus.N, either
// spelled directly or through an SVar holding that text. N is the count the
// body adds.
func (lc *lowerCtx) planarReplCount(w *cards.SA, varName string) (int, error) {
	if w == nil || w.API != "ReplaceEffect" {
		return 0, refuse("repl-hard: RollPlanarDice replacement")
	}
	bp := newParams(w.Params)
	bp.consume("VarName", "VarValue", "SubAbility")
	if bp.str("VarName") != varName {
		return 0, refuse("repl-hard: RollPlanarDice replacement")
	}
	if left := bp.unread(); len(left) > 0 {
		return 0, refuse("repl-hard: RollPlanarDice replacement params %v", left)
	}
	v := strings.TrimSpace(lc.face.SVars[bp.m["VarValue"]])
	if v == "" {
		v = bp.str("VarValue")
	}
	prefix := "ReplaceCount$" + varName + "/Plus."
	n, err := strconv.Atoi(strings.TrimPrefix(v, prefix))
	if err != nil || n <= 0 || n > 1<<14 || !strings.HasPrefix(v, prefix) {
		return 0, refuse("repl-hard: RollPlanarDice replacement %q", v)
	}
	return n, nil
}
