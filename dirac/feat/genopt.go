package feat

import "github.com/adams-shaun/gorge/dirac"

// genOption maps a generated table's resolution ask (kernel ChGen* choice
// kinds, dirac/doc.go "Generated tables") onto the gorge option it stands
// for: gorge poses most as KChoose with its own option kinds, and a branch,
// a free cast and a cost-in-an-effect pay as KModes.
func (e *Encoder) genOption(d *dirac.Decision, a dirac.Action, o *optIn) {
	src, mode := a.Source(), a.Mode()
	o.obj = dirac.Obj(src)
	switch d.Sub {
	case dirac.ChGenSearch:
		o.kind = okSearch
	case dirac.ChGenHiddenPick:
		o.kind = okHiddenPick
	case dirac.ChGenHandMove, dirac.ChGenImprint:
		o.kind = okHandMove
	case dirac.ChGenReveal:
		o.kind = okReveal
	case dirac.ChGenDig:
		o.kind = okDig
	case dirac.ChGenBolster:
		o.kind = okCounterPick
	case dirac.ChGenCopyPick:
		o.kind = okPermanent // gorge's CopyPermanent Choices$ pick
	case dirac.ChGenProlif:
		o.kind = okProliferate
		if src >= dirac.PlayerTarget0 {
			// gorge poses this as Kind "proliferate" with Obj 0 (not a
			// player-ref Obj), and policynet's isPlayer reads only a
			// player-ref Obj or Kind "player": not a player option.
			o.obj = noObj
		}
	case dirac.ChGenPayPick:
		o.kind = okSacrifice
	case dirac.ChGenPileSplit:
		o.kind = okTwoPiles
	case dirac.ChGenPileChoose:
		o.kind, o.obj = okPileA, noObj
		if mode == 1 {
			o.kind = okPileB
		}
	case dirac.ChGenTopBottom:
		o.kind, o.obj = okBottom, noObj
		if mode == 1 {
			o.kind = okPrimary
		}
	case dirac.ChGenBranch:
		o.dk, o.kind, o.obj = dkModes, okMode, e.source
	case dirac.ChGenCastFree:
		o.dk, o.kind = dkModes, okMode // obj: the card to cast ("Play <card>")
	case dirac.ChGenPayMode:
		// "unless_pay" / "unless_decline": the mode-other slot
		o.dk, o.kind, o.obj, o.mode = dkModes, okMode, e.source, uint8(len(modeValues))
	case dirac.ChGenPay:
		o.kind, o.obj = okTriggerCostDecline, e.source
		if mode == 1 {
			o.kind = okTriggerCostPay
		}
	case dirac.ChGenColor:
		o.kind, o.obj = okColor, noObj
	case dirac.ChGenType:
		o.kind, o.obj = okType, noObj
	case dirac.ChGenMayShuffle:
		o.kind, o.obj = okNo, noObj
		if mode == 1 {
			o.kind = okYes
		}
	case dirac.ChGenConfirm:
		o.kind, o.obj = okNo, noObj // gorge's hand_move_confirm yes / no
		if mode == 1 {
			o.kind = okYes
		}
	case dirac.ChGenDigOpt:
		o.kind, o.obj = okNo, noObj // gorge effDig's allOrNothing yes / no
		if mode == 1 {
			o.kind = okYes
		}
	case dirac.ChGenExplore:
		o.kind = okGraveyard // gorge's explore "graveyard" / "top" over the revealed card
		if mode == 1 {
			o.kind = okTop
		}
	case dirac.ChGenKeepTapped:
		o.kind = okUntap // gorge's untap-step election: "untap" / "keep_tapped" (tap stands for keep)
		if mode == 1 {
			o.kind = okTap
		}
	case dirac.ChGenTapOrUntap:
		o.kind = okUntap // gorge's taporuntap "tap" / "untap" over the permanent
		if mode == 1 {
			o.kind = okTap
		}
	case dirac.ChGenLookAck:
		o.kind, o.obj = okYes, noObj // gorge's look_ack "Continue"
	case dirac.ChGenChooseCard, dirac.ChGenChooseEach, dirac.ChGenResTarget:
		o.kind = okCard
	case dirac.ChGenClone:
		// gorge's ETB "clone" ask: a template card, or "enter as itself"
		o.kind = okCard
		if mode == 1 {
			o.kind, o.obj = okNo, noObj
		}
	case dirac.ChGenEnchant:
		o.kind = okEnchant // gorge's CR 303.4f "enchant" ask
	case dirac.ChCastDivide:
		// gorge's cast-time division: "divide" over the target (a player
		// target names Player, not Obj: no card, not a player option)
		o.kind = okDivide
		if src >= dirac.PlayerTarget0 {
			o.obj = noObj
		}
	case dirac.ChGenDivide:
		// gorge's resolution division: a "card" (or "player") pick per point
		o.kind = okCard
		if src >= dirac.PlayerTarget0 {
			o.kind, o.obj, o.player = okPlayer, noObj, true
			o.named = int8(src - dirac.PlayerTarget0)
		}
	case dirac.ChGenReplOrder:
		o.dk, o.kind = dkReplacement, okReplacement // obj: the replacement's source
	case dirac.ChGenPip:
		o.kind, o.obj, o.amount = okPayW+mode%5, noObj, 1 // gorge: Amount 1 (one pip)
		if mode == 6 {
			o.kind, o.amount = okPayGeneric, 2 // a twobrid pip's generic face: gorge pay_generic, Amount 2
		}
	case dirac.ChGenLeyline:
		o.kind = okOpeningNo
		if mode == 1 {
			o.kind = okOpeningYes
		}
	default:
		o.kind, o.obj = okNumber, noObj
	}
}

// genDoneDK is the decision kind gorge poses a generated ask closed by
// AkDone as (the kernel's Done itself has no gorge option).
func genDoneDK(sub dirac.ChoiceKind) (int8, bool) {
	switch sub {
	case dirac.ChGenBranch, dirac.ChGenCastFree, dirac.ChGenPayMode:
		return dkModes, true
	}
	return 0, false
}
