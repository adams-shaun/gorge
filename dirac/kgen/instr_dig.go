package kgen

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for dig, peek and card choices (instr.go).
func init() {
	instrOp("Dig", (*instrCtx).opDig)
	instrOp("DigUntil", (*instrCtx).opDigUntil)
	instrOp("Peek", (*instrCtx).opPeek)
	instrOp("TwoPiles", (*instrCtx).opTwoPiles)
	instrOp("ChooseCard", (*instrCtx).opChooseCard)
	instrOp("ChooseEachType", (*instrCtx).opChooseEachType)
	instrOp("DigBottom", (*instrCtx).opDigBottom)
	instrOp("RemBottom", (*instrCtx).opRemBottom)
	instrOp("DigImprint", (*instrCtx).opDigImprint)
}

func (ic *instrCtx) opDig() error {
	ki := &ic.ki
	in := &ic.in
	fl := ic.fl
	ki.Op = kop.OpDigEx
	rest := zoneNames[fl["rest"]]
	df := int(zoneMap[rest])
	for k, b := range map[string]int{"Optional": kop.DigOptional, "RestRandomOrder": kop.DigRandomOrder, "Reveal": kop.DigReveal,
		"RememberChanged": kop.DigRemember, "ForceRevealToController": kop.DigRevealToCtrl, "Tapped": kop.DigTapped, "RestTop": kop.DigRestTop, "RestTopAsk": kop.DigRestTopAsk, "ExileFaceDown": kop.DigFaceDown, "ExilerOther": kop.DigExilerOther} {
		if _, ok := fl[k]; ok {
			df |= b
		}
	}
	if p, ok := fl["pos"]; ok {
		switch {
		case p == "0" && in.C == zLibrary:
			df |= kop.DigTakeToTop
		case p == "-1" && rest == zLibrary:
			// the rest to the bottom: the default
		default:
			return fmt.Errorf("dig position %s", p)
		}
	}
	ki.C = int16(int(zoneMap[in.C]) | df<<3)
	if err := ic.objFilter(kop.ZoneLibrary); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opDigUntil() error {
	ki := &ic.ki
	in := &ic.in
	fl := ic.fl
	ki.Op = kop.OpDigUntil
	rev := zoneNames[fl["revealed"]]
	df := int(zoneMap[rev])
	if p, ok := fl["pos"]; ok {
		switch p {
		case "0":
			// RevealedLibraryPosition$ 0/absent is the stay-in-place default
			// (gorge effDigUntil: no move of the revealed rest)
		case "-1":
			df |= kop.DuRestBottom
		default:
			return fmt.Errorf("dig-until position %s", p)
		}
	}
	if _, ok := fl["RandomOrder"]; ok {
		df |= kop.DuRandomOrder
	}
	if _, ok := fl["FoundWithRevealed"]; ok {
		df |= kop.DuFoundWithRevealed
	}
	if _, ok := fl["Tapped"]; ok {
		df |= kop.DuFoundTapped
	}
	if _, ok := fl["Attacking"]; ok {
		df |= kop.DuFoundAttacking
	}
	if _, ok := fl["RememberFound"]; ok {
		df |= kop.DuRememberFound
	}
	if _, ok := fl["DugMark"]; ok {
		df |= kop.DuDugMark
	}
	if _, ok := fl["RememberRevealed"]; ok {
		df |= kop.DuRememberRevealed
	}
	if _, ok := fl["ImprintFound"]; ok {
		df |= kop.DuImprintFound
	}
	if _, ok := fl["MinTotalCMC"]; ok {
		df |= kop.DuMinTotalCMC
	}
	ki.C = int16(int(zoneMap[in.C]) | df<<3)
	if err := ic.objFilter(kop.ZoneLibrary); err != nil {
		return err
	}
	return nil
}

func (ic *instrCtx) opPeek() error {
	ki := &ic.ki
	fl := ic.fl
	ki.Op = kop.OpPeek
	if _, ok := fl["MayReveal"]; ok {
		ki.C = int16(kop.PeekMayReveal)
		if err := ic.objFilter(kop.ZoneLibrary); err != nil {
			return err
		}
	}
	if _, ok := fl["Hand"]; ok {
		// peekHand always remembers (act_peekreveal.go peekHand), so the
		// Remember$ intent rides on PeekHand itself, as the Public arm's
		// PeekRemember does. Each arm assigns ki.C outright; refuse a Hand
		// beside any other arm rather than silently dropping the other
		// intent, and refuse a remember-less hand peek (it would silently
		// lower to the remembering runtime).
		if _, ok := fl["MayReveal"]; ok {
			return fmt.Errorf("peek hand with MayReveal$")
		}
		if _, ok := fl["Public"]; ok {
			return fmt.Errorf("peek hand with Public$")
		}
		if _, ok := fl["Remember"]; !ok {
			return fmt.Errorf("peek hand without Remember$")
		}
		ki.C = int16(kop.PeekHand) // act_peekreveal.go peekHand
	}
	if _, ok := fl["Public"]; ok {
		ki.C = int16(kop.PeekPublic)
		if _, ok := fl["Remember"]; ok {
			ki.C |= int16(kop.PeekRemember)
		}
	}
	return nil
}

func (ic *instrCtx) opTwoPiles() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpTwoPiles, int16(zoneMap[in.C])
	return nil
}

func (ic *instrCtx) opChooseCard() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpChooseCard
	if err := ic.objFilter(zoneMap[in.C]); err != nil {
		return err
	}
	switch in.CS {
	case "Optional":
		ki.C = 1 // up to one: the ask carries a Done
	case "Remember":
		ki.C = 2 // RememberChosen$: the chosen card joins the remembered set
	case "LibraryUpTo":
		ki.C = 3 // up to B picks among the top A library cards
	case "AttachPick":
		ki.C = 4 // chooseAttachPick: the object-side Attach pick, mandatory
	case "AttachPickOpt":
		ki.C = 5 // chooseAttachPickOpt: the same pick, up to one
	case "AttachDest":
		ki.C = 10 // chooseAttachDest: the destination-side Attach pick over the remembered group (Fumble)
	case "PutCounterPick":
		ki.C = 6 // choosePutCounterPick
	case "FaceDownPick":
		ki.C = 7 // chooseFaceDownPick: the cloak-from-hand pick (gorge's face_down_choice)
	case "CopyPick":
		ki.C = 8 // chooseCopyPick: the CopyPermanent Choices$ pick (gorge's copypermanent_choice)
	case "PopulatePick":
		ki.C = 9 // choosePopulatePick: the Populate$ token pick, a lone candidate taken unasked
	case "Amount":
		ki.C = kop.ChooseCardAmount // A = the pick count, every pick taken
	case "AmountRemember":
		ki.C = kop.ChooseCardAmount // B = 1: each pick also joins the remembered set
	case "AmountOpt":
		ki.C = kop.ChooseCardAmountOpt // A = the most picks, Done ends it early
	case "AmountOptRemember":
		ki.C = kop.ChooseCardAmountOpt // B = 1: each pick also joins the remembered set
	case "AtRandom":
		ki.C = kop.ChooseCardAtRandom // A = the pick count, the engine rng picks
	case "EachPlayer":
		ki.C = kop.ChooseCardEach // every player picks one, AliveFrom(controller)
	case "ActivatorBase":
		ki.C = kop.ChooseCardActivatorBase // match the pool from the activator
	}
	return nil
}

func (ic *instrCtx) opChooseEachType() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpChooseEachType
	if in.CS != "Artifact & Battle & Creature & Enchantment & Land & Planeswalker" {
		return fmt.Errorf("ChooseEach %q", in.CS)
	}
	return nil
}

func (ic *instrCtx) opDigBottom() error {
	ki := &ic.ki
	// the DigUntil's cards still in exile to the library bottom in a
	// random order (lower_digimprint.go; kernel act_discoverbottom.go)
	ki.Op, ki.Obj, ki.C = kop.OpDiscover, kop.RefController, kop.DiscoverDugBottom
	return nil
}

func (ic *instrCtx) opRemBottom() error {
	ki := &ic.ki
	// the remembered cards still in the library to its bottom in a
	// random order (lower_playlibrary.go; kernel act_discoverbottom.go)
	ki.Op, ki.Obj, ki.C = kop.OpDiscover, kop.RefController, kop.DiscoverRemBottom
	return nil
}

func (ic *instrCtx) opDigImprint() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtDigImprint)) // marks the Dig before it (zone_digimprint.go)
	return nil
}
