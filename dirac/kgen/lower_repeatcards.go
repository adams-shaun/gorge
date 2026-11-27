package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// repeatEachCards lowers api:RepeatEach over cards (gorge effRepeatEach,
// repeatedCards): RepeatCards$ <filter> with Zone$ Battlefield (or none), or
// DefinedCards$ Targeted over the chain's one target slot, whose
// RepeatSubAbility$ body runs once per card with that card as Remembered
// (cc.Remembered = [card]). Lowered as OpRepeatEach (act_repeatcards.go) for
// the body the kernel loops: a lone Role Token whose AttachedTo$ Remembered
// names the repeated card (the WOE Roles: Asinine Antics, Become Brutes,
// Twisted Sewer-Witch), read as RefEach, or, under ChangeZoneTable$ True, a
// Token with a body chain (Ezuri's Predation: a Beast token per creature and
// a Fight, lower_repeatcards.go) or a CopyPermanent (Mirror-Style
// Master). That loop runs inside one zone batch under ChangeZoneTable$ True;
// without it the loop is a plain per-card one over a one-instruction body
// (repeatEachCardsLoop); every other rider (DamageMap$, a body of any other
// shape, a nested loop) refuses by name. The one other body is a DealDamage
// by the repeated card to its controller under DamageMap$ True
// (repeatEachCardsDamage), which moves no card, so it reads no
// ChangeZoneTable$.
//
// DefinedCards$ DirectRemembered and RememberedCard route to the remembered
// loops instead (lower_repeatremembered.go): a batched/inline body, or the
// plain resumable frame over a preceding token mint.
func (lc *lowerCtx) repeatEachCards(ps *params) error {
	cardsSpec, defSpec := strings.TrimSpace(ps.str("RepeatCards")), strings.TrimSpace(ps.str("DefinedCards"))
	if cardsSpec == "" && defSpec == "" {
		return refuse("RepeatEach without RepeatPlayers$")
	}
	if cardsSpec != "" && defSpec != "" {
		return refuse("RepeatEach RepeatCards$ beside DefinedCards$")
	}
	if lc.repeatPl > 0 || lc.repeatCard > 0 {
		return refuse("RepeatEach cards nested in a RepeatEach body")
	}
	if done, err := lc.imprintNameSweep(ps, cardsSpec); done || err != nil {
		return err // lower_imprintzone.go
	}
	// the damage body moves no card: it reads no ChangeZoneTable$
	// (repeatEachCardsDamage)
	dmgBody := false
	if b := cards.ResolveSVar(lc.face.SVars, ps.peek("RepeatSubAbility")); b != nil && b.API == "DealDamage" && b.Sub == nil && ps.peek("DamageMap") == "True" {
		dmgBody = true
	}
	// no ChangeZoneTable$: a plain per-card loop (repeatEachCardsLoop)
	table := dmgBody || ps.is("ChangeZoneTable", "True")
	if z := strings.TrimSpace(ps.str("Zone")); z != "" && z != "Battlefield" {
		return refuse("RepeatEach cards Zone$ %q", z)
	}
	in := lInstr{Op: "RepeatEachCard", Obj: lRef{Kind: rNone}}
	if strings.HasPrefix(defSpec, "DirectRemembered.") {
		// gorge's repeatedCards reads the subject spelling up to its first
		// dot (engine/effects/repeateach.go repeatedCards), so a filtered
		// DirectRemembered.<type> is the whole remembered list. Only the
		// optional remembered-searcher body lowers it (Wave of Vitriol);
		// lower_repeatremembered_ask.go refuses every other shape.
		return lc.repeatEachRememberedSearch(ps, in)
	}
	if defSpec == "DirectRemembered" {
		return lc.repeatEachRemembered(ps, in) // lower_repeatremembered.go
	}
	if defSpec == "RememberedCard" {
		return lc.repeatEachRememberedCard(ps, in) // lower_repeatremembered.go
	}
	if defSpec != "" {
		if defSpec != "Targeted" {
			return refuse("RepeatEach DefinedCards$ %q", defSpec)
		}
		if len(lc.chainT) != 1 {
			return refuse("RepeatEach DefinedCards$ Targeted over %d chain target slots", len(lc.chainT))
		}
		t := &lc.ab.Targets[lc.chainT[0]]
		if t.F == nil || t.F.Players != 0 || len(t.F.Clauses) == 0 {
			return refuse("RepeatEach DefinedCards$ Targeted: the target slot is not a card slot")
		}
		in.Obj = lRef{Kind: rTarget, N: lc.chainT[0]}
	} else {
		for _, w := range []string{"Remember", "Imprint", "Chosen", "Triggered", "Targeted", "Defined"} {
			if strings.Contains(cardsSpec, w) {
				return refuse("RepeatEach RepeatCards$ %q reads %s", cardsSpec, w)
			}
		}
		f, err := lc.filter(cardsSpec)
		if err != nil {
			return err
		}
		if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
			return refuse("RepeatEach RepeatCards$ %q: not a card filter", cardsSpec)
		}
		in.F = f
	}
	name := ps.str("RepeatSubAbility")
	body := cards.ResolveSVar(lc.face.SVars, name)
	if body == nil {
		return refuse("RepeatEach RepeatSubAbility %q", name)
	}
	if dmgBody {
		return lc.repeatEachCardsDamage(ps, in, body) // lower_repeatcardsdamage.go
	}
	if !table {
		return lc.repeatEachCardsLoop(in, body) // lower_repeatcardsloop.go
	}
	return lc.repeatEachCardsBatch(ps, in, body) // lower_repeatcards.go
}

// repeatEachCardsBatch lowers the batched loop's body (ChangeZoneTable$ True):
// a lone Role Token (the WOE Roles), or one of the two shapes the kernel's
// inline loop can run: a Token with a body chain (Ezuri's Predation's token
// then Fight) and a CopyPermanent (Mirror-Style Master). ChooseOrder$ True is
// read for those two shapes only (kop.RepeatEachOrder): the controller orders
// the subjects before the loop, as gorge's repeatEachChooseOrder asks.
func (lc *lowerCtx) repeatEachCardsBatch(ps *params, in lInstr, body *cards.SA) error {
	switch body.API {
	case "Token", "CopyPermanent":
	default:
		return refuse("RepeatEach cards ChangeZoneTable$ True body %s (only a lone Token: gorge batches the moves)", body.API)
	}
	useImp := false
	if v, ok := ps.get("UseImprinted"); ok {
		if v != "True" {
			return refuse("RepeatEach cards UseImprinted$ %q", v)
		}
		useImp = true
	}
	order := false
	if v, ok := ps.get("ChooseOrder"); ok {
		if v != "True" {
			return refuse("RepeatEach cards ChooseOrder$ %q", v)
		}
		if body.API == "Token" && body.Sub == nil {
			// the lone Role shape keeps its existing refusal (no corpus card
			// carries ChooseOrder$ on it)
			return refuse("RepeatEach cards ChooseOrder$ True body Token")
		}
		order = true
	}
	at, nt := len(lc.ab.Prog), len(lc.ab.Targets)
	lc.emit(in)
	savedImp := lc.repeatUseImp
	lc.repeatUseImp = useImp
	if useImp && body.API == "CopyPermanent" && lc.curNode != nil && lc.curNode.Sub != nil {
		// the delayed trigger that follows the loop reads the whole
		// remembered set (Mirror-Style Master's end-of-combat exile)
		lc.repeatCztNode = lc.curNode
	}
	lc.repeatCard++
	err := lc.chain(body, nil)
	lc.repeatCard--
	lc.repeatUseImp = savedImp
	if err != nil {
		return err
	}
	if len(lc.ab.Targets) != nt {
		return refuse("RepeatEach body with a target")
	}
	n := len(lc.ab.Prog) - at - 1
	if err := lc.checkRepeatBatchBody(body, lc.ab.Prog[at+1:at+1+n]); err != nil {
		return err
	}
	if order {
		lc.ab.Prog[at].C |= kop.RepeatEachOrder
	}
	lc.ab.Prog[at].Skip = n
	return nil
}

// checkRepeatBatchBody validates the instructions the batched body emitted.
// The lone Role shape keeps its exact check; a Token chain may only create the
// token, run the Imprinted/Remembered Fight and clear the remembered set
// (Ezuri's Predation); a CopyPermanent is one copy instruction.
func (lc *lowerCtx) checkRepeatBatchBody(body *cards.SA, prog []lInstr) error {
	if body.API == "CopyPermanent" {
		if body.Sub != nil {
			return refuse("RepeatEach cards CopyPermanent body with a SubAbility$")
		}
		if len(prog) != 1 || prog[0].Op != "CopyAsToken" {
			return refuse("RepeatEach cards CopyPermanent body emitted %d instructions", len(prog))
		}
		return nil
	}
	if body.Sub == nil {
		if len(prog) != 1 {
			return refuse("RepeatEach cards body emitted %d instructions", len(prog))
		}
		if tok := &prog[0]; tok.Op != "CreateToken" || tok.C&32 == 0 || tok.Obj2.Kind != rEach {
			return refuse("RepeatEach cards body is not a Role token attached to the repeated card")
		}
		return nil
	}
	if len(prog) == 0 || prog[0].Op != "CreateToken" {
		return refuse("RepeatEach cards Token body emitted %d instructions, not a CreateToken first", len(prog))
	}
	fight := false
	for i := range prog {
		switch prog[i].Op {
		case "CreateToken":
			if i != 0 {
				return refuse("RepeatEach cards Token body creates a second token")
			}
		case "FightRemembered":
			if fight {
				return refuse("RepeatEach cards Token body fights twice")
			}
			fight = true
		case "ClearRemembered":
		default:
			return refuse("RepeatEach cards Token body %s is not the token-then-Fight shape", prog[i].Op)
		}
	}
	if !fight {
		return refuse("RepeatEach cards Token body without the Imprinted/Remembered Fight")
	}
	return nil
}
