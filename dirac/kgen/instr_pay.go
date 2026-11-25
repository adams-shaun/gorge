package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for the unless-paid and if-paid payment gates (instr.go).
func init() {
	instrOp("UnlessPaid", (*instrCtx).opUnlessPaid)
	instrOp("IfPaid", (*instrCtx).opUnlessPaid)
	instrOp("UnlessSwitched", (*instrCtx).opUnlessPaid)
}

func (ic *instrCtx) opUnlessPaid() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpUnlessPaid
	if in.Op == "IfPaid" {
		ki.Op = kop.OpIfPaid
	} else if in.Op == "UnlessSwitched" {
		ki.Op = kop.OpUnlessSwitched
	}
	pk := map[string]int{"Mana": kop.PayMana, "Discard": kop.PayDiscard, "DiscardRandom": kop.PayDiscardRandom, "DiscardHand": kop.PayDiscardHand, "Sacrifice": kop.PaySac, "SacrificeRiders": kop.PaySac, "Blight": kop.PaySac, "PayLife": kop.PayLife, "PayEnergy": kop.PayEnergy, "Return": kop.PayReturn, "Forage": kop.PayForage, "SacrificeSelf": kop.PaySacSelf, "SacrificeSelfMust": kop.PaySacSelf, "SacrificeHost": kop.PaySacSelf, "Draw": kop.PayDraw, "DamageYou": kop.PayDraw, "Mill": kop.PayDraw, "SubCounter": kop.PaySubCounter, "TapX": kop.PayTapX, "TapXVar": kop.PayTapX, "Waterbend": kop.PayTapX, "AddCounter": kop.PayAddCounter, "ExileGrave": kop.PayExileGrave, "ExileTrigNew": kop.PayExileGrave, "ExileTrigNewEvidence": kop.PayExileGrave, "EvidenceX": kop.PayManaX, "ExileGraveChain": kop.PayExileGrave, "ExileStackSelf": kop.PayExileGrave, "ExileHand": kop.PayExileGrave, "MoveGrave": kop.PayExileGrave, "ManaX": kop.PayManaX, "Evidence": kop.PayManaX, "Reveal": kop.PayDiscard, "Behold": kop.PayDiscard}[in.CS]
	if pk == 0 {
		return fmt.Errorf("pay kind %q", in.CS)
	}
	if in.Skip < 0 || in.Skip > 2047 {
		return fmt.Errorf("pay skip %d", in.Skip)
	}
	ki.C = int16(pk | in.Skip<<4)
	if in.CS == "AddCounter" {
		// PayAddCounter is not a C-nibble kind (it is appended past the
		// 15 the nibble holds): the instruction rides PayTapX with B.N
		// packing the counter kind and the recipient (kop.PayAddCounterB).
		// The kernel widens it to its own pay kind
		// (cost_unlessaddcounter.go).
		r, err := cb.ref(in.Obj2)
		if err != nil {
			return err
		}
		n, ok := kop.PayAddCounterB(kop.CounterKind(in.B.N), r)
		if !ok {
			return fmt.Errorf("AddCounter recipient ref %d", r)
		}
		ki.C = int16(kop.PayTapX | in.Skip<<4)
		ki.B = kop.Const(n)
	}
	if in.CS == "SacrificeRiders" {
		ki.B = kop.Const(int(in.B.N)) // the packed life and draw riders (lower_unlesschain.go)
	}
	if in.CS == "Blight" {
		ki.B = kop.Const(kop.PayBSubBlight) // a Blight, not a sacrifice
	}
	if in.CS == "Evidence" {
		ki.B = kop.Const(kop.PayBSubEvidence) // CollectEvidence, not {X}: PayEvidence's nibble value is PayManaX's
	}
	if in.CS == "EvidenceX" {
		ki.B = kop.Const(kop.PayBSubEvidenceX) // the announced X, not {X} (cost_collectevidencex.go)
	}
	if in.CS == "ExileTrigNewEvidence" {
		ki.B = kop.Const(kop.PayBSubTrigNewEvidence) // the triggered card, then evidence A (cost_collectevidencex.go)
	}
	if in.CS == "SacrificeHost" {
		ki.B = kop.Const(kop.PayBSubSacHost) // the granting host, not the source
	}
	if in.CS == "SacrificeSelfMust" {
		ki.B = kop.Const(kop.PayBSubSacSelfMust) // Cost$ Mandatory: no pay/decline ask (cost_mandatorysac.go)
	}
	if in.CS == "ExileTrigNew" {
		ki.B = kop.Const(kop.PayBSubTrigNew) // ExileAnyGrave<1/Card.TriggeredNewCard>, not a filter exile
	}
	if in.CS == "ExileGraveChain" {
		ki.B = kop.Const(kop.PayBSubChain) // the exile half of a mana-and-exile cost (cost_exilepool.go)
	}
	if in.CS == "ExileStackSelf" {
		ki.B = kop.Const(kop.PayBSubStackSelf) // the resolving spell too, from the stack
	}
	if in.CS == "DamageYou" {
		ki.B = kop.Const(kop.PayBSubDamage) // the resolving source deals A damage to the payer, not a draw (cost_restokens.go)
	}
	if in.CS == "Mill" {
		ki.B = kop.Const(kop.PayBSubMill) // the payer mills A cards from their own library's top, not a draw (cost_unlessmill.go)
	}
	if in.CS == "TapXVar" {
		ki.B = kop.Const(kop.PayBSubTapXVar) // tap any number, X the count (cost_tapxvar.go)
	}
	if in.CS == "Waterbend" {
		ki.B = kop.Const(kop.PayBSubWaterbend) // an unless Waterbend<N>, not a tapXType (cost_unlesswaterbend.go)
	}
	if in.CS == "ExileHand" {
		ki.B = kop.Const(kop.PayBSubExileHand) // ExileFromHand<N/Card>, not a graveyard exile
	}
	if in.CS == "MoveGrave" {
		ki.B = kop.Const(kop.PayBSubMoveGrave) // ExiledMoveToGrave<N/Spec>: exile to the owners' graveyards (cost_exiledmovegrave.go)
	}
	if in.CS == "Reveal" {
		ki.B = kop.Const(kop.PayBReveal) // a reveal, not a discard: nothing moves
	}
	if in.CS == "Behold" {
		ki.B = kop.Const(kop.PayBBehold) // a behold, not a discard: nothing moves
	}
	if pk == kop.PaySac && strings.HasPrefix(in.Note, noteRememberSacrificed) {
		// a trigger's own IfPaid Sacrifice whose characteristic a value
		// head reads: B carries the remember bit and the sum bit
		// (kernel sacRemember's IfPaid arm)
		ki.B = kop.Const(1 + remLKBits(in.Note))
	}
	z := kop.ZoneBattlefield
	if pk == kop.PayExileGrave {
		z = kop.ZoneGraveyard
	}
	if in.CS == "ExileHand" {
		z = kop.ZoneHand
	}
	if in.CS == "Reveal" {
		z = kop.ZoneHand
	}
	if in.CS == "MoveGrave" {
		z = kop.ZoneExile
	}
	if err := ic.objFilter(z); err != nil {
		return err
	}
	if pk == kop.PayMana && (in.C > 0 || len(in.Hyb) != 0) {
		// PayMana's F is the life paid beside the mana (a trigger's own
		// mana-and-life cost, Zoraline) and its hybrid pips (Emiel the
		// Blessed's {G/W}; kop.PayManaF)
		f, ok := payManaF(in.C, in.Hyb) // lower_trighybrid.go
		if in.F != nil || !ok {
			return fmt.Errorf("mana-and-life pay shape")
		}
		ki.F = f
	}
	return nil
}
