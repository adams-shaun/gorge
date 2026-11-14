package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The api: effects for designations, speed, connive, class levels, doors and ward (lower_api.go).
func init() {
	apiPS("TakeInitiative", (*lowerCtx).takeInitiative) // lower_initiative.go
	apiPS("BecomeMonarch", (*lowerCtx).becomeMonarch)   // lower_monarch.go
	apiOp("Ward", (*lowerCtx).apiWard)
	apiPS("ChangeSpeed", (*lowerCtx).changeSpeed) // lower_speed.go
	apiPS("Connive", (*lowerCtx).connive)
	apiPS("ClassLevelUp", (*lowerCtx).classLevelUp)
	apiPS("UnlockDoor", (*lowerCtx).unlockDoor) // lower_unlockdoor.go
}

func (lc *lowerCtx) apiWard(sa *cards.SA, ps *params) error {
	uc := ps.str("UnlessCost")
	if m := wardEvidenceRe.FindStringSubmatch(uc); m != nil {
		// Ward—Collect evidence N (Axebane Ferox; gorge kw_ward.go
		// beginWardPayment's CollectEvidence arm: after "Pay", a
		// KChoose of 1..all of the payer's graveyard cards, zone
		// order; short of N mana value the ward counters)
		n, _ := strconv.Atoi(m[1])
		lc.emit(lInstr{Op: "Ward", CS: "CollectEvidence", A: cst(n), Obj: lRef{Kind: rTriggered}})
		return nil
	}
	if m := wardDiscManaRe.FindStringSubmatch(uc); m != nil {
		// Ward—discard a card or pay {N} (Titania, Rugged Rumbler; gorge
		// rules ward.go beginWardPayment's ">:" arm: after "Pay", one
		// KChoose of a hand card to discard or {N}, the {N} paid through
		// the ward mana window)
		n, _ := strconv.Atoi(m[1])
		lc.emit(lInstr{Op: "Ward", CS: "DiscardOrMana", A: cst(n), Obj: lRef{Kind: rTriggered}})
		return nil
	}
	if strings.HasPrefix(uc, "PayLife<X/") {
		// Ward—pay life equal to its power (Raubahn; gorge kw_ward.go
		// beginWardPayment: any PayLife<X/…> is life = the ward
		// permanent's power at payment, floored at 0, whatever X says)
		if x := lc.face.SVars["X"]; x != "Count$CardPower" {
			return refuse("ward life cost X %q", x)
		}
		lc.emit(lInstr{Op: "Ward", CS: "PayLifePower", A: cst(0), Obj: lRef{Kind: rTriggered}})
		return nil
	}
	if wi, ok, err := lc.wardPoison(uc); err != nil { // lower_costtokens.go
		return err
	} else if ok {
		lc.emit(wi)
		return nil
	}
	lc.wbOK = true
	cost, err := lc.cost(uc)
	lc.wbOK = false
	if err != nil {
		return err
	}
	if cost.Energy != 0 || cost.Return != nil {
		return refuse("ward energy / return cost")
	}
	lc.wardCost = true
	in, err := lc.payInstr(cost, lRef{Kind: rTriggeredPlayer}, false)
	lc.wardCost = false
	if err != nil {
		return err
	}
	if in.CS == "Blight" {
		return refuse("ward blight cost")
	}
	in.Op = "Ward"
	in.Obj = lRef{Kind: rTriggered}
	lc.emit(in)
	return nil
}
