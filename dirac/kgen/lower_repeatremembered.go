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
// lone Token is lowered. A body that asks runs as the resumable frame instead
// (repeatEachRememberedAsk). A filtered DirectRemembered.<type> stays refused
// by name. The preceding instruction must be the one that remembered (a
// DestroyAll with RememberDestroyed$ or an Exile that remembers): a loop over
// a set something else wrote is not this shape.
//
// RememberedCard is the other spelling of the same remembered set, over a
// preceding token mint (RepeatEach DefinedCards$ RememberedCard over a Token
// with RememberTokens$): repeatEachRememberedCard.
func (lc *lowerCtx) repeatEachRemembered(ps *params, in lInstr) error {
	if !ps.is("UseImprinted", "True") {
		return refuse("RepeatEach DefinedCards$ DirectRemembered without UseImprinted$ True")
	}
	if ps.peek("DamageMap") != "" {
		return lc.repeatEachRememberedAsk(ps, in) // lower_repeatremembered_ask.go
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

// repeatEachRememberedCard lowers the RememberedCard spelling over the tokens
// a preceding Token mint remembered:
//
//	SP$ Token | TokenAmount$ X | RememberTokens$ True | SubAbility$ DBRepeat
//	DBRepeat: DB$ RepeatEach | DefinedCards$ RememberedCard |
//	  RepeatSubAbility$ ArmToken | ClearRemembered$ True
//	ArmToken: DB$ Attach | Choices$ Equipment.YouCtrl | Defined$ Remembered |
//	  Optional$ True
//
// (Reckless Crew). gorge effRepeatEach lists the tokens the mint remembered
// (repeatedCards reads the RememberedCard spelling as the remembered set) and
// runs ArmToken once per token as a resumable pass (RefEach): the
// object-side Attach pick asks which Equipment to attach to that token. The
// kernel's OpRepeatEach with kop.RepeatRemembered|kop.RepeatPlain is that
// frame, and the ClearRemembered$ after the loop is gorge's loop hygiene (the
// clear runs before the first pass there, after the subject list is taken;
// this body writes no memory, so the set is empty either way). The preceding
// instruction must be the token mint that remembered them, and the body must
// be exactly the optional object-side pick onto RefEach.
func (lc *lowerCtx) repeatEachRememberedCard(ps *params, in lInstr) error {
	if ps.peek("UseImprinted") != "" {
		return refuse("RepeatEach DefinedCards$ RememberedCard with UseImprinted$")
	}
	if ps.peek("DamageMap") != "" {
		return refuse("RepeatEach DefinedCards$ RememberedCard with DamageMap$")
	}
	if ps.peek("ChangeZoneTable") != "" {
		return refuse("RepeatEach DefinedCards$ RememberedCard with ChangeZoneTable$")
	}
	if n := len(lc.ab.Prog); n == 0 || lc.ab.Prog[n-1].Op != "CreateToken" || lc.ab.Prog[n-1].C&4 == 0 {
		return refuse("RepeatEach DefinedCards$ RememberedCard over a set no preceding Token remembered")
	}
	clear := false
	if v, ok := ps.get("ClearRemembered"); ok {
		if strings.TrimSpace(v) != "True" {
			return refuse("RepeatEach DefinedCards$ RememberedCard with ClearRemembered$ %q", v)
		}
		clear = true
	}
	name := ps.str("RepeatSubAbility")
	body := cards.ResolveSVar(lc.face.SVars, name)
	if body == nil {
		return refuse("RepeatEach RepeatSubAbility %q", name)
	}
	in.C = kop.RepeatRemembered | kop.RepeatPlain
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
	if len(lc.ab.Prog) != at+3 {
		return refuse("RepeatEach RememberedCard body emitted %d instructions", len(lc.ab.Prog)-at-1)
	}
	pick, att := &lc.ab.Prog[at+1], &lc.ab.Prog[at+2]
	if pick.Op != "ChooseCard" || pick.Obj.Kind != rYou || pick.F == nil || pick.CS != "AttachPickOpt" ||
		att.Op != "AttachObj" || att.Obj.Kind != rEach || att.Obj2.Kind != rChosen {
		return refuse("RepeatEach RememberedCard body is not one optional object-side Attach pick onto the repeated token")
	}
	lc.ab.Prog[at].Skip = 2
	if clear {
		lc.emit(lInstr{Op: "ClearRemembered"})
	}
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
