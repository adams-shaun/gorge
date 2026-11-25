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
		return lc.repeatEachRememberedSearch(ps, in) // the Wave of Vitriol shape
	}
	if ps.peek("ChangeZoneTable") != "" {
		return refuse("RepeatEach DefinedCards$ DirectRemembered with both DamageMap$ and ChangeZoneTable$")
	}
	if n := len(lc.ab.Prog); n == 0 || !remembersMoved(&lc.ab.Prog[n-1]) {
		return refuse("RepeatEach DefinedCards$ DirectRemembered over a set no preceding DestroyAll/SacrificeAll/Exile remembered")
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

// repeatEachRememberedSearch lowers api:RepeatEach over the remembered cards
// with the optional basic-land search body and the remembered-searcher shuffle
// that follows it (Wave of Vitriol):
//
//	DB$ RepeatEach | DefinedCards$ DirectRemembered.<filter> | UseImprinted$ True |
//	  RepeatSubAbility$ <body> | ClearRemembered$ True | SubAbility$ <shuffle>
//	<body>: DB$ ChangeZone | Origin$ Library | Destination$ Battlefield |
//	  ChangeType$ Land.Basic | Tapped$ True | RememberSearcher$ True |
//	  DefinedPlayer$ ImprintedController | Chooser$ ImprintedController |
//	  NoShuffle$ True | Optional$ True
//
// gorge's repeatedCards reads DefinedCards$ up to its first dot, so the
// filtered spelling is the whole remembered list (no .Land filter). The loop
// opens no zone batch (no ChangeZoneTable$), so it is the resumable frame
// (kop.RepeatRemembered | kop.RepeatPlain): each pass poses its controller's
// Optional$ confirm and, on a yes, the search, with RefEach naming the bound
// card's controller (act_repeatremembered.go eachCtrl). RememberSearcher$
// records the seat that searched, found or not, for the Shuffle $
// Player.IsRemembered after it (zone_remembersearcher.go); ClearRemembered$
// empties the object set once the subject list is taken and leaves that seat
// mask alone.
func (lc *lowerCtx) repeatEachRememberedSearch(ps *params, in lInstr) error {
	if !ps.is("UseImprinted", "True") {
		return refuse("RepeatEach DefinedCards$ DirectRemembered without UseImprinted$ True")
	}
	if ps.peek("DamageMap") != "" {
		return refuse("RepeatEach DefinedCards$ DirectRemembered.<filter> beside DamageMap$")
	}
	if ps.peek("ChangeZoneTable") != "" {
		return refuse("RepeatEach DefinedCards$ DirectRemembered.<filter> beside ChangeZoneTable$")
	}
	if n := len(lc.ab.Prog); n == 0 || !remembersMoved(&lc.ab.Prog[n-1]) {
		return refuse("RepeatEach DefinedCards$ DirectRemembered over a set no preceding DestroyAll/SacrificeAll/Exile remembered")
	}
	name := ps.str("RepeatSubAbility")
	body := cards.ResolveSVar(lc.face.SVars, name)
	if body == nil {
		return refuse("RepeatEach RepeatSubAbility %q", name)
	}
	if body.API != "ChangeZone" || body.Sub != nil ||
		body.ParamStr(cards.PKOrigin) != "Library" || body.ParamStr(cards.PKDestination) != "Battlefield" ||
		body.ParamStr(cards.PKRememberSearcher) != "True" || body.ParamStr(cards.PKDefinedPlayer) != "ImprintedController" ||
		body.ParamStr(cards.PKChooser) != "ImprintedController" || body.ParamStr(cards.PKOptional) != "True" {
		return refuse("RepeatEach DefinedCards$ DirectRemembered.<filter> body %s (only the optional basic-land search by ImprintedController)", body.API)
	}
	if !ps.is("ClearRemembered", "True") {
		return refuse("RepeatEach DefinedCards$ DirectRemembered.<filter> without ClearRemembered$ True")
	}
	ps.consume("ClearRemembered")
	in.C = kop.RepeatRemembered | kop.RepeatPlain
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
	if len(lc.ab.Prog) != at+3 ||
		lc.ab.Prog[at+1].Op != "RememberSearcher" || lc.ab.Prog[at+1].Obj.Kind != rEach ||
		lc.ab.Prog[at+2].Op != "Search" || lc.ab.Prog[at+2].Obj.Kind != rEach {
		return refuse("RepeatEach remembered search body is not one RememberSearcher$ and one Search for the repeated card's controller")
	}
	lc.ab.Prog[at].Skip = 2
	// the subject list is taken at the header; clearing after the body leaves
	// the remembered-searcher seat mask for the Shuffle that follows.
	lc.emit(lInstr{Op: "ClearRemembered"})
	return nil
}
