package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A RaiseCost static whose Cost$ is a literal PayLife<N> ("Spells your
// opponents cast that target this creature cost an additional 3 life to
// cast": Terror of the Peaks; CR 601.2f/h, 119.4). gorge folds the static's
// Cost$ into the cast's price (composeRaiseCost -> raiseExtraFor: the life
// rides costMods.RaiseLife beside the generic raise), once a chosen target
// matches a ValidTarget$ (costTargetsMatch). The lowering keeps the static a
// StatIncreaseCost with StRaiseLife: Static.A is then life, not generic mana
// (kernel cost_raiselife.go).

func init() { mask[kop.StaticFlags]("kop.StRaiseLife", kop.StRaiseLife) }

// raiseLifeN reads a Cost$ that is exactly PayLife<N>, N a positive literal
// that fits a byte.
func raiseLifeN(cost string) (int, bool) {
	rest, ok := strings.CutPrefix(strings.TrimSpace(cost), "PayLife<")
	if !ok {
		return 0, false
	}
	rest, ok = strings.CutSuffix(rest, ">")
	n, err := strconv.Atoi(rest)
	if !ok || err != nil || n < 1 || n > 255 {
		return 0, false
	}
	return n, true
}

// raiseLife lowers a PayLife<N> Cost$ into st (Amount N, flag "RaiseLife");
// false when the static's Cost$ is not that shape. Amount$ beside it is gorge's
// "that cost, Amount times", which the kernel does not price.
func (lc *lowerCtx) raiseLife(sp *params, st *lStatic) (bool, error) {
	cost, ok := sp.m["Cost"]
	if !ok {
		return false, nil
	}
	n, ok := raiseLifeN(cost)
	if !ok {
		return false, refuse("RaiseCost Cost$ %q", strings.TrimSpace(cost))
	}
	if _, has := sp.m["Amount"]; has {
		return false, refuse("RaiseCost Cost$ %q with an Amount$", strings.TrimSpace(cost))
	}
	if sp.str("Type") != "Spell" || sp.str("EffectZone") != "" {
		return false, refuse("RaiseCost Cost$ %q: not a spell raise", strings.TrimSpace(cost))
	}
	sp.consume("Cost")
	var err error
	if st.Amount, err = lc.value(strconv.Itoa(n)); err != nil {
		return false, err
	}
	st.Flags = append(st.Flags, "RaiseLife")
	return true, nil
}
