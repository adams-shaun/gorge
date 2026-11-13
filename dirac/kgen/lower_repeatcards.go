package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// repeatEachCards lowers api:RepeatEach over cards (gorge effRepeatEach,
// repeatedCards): RepeatCards$ <filter> with Zone$ Battlefield (or none), or
// DefinedCards$ Targeted over the chain's one target slot, whose
// RepeatSubAbility$ body runs once per card with that card as Remembered
// (cc.Remembered = [card]). Lowered as OpRepeatEach (act_repeatcards.go) for
// the one body the kernel loops: a single Token whose AttachedTo$ Remembered
// names the repeated card (the WOE Roles: Asinine Antics, Become Brutes,
// Twisted Sewer-Witch), read as RefEach. The loop runs inside one zone batch,
// so ChangeZoneTable$ True is required (it is what the kernel does); every
// other rider (ChooseOrder$, DamageMap$, a body of any other shape, a
// nested loop) refuses by name. The one other body is a DealDamage by the
// repeated card to its controller (repeatEachCardsDamage), which moves no
// card, so it needs no ChangeZoneTable$.
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
	if z := strings.TrimSpace(ps.str("Zone")); z != "" && z != "Battlefield" {
		return refuse("RepeatEach cards Zone$ %q", z)
	}
	in := lInstr{Op: "RepeatEachCard", Obj: lRef{Kind: rNone}}
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
	if body.API == "DealDamage" && body.Sub == nil {
		return lc.repeatEachCardsDamage(ps, in, body) // lower_repeatcardsdamage.go
	}
	if !ps.is("ChangeZoneTable", "True") {
		return refuse("RepeatEach cards without ChangeZoneTable$ True")
	}
	if body.API != "Token" || body.Sub != nil {
		return refuse("RepeatEach cards body %s (only a lone Token)", body.API)
	}
	at, nt := len(lc.ab.Prog), len(lc.ab.Targets)
	lc.emit(in)
	lc.repeatCard++
	err := lc.chain(body, nil)
	lc.repeatCard--
	if err != nil {
		return err
	}
	if len(lc.ab.Targets) != nt {
		return refuse("RepeatEach body with a target")
	}
	if len(lc.ab.Prog) != at+2 {
		return refuse("RepeatEach cards body emitted %d instructions", len(lc.ab.Prog)-at-1)
	}
	if tok := &lc.ab.Prog[at+1]; tok.Op != "CreateToken" || tok.C&32 == 0 || tok.Obj2.Kind != rEach {
		return refuse("RepeatEach cards body is not a Role token attached to the repeated card")
	}
	lc.ab.Prog[at].Skip = 1
	return nil
}
