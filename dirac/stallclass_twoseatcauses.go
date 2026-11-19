package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The classes of TwoSeatCauses: each names one reading the four-seat build
// keeps for two seats only, so a game of three or more seats on a table
// holding such a card stops at its start (StallTwoSeat). The Commander pod
// census (dirac/oracle TestCommanderPodTwoSeatCensus) ranks them by the pods
// they block.
const (
	TwoSeatKgen       = "kgen"        // kop.CardTwoSeat: the lowering reads a two-player identity (kgen lowerCtx.twoSeat, or a kop.TwoSeatInstr operand)
	TwoSeatSpeed      = "speed"       // OpChangeSpeed or Start your engines: genState.speed packs two seats (kw_speed.go)
	TwoSeatSkipUntap  = "skip-untap"  // OpSkipUntap: perm.cseen holds two seats' marks (act_skipuntap.go)
	TwoSeatBattle     = "battle"      // TBattle: the protector is the one opponent (face_battle.go; CR 310.10)
	TwoSeatGoadStatic = "goad-static" // StatGoad: a static goad has no goader seat (act_goad.go prepGoad)
	TwoSeatCtrAdded   = "ctr-added"   // a counters-added-this-turn condition on the card itself: ctrAdd.e[].fl packs two seats (ctr_addedthisturn.go)
)

// TwoSeatCauses is, per CardID of t, the TwoSeat* classes that make the card
// two-seat only, in the order above; nil for a card a pod can hold. It reads
// what prepTwoSeat, prepGoad and prepCtrAdded read, and TwoSeatOnly holds
// exactly when some card has a cause (TestTwoSeatCausesArePrep).
func TwoSeatCauses(t *kop.Table) [][]string {
	out := make([][]string, len(t.Cards))
	anyCtr := false
	for i := range t.Exprs {
		anyCtr = anyCtr || t.Exprs[i].Kind == kop.ExCtrAdded
	}
	for c := range t.Cards {
		cd := &t.Cards[c]
		if cardTwoSeat(cd) {
			out[c] = append(out[c], TwoSeatKgen)
		}
		if cardSpeed(cd) {
			out[c] = append(out[c], TwoSeatSpeed)
		}
		if skipUntapCard(cd) {
			out[c] = append(out[c], TwoSeatSkipUntap)
		}
		if cd.Types&kop.TBattle != 0 {
			out[c] = append(out[c], TwoSeatBattle)
		}
		goad, ctr := false, false
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			goad = goad || a.Kind == kop.AbStatic && a.Static.Kind == kop.StatGoad
			ctr = ctr || anyCtr && (condReadsCtrSelf(t, a.Trig.Cond) || condReadsCtrSelf(t, a.Static.Cond2) || condReadsCtrSelf(t, a.Repl.Cond))
		}
		if goad {
			out[c] = append(out[c], TwoSeatGoadStatic)
		}
		if ctr {
			out[c] = append(out[c], TwoSeatCtrAdded)
		}
	}
	return out
}

// cardSpeed reports whether c reads or changes speed (prepSpeed's per-card test).
func cardSpeed(c *kop.CardDef) bool {
	if c.Engines {
		return true
	}
	for j := range c.Abilities {
		for _, in := range c.Abilities[j].Prog {
			if in.Op == kop.OpChangeSpeed {
				return true
			}
		}
	}
	return false
}

// TwoSeatOnly reports whether a game of three or more seats on t stops at its
// start with StallTwoSeat: the kernel's own table preparation, read whole.
func TwoSeatOnly(t *kop.Table) bool { return prepare(t).gen.twoSeatOnly }
