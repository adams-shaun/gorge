package effects

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// dice_rollmodify.go is the "After you roll a die, you may pay 1 life. If you
// do, increase or decrease the result by 1. Do this only once each turn."
// keyword (Night Shift of the Living Dead; CR 706.2: a modifier applies to the
// natural result and may be optional and carry a cost). Only the fixed
// PayLife<N> cost shape is modelled: a carrier whose RollModifyCost SVar is
// anything else (Xenosquirrels' SubCounter<1/P1P1>) is never offered, and its
// keyword token stays unregistered.

// rollModifyKeyword is the printed keyword line, which is its own head.
const rollModifyKeyword = "After you roll a die, you may pay 1 life. If you do, increase or decrease the result by 1. Do this only once each turn."

func init() { RegisterNonAPI("kw:" + rollModifyKeyword) }

// Svar names of the keyword's per-card parameters.
const (
	rollModsThisTurnSVar = "ModsThisTurn"
	rollModsLimitSVar    = "RollModificationsLimit"
	rollModsCostSVar     = "RollModifyCost"
)

// modifyRolledDie offers every roll-modify carrier the roller controls the
// chance to adjust one die's result, in battlefield order, each seeing the
// result the previous one left, and returns the final result. It emits
// nothing until a carrier's answer is in hand (AskTape may unwind and
// re-execute the resolution from its checkpoint), and a die no carrier offers
// for returns result unchanged without posing anything.
func modifyRolledDie(h Host, c *Ctx, result int32) int32 {
	for _, id := range h.Game().Zone(state.ZBattlefield, c.Controller) {
		o := h.Game().Obj(id)
		if o == nil || !PrintedHasKeyword(o, rollModifyKeyword) {
			continue
		}
		used, ok := rollModifyEligible(h, o, c.Controller)
		if !ok {
			continue
		}
		d := RollModifyAsk(c.Controller, id, result)
		ans, asked := AskTape(h, d)
		if !asked || len(ans) == 0 {
			continue
		}
		var delta int32
		switch ans[0].Index {
		case 1:
			delta = 1
		case 2:
			delta = -1
		default:
			continue
		}
		if !h.PayRollModifyCost(c.Controller, o.Face().SVars[rollModsCostSVar]) {
			continue
		}
		result += delta
		h.Emit(events.Event{Kind: events.StoreSVar, Obj: id, Text: rollModsThisTurnSVar, Amount: used + 1})
	}
	return result
}

// RollModifyAsk is the roll-modify offer: a choose-one with the unmodified
// result first, so the engine's clamp fallback and a bot answering option 0
// keep the die as rolled.
func RollModifyAsk(player state.PlayerID, source state.ObjID, result int32) *decision.Decision {
	return &decision.Decision{Player: player, Kind: decision.KChoose, Min: 1, Max: 1, Source: source,
		Prompt: "Modify the die result?",
		Options: []decision.Option{
			{Index: 0, Kind: "keep", Label: "Keep the result (" + strconv.FormatInt(int64(result), 10) + ")", Player: player},
			{Index: 1, Kind: "increase", Label: "Pay 1 life: increase the result by 1", Player: player},
			{Index: 2, Kind: "decrease", Label: "Pay 1 life: decrease the result by 1", Player: player},
		}}
}

// rollModifyEligible reports whether carrier o may modify a roll by p now: the
// once-per-turn count is under the printed limit ("None" is unlimited) and
// the cost is payable. used is the count so far this turn.
func rollModifyEligible(h Host, o *state.Object, p state.PlayerID) (used int32, ok bool) {
	f := o.Face()
	used = o.RuntimeSVars[rollModsThisTurnSVar]
	if limit := strings.TrimSpace(f.SVars[rollModsLimitSVar]); !strings.EqualFold(limit, "None") {
		n, err := strconv.Atoi(limit)
		if err != nil || used >= int32(n) {
			return used, false
		}
	}
	return used, h.RollModifyCostPayable(p, f.SVars[rollModsCostSVar])
}
