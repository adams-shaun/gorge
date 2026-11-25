package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// repeatEachRememberedAsk lowers the asking body of api:RepeatEach over the
// remembered cards, the one gorge runs without ChangeZoneTable$ and inside a
// damage bracket:
//
//	DB$ RepeatEach | DefinedCards$ DirectRemembered | UseImprinted$ True |
//	  RepeatSubAbility$ <body> | DamageMap$ True
//	<body>: DB$ DealDamage | NumDmg$ <N> | Defined$ ImprintedController |
//	  UnlessCost$ <cost> | UnlessPayer$ ImprintedController
//
// (Stench of Evil: "for each land destroyed this way, deals 1 damage to that
// land's controller unless they pay {2}"). gorge effRepeatEach opens its
// damage batch around the whole loop and runs the body once per remembered
// card, in remembered order, as a resumable pass: each pass asks its
// controller to pay. The kernel's OpRepeatEach with C RepeatRemembered |
// RepeatPlain | RepeatDmgBatch is that frame (dirac act_repeatcards.go): the
// header opens the damage batch, the body's [UnlessPaid, DealDamage] pair runs
// each pass with RefEach, the bound card's controller, as payer and recipient,
// and loopBack closes the batch behind the last pass. The body is exactly one
// DealDamage behind its one unless-payment ask.
func (lc *lowerCtx) repeatEachRememberedAsk(ps *params, in lInstr) error {
	if !ps.is("DamageMap", "True") {
		return refuse("RepeatEach DefinedCards$ DirectRemembered DamageMap$ %q", ps.peek("DamageMap"))
	}
	if ps.peek("ChangeZoneTable") != "" {
		return refuse("RepeatEach DefinedCards$ DirectRemembered with both DamageMap$ and ChangeZoneTable$")
	}
	if n := len(lc.ab.Prog); n == 0 || !remembersMoved(&lc.ab.Prog[n-1]) {
		return refuse("RepeatEach DefinedCards$ DirectRemembered over a set no preceding DestroyAll/Exile remembered")
	}
	name := ps.str("RepeatSubAbility")
	body := cards.ResolveSVar(lc.face.SVars, name)
	if body == nil {
		return refuse("RepeatEach RepeatSubAbility %q", name)
	}
	if body.API != "DealDamage" || body.Sub != nil {
		return refuse("RepeatEach DefinedCards$ DirectRemembered DamageMap$ body %s (only a lone DealDamage)", body.API)
	}
	ps.consume("DamageMap")
	// ShowCurrentCard$ names the card the prompt shows: Forge's display hint,
	// read nowhere in engine/. Dropped from a copy of the body, for this shape
	// alone.
	shown := *body
	shown.Params = make(map[string]string, len(body.Params))
	for k, v := range body.Params {
		if k != "ShowCurrentCard" {
			shown.Params[k] = v
		}
	}
	body = &shown
	in.C = kop.RepeatRemembered | kop.RepeatPlain | kop.RepeatDmgBatch
	at, nt := len(lc.ab.Prog), len(lc.ab.Targets)
	lc.emit(in)
	lc.repeatCard++
	lc.repeatRem = true
	err := lc.chain(body, nil)
	lc.repeatRem = false
	lc.repeatCard--
	if err != nil {
		return err
	}
	if len(lc.ab.Targets) != nt {
		return refuse("RepeatEach body with a target")
	}
	if len(lc.ab.Prog) != at+3 || lc.ab.Prog[at+1].Op != "UnlessPaid" || lc.ab.Prog[at+1].Skip != 1 ||
		lc.ab.Prog[at+1].Obj.Kind != rEach || lc.ab.Prog[at+2].Op != "DealDamage" || lc.ab.Prog[at+2].Obj.Kind != rEach {
		return refuse("RepeatEach remembered body is not one DealDamage to the repeated card's controller behind its unless-payment")
	}
	lc.ab.Prog[at].Skip = 2
	return nil
}
