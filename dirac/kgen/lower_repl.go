package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// lowerRepl lowers an R: line to a replacement-lite record.
func (lc *lowerCtx) lowerRepl(r cards.Repl) error {
	if printedPreventRepl(r) {
		// the printed K:Prevent keyword already became a CardDef fact; this
		// minted bodyless prevention line must not also lower (lower_preventkw.go)
		return nil
	}
	if lc.selfPreventRepl(r) { // lower_preventkw.go
		return nil
	}
	rp := newParams(r.Params)
	rp.consume("Event", "ReplaceWith", "ReplacementResult")
	rl := lRepl{}
	if b, ok := rp.get("ClassBand"); ok {
		c, err := classBandCond(b) // lower_delayed.go
		if err != nil {
			return err
		}
		rl.Cond = c
	}
	if _, kw := r.Params["Keyword"]; !kw {
		// a plain R: line (Banner of Kinship): gorge runs its choice on the
		// entered object; the ETBReplacement keyword's line (Heraldic
		// Banner) asks while the spell is on the stack
		rl.S = "Updated"
	}
	var err error
	switch r.Event {
	case "Moved":
		if rp.str("Destination") == "Graveyard" {
			if ok, err := lc.exileOnLeave(rp, r.With, &rl); ok || err != nil {
				if err != nil {
					return err
				}
				break
			}
			if err := lc.graveToLibrary(rp, r.With, &rl); err != nil {
				return err
			}
			break
		}
		if ok, err := lc.discardOppToBattlefield(rp, r.With, &rl); ok || err != nil { // lower_discardetb.go
			if err != nil {
				return err
			}
			break
		}
		if ok, err := lc.payBeforeETB(rp, r.With, &rl); ok || err != nil { // lower_paybeforeetb.go
			if err != nil {
				return err
			}
			break
		}
		if rp.str("Destination") != "Battlefield" {
			return refuse("repl-hard: Moved to %s", rp.str("Destination"))
		}
		vc := rp.str("ValidCard")
		other := vc != "Card.Self" && !strings.HasPrefix(vc, "Card.Self+")
		// a self ETB replacement's condition and body read the
		// entering spell (Count$Converge)
		lc.replSelf = !other
		defer func() { lc.replSelf = false }()
		if other {
			if rp.str("ActiveZones") != "Battlefield" {
				return refuse("other-object ETB replacement without ActiveZones Battlefield")
			}
			// the entrant's cast-provenance words (wasCastFromExile,
			// !wasCastFromYourHand): gorge matches the entering object at the
			// Moved replacement window, where the spell resolving into it is
			// still on the stack and the record is alive (gorge
			// castProvenanceAdmits); the kernel reads the same spell entry at
			// the ReplOthersEnterProgram match (runOthersEnterPrograms, inside
			// enterGen; dirac/cast_wascastspell.go). An entrant that arrived
			// any other way has no entry and reads not cast, as gorge's dead
			// ledger record does.
			save := lc.spellProvOK
			lc.spellProvOK = true
			var err error
			if rl.F, err = lc.filter(vc); err != nil {
				lc.spellProvOK = save
				return err
			}
			lc.spellProvOK = save
		} else if vc != "Card.Self" {
			// Card.Self+wasCastByYou: a condition on the entering object.
			if rl.Cond.Kind != cNone {
				return refuse("two replacement conditions")
			}
			f, err := lc.filter(vc)
			if err != nil {
				return err
			}
			rl.Cond = lCond{Kind: cRefMatches, Ref: lRef{Kind: rSelf}, F: f, Op: opGE, A: cst(1)}
		}
		wc, wcOK, err := lc.wasCastHandCond(rp, other) // lower_wascast.go
		if err != nil {
			return err
		}
		if wcOK {
			if rl.Cond.Kind != cNone {
				return refuse("two replacement conditions")
			}
			rl.Cond = wc
		} else if cs, ok := rp.get("CheckSVar"); ok {
			v, err := lc.value(cs)
			if err != nil {
				return err
			}
			if rl.Cond.Kind != cNone {
				return refuse("two replacement conditions")
			}
			rl.Cond = lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
			// gorge replacementMatches: splitCompare/applyCompare on the
			// value; read only for the converge count (Steel Exemplar),
			// every other SVarCompare$ stays unread; the
			// spend too (Freestrider Commando's EQ0), and bloodthirst's GT0
			// (lower_bloodthirst.go)
			if cmp, ok := rp.m["SVarCompare"]; ok && (v.Kind == vConverge || v.Kind == vCastManaSpent || v.Kind == vDmgOppsTaken) {
				rp.used["SVarCompare"] = true
				op, b, err := lc.cmpSpec(cmp)
				if err != nil {
					return err
				}
				rl.Cond.Op, rl.Cond.B = op, b
			}
		}
		if err := lc.replIsPresent(rp, &rl); err != nil { // lower_handmove.go
			return err
		}
		if r.With == nil {
			return refuse("ETB replacement without ReplaceWith")
		}
		ab := &lAbility{Kind: akLatent}
		lc.replOptional = strings.HasSuffix(strings.ToLower(rp.str("KeywordLine")), ":optional")
		if r.With.API == "Clone" && (rl.S == "Updated" || other || rl.Cond.Kind != cNone) {
			return refuse("ETB copy replacement shape")
		}
		savedRE := lc.replEnter
		lc.replEnter = true
		err = lc.withAbility(ab, func() error { return lc.replChain(r.With, other, &rl.Cond) })
		lc.replEnter = savedRE
		if err != nil {
			return err
		}
		rl.Prog = ab.Prog
		rl.Kind = "EntersWith"
		if err := etbRememberedRead(ab.Prog); err != nil {
			return err
		}
		if spyglassChain(r.With) || handNameChain(r.With) {
			// gorge runs this chain (not a bare NameCard body) once the permanent
			// is on the battlefield: the queued ask, not the stack pre-ask
			rl.S = "Updated"
		}
		if other {
			rl.Kind = "OthersEnterWith"
		}
	case "Counter":
		if _, ok := rp.m["ValidCard"]; !ok && rp.str("ActiveZones") == "Battlefield" {
			if err := lc.spellsUncounterable(rp, &rl); err != nil { // lower_uncounterable.go
				return err
			}
			break
		}
		if rp.str("Layer") != "CantHappen" || rp.str("ValidCard") != "Card.Self" || rp.str("ValidSA") != "Spell" {
			return refuse("Counter replacement shape")
		}
		rl.Kind = "CantBeCountered"
		if cs, ok := rp.get("CheckSVar"); ok {
			// Banefire's "if X is 5 or more": the gate reads the spell's X
			v, err := lc.value(cs)
			if err != nil {
				return err
			}
			op, b, err := lc.cmpSpec(rp.str("SVarCompare"))
			if err != nil {
				return err
			}
			rl.Cond = lCond{Kind: cValue, A: v, Op: op, B: b}
		}
	case "Untap":
		if err = lc.untapRepl(r, rp, &rl); err != nil {
			return err
		}
	case "GameLoss", "GameWin":
		if rp.str("Layer") != "CantHappen" || rp.str("ActiveZones") != "Battlefield" {
			return refuse("%s replacement shape", r.Event)
		}
		vp := rp.str("ValidPlayer")
		switch {
		case r.Event == "GameLoss" && vp == "You":
			rl.Kind = "YouCantLose"
			// ValidLoseReason$ LifeReachedZero narrows the replacement to the
			// loss for 0 or less life (CR 704.5a): poison, a drawn-from-empty
			// library and "loses the game" effects still lose (gorge
			// replacement_match.go compares the reason to the loss event's).
			// Any other reason refuses.
			if reason, ok := rp.get("ValidLoseReason"); ok {
				if strings.TrimSpace(reason) != "LifeReachedZero" {
					return refuse("GameLoss ValidLoseReason %q", reason)
				}
				rl.Bits = kop.CantLoseLife
			}
		case r.Event == "GameWin" && vp == "Opponent":
			rl.Kind = "OppsCantWin"
		default:
			return refuse("%s ValidPlayer %q", r.Event, vp)
		}
	case "DamageDone":
		if ok, err := lc.damageRaise(rp, r.With, &rl); ok || err != nil { // lower_damageraise.go
			if err != nil {
				return err
			}
			break
		}
		if ok, err := lc.damagePlus(rp, r.With, &rl); ok || err != nil {
			if err != nil {
				return err
			}
			break
		}
		if err = lc.damageRepl(rp, r.With, &rl); err != nil {
			return err
		}
	case "GainLife":
		if err := lc.lifeGainRepl(rp, r.With, &rl); err != nil {
			return err
		}
	case "Draw":
		if err := lc.drawRepl(rp, r.With, &rl); err != nil { // lower_drawrepl.go
			return err
		}
	case "LifeReduced":
		if err := lc.lifeLossRepl(rp, r.With, &rl); err != nil {
			return err
		}
	case "CreateToken":
		if err := lc.tokenRepl(rp, r.With, &rl); err != nil { // lower_replmult.go
			return err
		}
	case "AddCounter":
		if err := lc.addCounterRepl(rp, r.With, &rl); err != nil {
			return err
		}
	case "TurnFaceUp":
		if err := lc.turnUpRepl(rp, r.With, &rl); err != nil { // lower_turnfaceuprepl.go
			return err
		}
	case "Destroy":
		if ok, err := lc.destroyRegenRepl(rp, r.With, &rl); err != nil { // lower_destroyregen.go
			return err
		} else if !ok {
			if err := lc.destroyHealRepl(rp, r.With, &rl); err != nil {
				return err
			}
		}
	default:
		return refuse("repl-hard: %s replacement", r.Event)
	}
	if err := lc.replCondKeys(rp, &rl, rl.Kind); err != nil { // lower_condkeys.go
		return err
	}
	if left := rp.unread(); len(left) > 0 {
		return refuse("replacement unread params %v", left)
	}
	if _, ok := r.Params["ClassBand"]; ok && rl.Kind != "DoubleCountersYou" {
		// the kernel reads Repl.Cond only where lower_delayed.go wired it
		return refuse("ClassBand on a %s replacement", rl.Kind)
	}
	if rl.Kind == "ExileOnLeave" {
		// State.exileOnLeave holds one filter per card, so two dies-to-exile
		// abilities on one face would both use the last one's filter.
		for i := range lc.out.Repls {
			if lc.out.Repls[i].Kind == "ExileOnLeave" {
				return refuse("repl-hard: two dies-to-exile replacements on one face")
			}
		}
	}
	lc.out.Repls = append(lc.out.Repls, rl)
	return nil
}

// withIs: the replacement body is exactly DB$ api with the given params
// (and the always-present descriptive ones), no sub-ability.
func withIs(w *cards.SA, api string, want map[string]string) bool {
	if w == nil || w.API != api || w.Sub != nil {
		return false
	}
	n := 0
	for k, v := range w.Params {
		if ignoredParams[k] {
			continue
		}
		if strings.TrimSpace(want[k]) != strings.TrimSpace(v) || want[k] == "" {
			return false
		}
		n++
	}
	return n == len(want)
}

// destroyHealRepl lowers Dr. McCoy, Surly Surgeon's destroy replacement: an
// other creature its controller controls with a +1/+1 counter would be
// destroyed; instead remove all +1/+1 counters from it and heal all damage
// on it. The body is a chain, so withIs cannot read it.
func (lc *lowerCtx) destroyHealRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ActiveZones") != "Battlefield" || rp.str("ValidCard") != "Creature.Other+YouCtrl+counters_GE1_P1P1" {
		return refuse("repl-hard: Destroy replacement shape")
	}
	if w == nil || w.API != "RemoveCounter" || w.Sub == nil {
		return refuse("repl-hard: Destroy replacement body")
	}
	bp := newParams(w.Params)
	bp.consume("Defined", "CounterType", "CounterNum", "SubAbility")
	if bp.str("Defined") != "ReplacedCard" || bp.str("CounterType") != "P1P1" || bp.str("CounterNum") != "All" || len(bp.unread()) != 0 {
		return refuse("repl-hard: Destroy replacement body")
	}
	hp := w.Sub
	if hp.API != "HealDamage" || hp.Sub != nil {
		return refuse("repl-hard: Destroy replacement heal body")
	}
	subp := newParams(hp.Params)
	subp.consume("Defined")
	if subp.str("Defined") != "ReplacedCard" || len(subp.unread()) != 0 {
		return refuse("repl-hard: Destroy replacement heal body")
	}
	f, err := lc.filter(rp.str("ValidCard"))
	if err != nil {
		return err
	}
	rl.Kind, rl.F = "DestroyHeal", f
	return nil
}

// damageRepl lowers the DamageDone replacements that are not plus-N:
// Twinflame Tyrant's doubling (a source you control, to an opponent or a
// permanent an opponent controls), the filtered doublers (lower_replmult.go)
// and Crystal Barricade's prevention (noncombat damage to your other
// creatures).
func (lc *lowerCtx) damageRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if ok, err := lc.preventToSelf(rp, w, rl); ok || err != nil { // lower_preventtoself.go
		return err
	}
	if ok, err := lc.staticDamagePrevent(rp, w, rl); ok || err != nil { // lower_staticprevent.go
		return err
	}
	if rp.str("ActiveZones") != "Battlefield" {
		return refuse("repl-hard: DamageDone ActiveZones")
	}
	if rp.is("Prevent", "True") {
		if rp.str("ValidTarget") != "Creature.YouCtrl+Other" || !rp.is("IsCombat", "False") || w != nil {
			return refuse("repl-hard: DamageDone prevention shape")
		}
		rl.Kind = "PreventNoncombatToOthers"
		return nil
	}
	if rp.str("ValidSource") == "Card.YouCtrl,Emblem.YouCtrl" && rp.str("ValidTarget") == "Opponent,Permanent.OppCtrl" &&
		w != nil && withIs(w, "ReplaceEffect", map[string]string{"VarName": "DamageAmount", "VarValue": w.Params["VarValue"]}) &&
		lc.face.SVars[w.Params["VarValue"]] == "ReplaceCount$DamageAmount/Twice" {
		rl.Kind = "DoubleDamageToOpps"
		return nil
	}
	if ok, err := lc.preventMillRepl(rp, w, rl); ok || err != nil { // lower_replmult.go
		return err
	}
	if ok, err := lc.selfDamageRepl(rp, w, rl); ok || err != nil { // lower_replmult.go
		return err
	}
	if ok, err := lc.staticDmgRedirect(rp, w, rl); ok || err != nil { // lower_staticredirect.go
		return err
	}
	return lc.damageDoubler(rp, w, rl)
}

// graveToLibrary lowers a self "if this card would be put into a graveyard
// from anywhere, put it into its owner's library instead" line: Progenitus
// and the other reveal-and-shuffle cards, plus Gravebane Zombie's top and
// Nissa's Chosen's bottom (LibraryPosition$ 0 / -1, no shuffle). The
// replacement applies only to a move from the battlefield when the R: line
// names Origin$ Battlefield (or ActiveZones$ Battlefield); a missing Origin$
// is gorge's from-anywhere reading.
func (lc *lowerCtx) graveToLibrary(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ValidCard") != "Card.Self" {
		return refuse("repl-hard: Moved to Graveyard shape")
	}
	az := rp.str("ActiveZones")
	if az != "" && az != "Battlefield" {
		return refuse("repl-hard: Moved to Graveyard ActiveZones %q", az)
	}
	ro := rp.str("Origin")
	switch ro {
	case "", "All", "Any", "Battlefield":
	default:
		return refuse("repl-hard: Moved to Graveyard origin %q", ro)
	}
	if w == nil || w.API != "ChangeZone" || w.Sub != nil {
		return refuse("repl-hard: Moved to Graveyard shape")
	}
	bp := newParams(w.Params)
	bp.consume("Defined", "Destination", "Origin", "Hidden", "Reveal", "Shuffle", "LibraryPosition")
	if bp.str("Defined") != "ReplacedCard" || bp.str("Destination") != "Library" || len(bp.unread()) != 0 {
		return refuse("repl-hard: Moved to Graveyard shape")
	}
	bo := bp.str("Origin")
	switch bo {
	case "", "All", "Any", "Battlefield":
	default:
		return refuse("repl-hard: Moved to Graveyard body origin %q", bo)
	}
	if h, ok := bp.get("Hidden"); ok && !strings.EqualFold(strings.TrimSpace(h), "True") {
		return refuse("repl-hard: Moved to Graveyard Hidden %q", h)
	}
	if rv, ok := bp.get("Reveal"); ok && !strings.EqualFold(strings.TrimSpace(rv), "True") {
		return refuse("repl-hard: Moved to Graveyard Reveal %q", rv)
	}
	shuffle := false
	if sh, ok := bp.get("Shuffle"); ok {
		if !strings.EqualFold(strings.TrimSpace(sh), "True") {
			return refuse("repl-hard: Moved to Graveyard Shuffle %q", sh)
		}
		shuffle = true
	}
	bottom := false
	if pos, ok := bp.get("LibraryPosition"); ok {
		switch strings.TrimSpace(pos) {
		case "0":
		case "-1":
			bottom = true
		default:
			return refuse("repl-hard: Moved to Graveyard LibraryPosition %q", pos)
		}
		if shuffle {
			return refuse("repl-hard: Moved to Graveyard LibraryPosition with Shuffle")
		}
	} else if !shuffle {
		return refuse("repl-hard: Moved to Graveyard shape")
	}
	rl.Kind = "GraveToLibrary"
	rl.LibBottom, rl.LibShuffle = bottom, shuffle
	rl.LibFromBattlefield = ro == "Battlefield" || az == "Battlefield"
	return nil
}

// exileOnLeave lowers the dies-to-exile shape: a non-self permanent
// matching ValidCard/ValidLKI is exiled instead of put into a graveyard.
// Only battlefield-origin shapes are exact: an explicit Origin$ Battlefield,
// a ValidLKI Creature.* (gorge's implicit battlefield gate), or a Permanent
// filter (permanents exist only on the battlefield). The body must be a plain
// ChangeZone of ReplacedCard to Exile; Hidden is accepted (it changes nothing
// for a Defined$ object and the kernel does not model face-down exile).
func (lc *lowerCtx) exileOnLeave(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	vc, vl := rp.str("ValidCard"), rp.str("ValidLKI")
	if vc == "" && vl == "" {
		return false, nil
	}
	if vc == "Card.Self" {
		return lc.selfExile(rp, w, rl)
	}
	if strings.HasPrefix(vc, "Card.Self+") {
		return false, nil // other self replacements Progenitus, the library forms
	}
	ctr, ctrN, bodyOK := exileBody(w)
	tokRider := false
	if !bodyOK {
		var claimed bool
		var err error
		if tokRider, claimed, err = exileRiderClaim(w); err != nil || !claimed { // lower_exilectrrider.go
			return err != nil, err
		}
	}
	if az := rp.str("ActiveZones"); az != "" && az != "Battlefield" {
		return true, refuse("repl-hard: dies-to-exile ActiveZones %q", az)
	}
	origin := rp.str("Origin")
	battlefield := origin == "Battlefield"
	if !battlefield {
		switch origin {
		case "", "All", "Any":
			battlefield = permanentSpec(vc) || strings.HasPrefix(vl, "Creature.")
		}
	}
	anyZone := false
	if !battlefield {
		// "from anywhere": no Origin$ (or All/Any) and a card spec. gorge's
		// replacementMatches then takes a move to a graveyard from every zone
		// an explicit other origin and ValidLKI stay refused.
		switch origin {
		case "", "All", "Any":
			anyZone = vl == ""
		}
		if !anyZone {
			return true, refuse("repl-hard: dies-to-exile from any zone")
		}
	}
	spec := vc
	if spec == "" {
		spec = vl
	} else if vl != "" {
		return true, refuse("repl-hard: dies-to-exile ValidCard and ValidLKI")
	}
	f, err := lc.filter(spec)
	if err != nil {
		return true, err
	}
	if anyZone {
		for _, c := range f.Clauses {
			// the kernel matches the moving card by what it is (types, colours,
			// subtypes, owner, token); a controller or a stat clause on a card
			// in a hidden zone is not read exactly
			if c.Ctrl != whoAny || c.CmpStat != cmpNone || c.BasePermanent || c.SpellBase || c.Unblocked ||
				c.Flags&^(fToken|fNonToken) != 0 {
				return true, refuse("repl-hard: dies-to-exile from any zone with a controller or stat clause")
			}
		}
		rl.AnyZone = true
	}
	if tokRider {
		if err := lc.exileRiderProg(w, anyZone, rl); err != nil {
			return true, err
		}
	}
	rl.Kind = "ExileOnLeave"
	rl.F = f
	rl.Ctr, rl.CtrN = ctr, int(ctrN)
	return true, nil
}

// selfExile lowers the self "if this would be put into a graveyard from
// anywhere, exile it instead" body: ValidCard$ Card.Self with a plain
// ChangeZone of ReplacedCard to Exile. A missing Origin$ (or All/Any) is
// gorge's from-anywhere reading unless ActiveZones$ Battlefield keeps the
// ability to the battlefield; an explicit Origin$ Battlefield is the
// dies-to-exile reading (Possessed Skaab, Gloomshrieker).
func (lc *lowerCtx) selfExile(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	ctr, ctrN, bodyOK := exileBody(w)
	if !bodyOK {
		tok, claimed, err := exileRiderClaim(w) // lower_exilectrrider.go
		if err != nil || !claimed {
			return err != nil, err
		}
		if tok {
			// the rider runs from the replacing permanent, which here is the
			// dying card itself and is gone when the prog runs
			return true, refuse("repl-hard: dies-to-exile rider on a self replacement")
		}
	}
	az := rp.str("ActiveZones")
	if az != "" && az != "Battlefield" {
		return true, refuse("repl-hard: dies-to-exile ActiveZones %q", az)
	}
	anyZone := false
	switch rp.str("Origin") {
	case "Battlefield":
		// dies-to-exile; the kernel's battlefield matching reads it
	case "", "All", "Any":
		// a missing Origin$ is from anywhere (gorge replacementMatches); the
		// ability itself is still battlefield-only under ActiveZones
		anyZone = az != "Battlefield"
	default:
		return true, refuse("repl-hard: dies-to-exile origin %q", rp.str("Origin"))
	}
	f, err := lc.filter(rp.str("ValidCard"))
	if err != nil {
		return true, err
	}
	rl.Kind, rl.F, rl.AnyZone = "ExileOnLeave", f, anyZone
	rl.Ctr, rl.CtrN = ctr, int(ctrN)
	return true, nil
}

// permanentSpec: a bare Permanent base matches only battlefield objects.
func permanentSpec(vc string) bool {
	if !strings.HasPrefix(vc, "Permanent") {
		return false
	}
	if len(vc) == len("Permanent") {
		return true
	}
	switch vc[len("Permanent")] {
	case '.', '+', ',':
		return true
	}
	return false
}

// exileBody: DB$ ChangeZone | Defined$ ReplacedCard | Destination$ Exile
// (Origin All/Battlefield, Hidden optional), no sub-ability. The body may
// also put counters on the exiled card (WithCountersType$ and the optional
// WithCountersAmount$, default 1): Dauthi Voidwalker's VOID, Draugr
// Necromancer's ICE. ctr is "" when the body puts none.
func exileBody(w *cards.SA) (ctr string, n uint8, ok bool) {
	if w == nil || w.API != "ChangeZone" {
		return "", 0, false
	}
	if w.Sub != nil {
		return exileCounterChainBody(w) // lower_exilectrrider.go
	}
	bp := newParams(w.Params)
	if bp.str("Defined") != "ReplacedCard" || bp.str("Destination") != "Exile" {
		return "", 0, false
	}
	if o := bp.str("Origin"); o != "" && o != "All" && o != "Battlefield" {
		return "", 0, false
	}
	if h, ok := bp.get("Hidden"); ok && !strings.EqualFold(h, "True") {
		return "", 0, false
	}
	ctr = bp.str("WithCountersType")
	n = 1
	amountSet := false
	if v, ok := bp.get("WithCountersAmount"); ok {
		amountSet = true
		amount, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || amount < 1 || amount > 255 {
			return "", 0, false
		}
		n = uint8(amount)
	}
	if amountSet && ctr == "" {
		return "", 0, false // an amount with no kind is not a counter
	}
	if ctr != "" {
		// the kernel's exile-counter record holds one of the four kinds the
		// dies-to-exile bodies name, and a count up to 3 (repl_exilectr.go)
		switch strings.ToUpper(ctr) {
		case "VOID", "ICE", "BLOOD", "HIT":
		default:
			return "", 0, false
		}
		if n > 3 {
			return "", 0, false
		}
	}
	bp.consume("Defined", "Destination", "Origin", "Hidden", "WithCountersType", "WithCountersAmount")
	if len(bp.unread()) != 0 {
		return "", 0, false
	}
	return ctr, n, true
}

// addCounterRepl lowers the counter replacements: Doubling Season's
// unfiltered EffectOnly doubling, plus the filtered doublers and plus/minus
// one shapes (ValidCounterType, DB$ ReplaceCounter).
func (lc *lowerCtx) addCounterRepl(rp *params, w *cards.SA, rl *lRepl) error {
	if rp.str("ActiveZones") != "Battlefield" {
		return refuse("repl-hard: AddCounter ActiveZones")
	}
	if _, ok := rp.get("ValidPlayer"); ok {
		return lc.onePoisonRepl(rp, w, rl) // lower_delayed.go
	}
	if _, ok := rp.get("ValidSource"); ok {
		if rp.str("ValidObject") == youPlusObject {
			return lc.youPlusCountersRepl(rp, w, rl) // lower_counterrepl.go
		}
		if rp.str("ValidObject") == "Permanent.inZoneBattlefield,Player" {
			return lc.youDoubleCountersRepl(rp, w, rl) // lower_delayed.go
		}
		return lc.putterDoubleCountersRepl(rp, w, rl) // lower_counterrepl.go
	}
	vc := rp.str("ValidCard")
	if vc == "" {
		return refuse("repl-hard: AddCounter ValidCard")
	}
	if rp.str("ValidSource") != "" || rp.str("ValidObject") != "" {
		return refuse("repl-hard: AddCounter putter shape")
	}
	if rp.is("EffectOnly", "True") {
		// Doubling Season: ValidCard$ Permanent.YouCtrl+inZoneBattlefield,
		// DB$ ReplaceCounter | Amount$ Y, Y:ReplaceCount$CounterNum/Twice
		if vc != "Permanent.YouCtrl+inZoneBattlefield" || w == nil ||
			!withIs(w, "ReplaceCounter", map[string]string{"Amount": w.Params["Amount"]}) ||
			lc.face.SVars[w.Params["Amount"]] != "ReplaceCount$CounterNum/Twice" {
			return refuse("repl-hard: AddCounter replacement shape")
		}
		rl.Kind = "DoubleCounters"
		return nil
	}
	ck := rp.str("ValidCounterType")
	if ck == "" {
		return lc.anyKindCountersRepl(rp, w, rl, vc) // lower_counterrepl.go
	}
	if _, err := counterKind(ck); err != nil {
		return refuse("repl-hard: AddCounter counter type %q", ck)
	}
	if w == nil || w.API != "ReplaceCounter" || w.Sub != nil {
		return refuse("repl-hard: AddCounter body")
	}
	bp := newParams(w.Params)
	if bp.str("ValidCounterType") != ck {
		return refuse("repl-hard: AddCounter body counter type")
	}
	amt := bp.str("Amount")
	if cc, ok := bp.get("ChooseCounter"); ok && !strings.EqualFold(cc, "True") {
		return refuse("repl-hard: AddCounter ChooseCounter %q", cc)
	}
	bp.consume("ValidCounterType", "Amount", "ChooseCounter")
	if left := bp.unread(); len(left) > 0 {
		return refuse("repl-hard: AddCounter body params %v", left)
	}
	switch lc.face.SVars[amt] {
	case "ReplaceCount$CounterNum/Twice":
		rl.Kind, rl.Ctr = "DoubleCountersOn", ck
	case "ReplaceCount$CounterNum/Plus.1":
		rl.Kind, rl.N, rl.Ctr = "AddCountersOn", cst(1), ck
	case "ReplaceCount$CounterNum/Minus.1":
		rl.Kind, rl.N, rl.Ctr = "AddCountersOn", cst(-1), ck
	default:
		return refuse("repl-hard: AddCounter amount %q", lc.face.SVars[amt])
	}
	var err error
	if rl.F, err = lc.filter(vc); err != nil {
		return err
	}
	return nil
}

// chainElectionOK reports whether this Tap node's pay-life election lowers:
// the unchained shock shape (the whole replacement is the one unswitched
// Tap), or the ask-then-elect chain (Multiversal Passage): an as-enters
// basic-land ask whose SubAbility$ is the entering land's own pay-life
// election, the chain's last node. gorge parks the entry on each ask in
// order and its land play's count waits for the whole entry (settleLandPlay,
// landEntryParked); the kernel's count-deferral machinery (gfShockEntry,
// repl_shockland.go) reads the election behind the opener ask (electionPC)
// and carries the deferral across the chain (answerQueuedAsk). A rider
// behind any other chain, an others-enter election, or one with a condition
// of its own stays refused.
func chainElectionOK(sa, node *cards.SA) bool {
	return node == sa && sa.Sub == nil ||
		node != sa && sa.API == "ChooseType" && strings.TrimSpace(sa.Params["Type"]) == "Basic Land" &&
			sa.Sub == node && node.Sub == nil
}

// replChain lowers an ETB replacement's ReplaceWith chain: enters tapped,
// enters with counters, as-enters choices.
// A chained Tap/PutCounter node may carry its own ConditionCheckSVar$ gate (a
// SkipIf over that node); an absent Defined$ names the
// entering object.
func (lc *lowerCtx) replChain(sa *cards.SA, other bool, cond *lCond) error {
	for node := sa; node != nil; node = node.Sub {
		ps := newParams(node.Params)
		ps.consume("SubAbility")
		want := "Self"
		if other {
			want = "ReplacedCard"
		}
		// A chained node's own Condition gate (gorge registry.go:1226: a
		// failed gate skips this node only; the chain continues). The
		// unchained single-node shapes keep the whole-replacement condition
		// below (*cond), unchanged.
		chained := sa.Sub != nil || node != sa
		var ncond lCond
		if chained && (node.API == "Tap" || node.API == "PutCounter") {
			if cs, ok := ps.get("ConditionCheckSVar"); ok {
				if other {
					return refuse("per-node ETB gate on an others-enter replacement")
				}
				v, err := lc.value(cs)
				if err != nil {
					return err
				}
				ncond = lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
				if cmp, ok := ps.get("ConditionSVarCompare"); ok {
					op, b, err := lc.cmpSpec(cmp)
					if err != nil {
						return err
					}
					ncond.Op, ncond.B = op, b
				}
			}
		}
		gAt := -1
		if ncond.Kind != cNone {
			gAt = len(lc.ab.Prog)
			lc.emit(lInstr{Op: "SkipIf", Cond: ncond})
		}
		switch node.API {
		case "Tap":
			if d, hasD := ps.get("Defined"); !ps.is("ETB", "True") || hasD && strings.TrimSpace(d) != want {
				return refuse("ETB tap shape")
			}
			// The shock lands' as-enters election: "As CARDNAME enters, you
			// may pay N life. If you don't, it enters tapped." gorge poses
			// it through Resolve's generic unless gate (effects/registry.go
			// unlessProceed -> poseUnlessAsk) to UnlessPayer$ You, the
			// entering permanent's controller, and offers pay only while
			// life >= N (CR 119.4, UnlessCostPayableFromCtx). Only the exact
			// shape lowers: the whole replacement is the one unswitched Tap
			// with no other condition, and the cost is a literal PayLife<N> or
			// a one-card Reveal or Behold (revealLandCost). Anything else
			// refuses.
			if uc, ok := ps.get("UnlessCost"); ok {
				if other || cond.Kind != cNone || !chainElectionOK(sa, node) {
					return refuse("ETB tap UnlessCost on a chained ETB replacement")
				}
				if p, ok := ps.get("UnlessPayer"); !ok || p != "You" {
					return refuse("ETB tap UnlessCost without UnlessPayer$ You")
				}
				if sw, ok := ps.get("UnlessSwitched"); ok {
					return refuse("ETB tap UnlessCost UnlessSwitched$ %s", sw)
				}
				if verb, spec, ok := revealLandCost(uc); ok {
					if err := lc.revealLandElection(verb, spec); err != nil {
						return err
					}
					break
				}
				n, literal := 0, false
				if rest, has := strings.CutPrefix(strings.TrimSpace(uc), "PayLife<"); has {
					if arg, hasArg := strings.CutSuffix(rest, ">"); hasArg {
						if v, err := strconv.Atoi(arg); err == nil && v >= 1 {
							n, literal = v, true
						}
					}
				}
				cost, err := lc.costUnless(uc)
				if !literal || err != nil || cost.PayLife != n {
					return refuse("ETB tap UnlessCost %q", uc)
				}
				// UnlessPaid(PayLife N, You) then the Tap body: gorge's
				// unswitched orientation skips the body when the life is
				// paid and runs it on a decline.
				if err := lc.unless(ps, func() error {
					lc.emit(lInstr{Op: "EnterTapped", Obj: lRef{Kind: rReplaced}})
					return nil
				}); err != nil {
					return err
				}
				break
			}
			// ConditionPlayerTurn$ (gorge Resolve's Condition* gate): "enters
			// tapped if it's not your turn" -- the whole replacement is the
			// one Tap, so the gate becomes the replacement's condition
			// (CondYourTurn: the entering permanent's controller is active).
			if pt, ok := ps.get("ConditionPlayerTurn"); ok {
				if sa.Sub != nil || node != sa || cond.Kind != cNone || other {
					return refuse("ConditionPlayerTurn on a chained ETB replacement")
				}
				switch pt {
				case "True":
					*cond = lCond{Kind: cYourTurn}
				case "False":
					*cond = lCond{Kind: cYourTurn, Neg: true}
				default:
					return refuse("ConditionPlayerTurn %q", pt)
				}
			}
			// ConditionPresent$ + ConditionCompare$ (gorge
			// conditionMetBattlefield, conditions.go:1183): "enters tapped
			// unless you control <N> <spec>". The whole replacement is the
			// one Tap and gorge counts the battlefield for the entering
			// permanent's controller excluding the entering object itself
			// (kernel turn_gen.go clears the object from occ before the
			// replacement runs, so the count excludes it the same way).
			if cp, ok := ps.get("ConditionPresent"); ok {
				if other {
					return refuse("ConditionPresent on an others-enter replacement")
				}
				if sa.Sub != nil || node != sa || cond.Kind != cNone {
					return refuse("ConditionPresent on a chained ETB replacement")
				}
				cmp, ok := ps.get("ConditionCompare")
				if !ok {
					return refuse("ConditionPresent without ConditionCompare on an ETB replacement")
				}
				f, err := lc.filter(cp)
				if err != nil {
					return err
				}
				op, v, err := lc.cmpSpec(cmp)
				if err != nil {
					return err
				}
				*cond = lCond{Kind: cPresent, F: f, Op: op, A: v}
			}
			// ConditionCheckSVar$ (+ ConditionSVarCompare$): the replacement
			// applies while the SVar's value passes the comparison (gorge
			// Resolve's Condition* gate on the one Tap; Starting Town's
			// "unless it's your first, second, or third turn")
			if cs, ok := ps.get("ConditionCheckSVar"); ok && ncond.Kind == cNone {
				if cond.Kind != cNone {
					return refuse("ConditionCheckSVar beside another ETB replacement condition")
				}
				if sa.Sub != nil || node != sa || other {
					return refuse("ConditionCheckSVar on a chained ETB replacement")
				}
				v, err := lc.value(cs)
				if err != nil {
					return err
				}
				c := lCond{Kind: cValue, A: v, Op: opGE, B: cst(1)}
				if cmp, ok := ps.get("ConditionSVarCompare"); ok {
					op, b, err := lc.cmpSpec(cmp)
					if err != nil {
						return err
					}
					c.Op, c.B = op, b
				}
				*cond = c
			}
			lc.emit(lInstr{Op: "EnterTapped", Obj: lRef{Kind: rReplaced}})
		case "Untap":
			// "Lands you control enter untapped" (The Wandering Minstrel,
			// Spelunking): the plain ETB untap of the entering
			// card, ordered against an enters-tapped body by the kernel's
			// CR 616.1 entry-order ask. Only the unconditional single-node
			// other-enter shape: enterGen does not evaluate a condition on
			// an other-enter program (Archelos' Card.Self+tapped/untapped
			// pair stays refused).
			if d, hasD := ps.get("Defined"); !ps.is("ETB", "True") || hasD && strings.TrimSpace(d) != want {
				return refuse("ETB untap shape")
			}
			if !other || node != sa || sa.Sub != nil || cond.Kind != cNone {
				return refuse("conditional/chained/self ETB untap shape")
			}
			lc.emit(lInstr{Op: "EnterUntapped", Obj: lRef{Kind: rReplaced}})
		case "ChangeZone":
			// "creatures you control enter tapped and attacking" (Kira Nerys, Firebrand):
			// Origin$ All to the battlefield with Tapped$ True and Attacking$ True,
			// nothing else (gorge effects.EntryStateBody). Only the unchained other-enter shape.
			if !other || node != sa || sa.Sub != nil || ncond.Kind != cNone ||
				!ps.is("Origin", "All") || !ps.is("Destination", "Battlefield") || ps.str("Defined") != want ||
				!ps.is("Tapped", "True") || !ps.is("Attacking", "True") {
				return refuse("ETB replacement ChangeZone shape")
			}
			lc.emit(lInstr{Op: "EnterTapped", Obj: lRef{Kind: rReplaced}})
			lc.emit(lInstr{Op: "EnterAttacking", Obj: lRef{Kind: rReplaced}})
		case "PutCounter":
			if err := lc.etbPutCounter(ps, node, sa, other, cond, want); err != nil { // lower_enterctrpick.go
				return err
			}
		case "ChooseColor":
			if ps.str("Defined") != "You" {
				return refuse("ChooseColor Defined")
			}
			ex, err := chooseColorExclude(ps) // lower_handmove.go
			if err != nil {
				return err
			}
			lc.emit(lInstr{Op: "ChooseColor", Obj: lRef{Kind: rYou}, C: ex})
		case "GenericChoice":
			if err := lc.etbSetChosenMode(node, sa, other, cond); err != nil { // lower_siegemode.go
				return err
			}
			ps.consume("Choices", "Defined", "SetChosenMode", "ShowChoice", "AILogic", "SpellDescription")
		case "Clone":
			if err := lc.cloneEnter(ps, node, sa); err != nil {
				return err
			}
			if node == sa && sa.Sub != nil {
				// cloneEnter consumed the exact counter-rider chain into the
				// BecomeCopy flag; its links are not separate instructions.
				if left := ps.unread(); len(left) > 0 {
					return refuse("Clone unread params %v", left)
				}
				return nil
			}
		case "AlterAttribute":
			// "enters prepared" (K:ETBReplacement:Other:<SVar> with
			// AlterAttribute Prepared; Updated)
			if other || ps.str("Attributes") != "Prepared" {
				return refuse("ETB replacement api AlterAttribute")
			}
			if err := lc.alterPrepared(ps, lRef{Kind: rReplaced}); err != nil {
				return err
			}
		case "ChooseNumber":
			if err := lc.etbChooseNumber(ps, node, sa, other, cond); err != nil { // lower_choosenumber.go
				return err
			}
		case "ChoosePlayer":
			if !other && node == sa && !spyglassChain(sa) {
				// the as-enters opponent (lower_chooseplayerenter.go)
				if err := lc.etbChoosePlayer(ps, node, sa, other, cond); err != nil {
					return err
				}
				break
			}
			// Sorcerous Spyglass's chain head (lower_choosename.go spyglassChain):
			// the pod-wide choice of an opponent, ahead of the look-then-name
			// ask (choosePlayerPod)
			if other || node != sa || !spyglassChain(sa) {
				return refuse("ETB replacement api ChoosePlayer")
			}
			if err := lc.choosePlayerPod(ps); err != nil {
				return err
			}
		case "RevealHand":
			// the chain's look: folded into the name ask's decisions (B = 2)
			if other || !spyglassChain(sa) && !handNameChain(sa) {
				return refuse("ETB replacement api RevealHand")
			}
			ps.consume("Defined", "Look", "RememberRevealed", "SpellDescription")
		case "Cleanup":
			if other || !spyglassChain(sa) && !handNameChain(sa) {
				return refuse("ETB replacement api Cleanup")
			}
			lc.clearChosenPlayer(ps)
			ps.consume("ClearRemembered") // Alhammarret's chain: the revealed set was never kept
		case "NameCard":
			if err := lc.etbChooseName(ps, node, sa, other, cond); err != nil { // lower_choosename.go
				return err
			}
		case "ChooseCard":
			if err := lc.etbChooseCard(ps, node, sa, other, cond); err != nil { // lower_choosecardenter.go
				return err
			}
		case "ChooseType":
			if err := lc.etbChooseType(ps); err != nil { // lower_choosetype.go
				return err
			}
		default:
			return refuse("ETB replacement api %s", node.API)
		}
		if gAt >= 0 {
			lc.ab.Prog[gAt].Skip = len(lc.ab.Prog) - gAt - 1
		}
		if left := ps.unread(); len(left) > 0 {
			return refuse("%s unread params %v", node.API, left)
		}
	}
	return nil
}
