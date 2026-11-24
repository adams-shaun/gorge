package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// repeatEachRemembered lowers api:RepeatEach over the resolution's remembered
// cards, the loop that follows a mass effect that remembers what it moved:
//
//	DB$ RepeatEach | DefinedCards$ DirectRemembered | UseImprinted$ True |
//	  RepeatSubAbility$ <body> | ChangeZoneTable$ True
//	<body>: DB$ Token | TokenOwner$ ImprintedController | TokenScript$ <script>
//
// (Rampage of the Clans: DestroyAll RememberDestroyed$; Curse of the Swine and
// Will, Scholar of Frost's ultimate: ChangeZone Destination$ Exile
// RememberLKI$). gorge effRepeatEach lists the remembered objects (repeatedCards
// over hostRemembered, which "DirectRemembered" reads as the RememberLKI$ set),
// binds each as the loop subject (UseImprinted$ names it Imprinted, so the
// resolution's Remembered is untouched) and runs the body once per card, in one
// zone batch (ChangeZoneTable$ True). The body reads the subject's controller
// as ImprintedController: its last known one when the card left the
// battlefield under a RememberLKI$ move, else its live controller (a card that
// moved with no capture reads its owner). That is the kernel's OpRepeatEach
// with kop.RepeatRemembered, the body's player ref RefEach naming that
// controller (dirac act_repeatremembered.go).
//
// The loop runs its body inline in one call, so the body may not ask: only a
// lone Token is lowered. A body that asks (Wave of Vitriol's optional search,
// Reckless Crew's optional attach), a body under DamageMap$ True beside an
// unless-payment ask (Stench of Evil: the batch would have to stay open across
// the ask), a filtered DirectRemembered.<type> and the RememberedCard spelling
// stay refused by name. The preceding instruction must be the one that
// remembered (a DestroyAll with RememberDestroyed$ or an Exile that
// remembers): a loop over a set something else wrote is not this shape.
func (lc *lowerCtx) repeatEachRemembered(ps *params, in lInstr) error {
	if !ps.is("UseImprinted", "True") {
		return refuse("RepeatEach DefinedCards$ DirectRemembered without UseImprinted$ True")
	}
	if ps.peek("DamageMap") != "" {
		return refuse("RepeatEach DefinedCards$ DirectRemembered under DamageMap$ (an unless-payment ask needs the damage batch open across the ask)")
	}
	if !ps.is("ChangeZoneTable", "True") {
		return refuse("RepeatEach DefinedCards$ DirectRemembered without ChangeZoneTable$ True")
	}
	if n := len(lc.ab.Prog); n == 0 || !remembersMoved(&lc.ab.Prog[n-1]) {
		return refuse("RepeatEach DefinedCards$ DirectRemembered over a set no preceding DestroyAll/Exile remembered")
	}
	name := ps.str("RepeatSubAbility")
	body := cards.ResolveSVar(lc.face.SVars, name)
	if body == nil {
		return refuse("RepeatEach RepeatSubAbility %q", name)
	}
	if body.API != "Token" || body.Sub != nil || strings.TrimSpace(body.Params["TokenOwner"]) != "ImprintedController" {
		return refuse("RepeatEach DefinedCards$ DirectRemembered body %s (only a lone Token minted for ImprintedController: a body that asks cannot resume inside the batched loop)", body.API)
	}
	in.C = kop.RepeatRemembered
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
	if len(lc.ab.Prog) != at+2 {
		return refuse("RepeatEach remembered body emitted %d instructions", len(lc.ab.Prog)-at-1)
	}
	if tok := &lc.ab.Prog[at+1]; tok.Op != "CreateToken" || tok.Obj.Kind != rEach {
		return refuse("RepeatEach remembered body is not a Token minted for the repeated card's controller")
	}
	lc.ab.Prog[at].Skip = 1
	return nil
}

// remembersMoved reports whether in is an instruction that moved cards and
// remembered them: a DestroyAll with RememberDestroyed$ (C bit 1), or an Exile
// whose move flags remember (RememberLKI$).
func remembersMoved(in *lInstr) bool {
	switch in.Op {
	case "DestroyAll":
		return in.C&1 != 0
	case "Exile":
		return strings.Contains(in.CS, "Remember")
	}
	return false
}
