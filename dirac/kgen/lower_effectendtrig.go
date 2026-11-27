package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// effectEndingTrigger lowers the one Effect shape whose Triggers$ body only
// ends the Effect: a Duration$ Permanent Effect over its single targeted
// permanent (RememberObjects$ Targeted, ExileOnMoved$ Battlefield) with one
// Continuous static over Card.IsRemembered and one Static$ True trigger whose
// Execute is the Effect's own exile (DB$ ChangeZone, Origin$ Command,
// Destination$ Exile). The trigger is the Effect's lifetime, so no emblem is
// made: the static is an override record on the target (object-bound, CR
// 400.7) that carries the end on its Override row. Two triggers are read:
//
//   - Mode$ Phase | Phase$ Untap | ValidPlayer$ Player.controlsCard.IsRemembered
//     (Orcish Farmer, "until its controller's next untap step"):
//     kop.Override.UntilCtrlUntap;
//   - Mode$ SpellCast | ValidActivatingPlayer$ You | ValidCard$ <spec> |
//     TriggerZones$ Command (Soul Sculptor, "until a player casts a creature
//     spell", the script's own You): kop.Override.UntilCast, the spec as a
//     stack filter.
//
// done is false, with no error, for any Effect that is not this shape's Duration$
// Permanent pair, so the caller falls through to its other arms. Everything
// else that is this shape but not exact refuses by name.
func (lc *lowerCtx) effectEndingTrigger(ps *params, tr string) (done bool, err error) {
	if !ps.is("Duration", "Permanent") {
		return false, nil
	}
	list := ps.peek("StaticAbilities")
	why := func(more string) (bool, error) {
		return true, refuse("Effect StaticAbilities$ %q beside an ending Triggers$ (%s)", list, more)
	}
	// Ugin, the Ineffable's [+1]: an Effect over the exiled card it remembers
	// and the token its Token SubAbility imprints. Its MayLook static is
	// view-only and its ChangesZone trigger over Card.IsImprinted returns the
	// remembered card when the token leaves, then exiles itself.
	if strings.TrimSpace(ps.peek("RememberObjects")) == "RememberedLKI" {
		return lc.imprintTokenEffect(ps, tr, list)
	}
	t, ok := cards.ParseTriggerLine(lc.face.SVars[strings.TrimSpace(tr)])
	if !ok {
		return why("trigger " + tr)
	}
	end := strings.TrimSpace(t.Params["Execute"])
	if !effectSelfExile(lc.face.SVars, end) || strings.TrimSpace(t.Params["Static"]) != "True" {
		return why("trigger does not only exile the Effect")
	}
	if strings.TrimSpace(ps.peek("RememberObjects")) != "Targeted" || ps.peek("ExileOnMoved") != "Battlefield" {
		return why("RememberObjects$/ExileOnMoved$")
	}
	if _, has := ps.m["ValidTgts"]; !has || len(strings.Fields(list)) != 1 || strings.Contains(list, ",") {
		return why("one targeted static")
	}
	tp := newParams(t.Params)
	tp.consume("Mode", "Execute", "Static")
	o := &lOverride{}
	switch t.Mode {
	case "Phase":
		if tp.str("Phase") != "Untap" || tp.str("ValidPlayer") != "Player.controlsCard.IsRemembered" {
			return why("Phase trigger")
		}
		o.UntilCtrlUntap = true
	case "SpellCast":
		spec := tp.str("ValidCard")
		if tp.str("ValidActivatingPlayer") != "You" || tp.str("TriggerZones") != "Command" || spec == "" {
			return why("SpellCast trigger")
		}
		f, ferr := lc.filter(spec)
		if ferr != nil {
			return true, ferr
		}
		o.UntilCast = f
	default:
		return why("trigger Mode$ " + t.Mode)
	}
	if left := tp.unread(); len(left) > 0 {
		return why("trigger unread " + strings.Join(left, ","))
	}
	if err := lc.endingTriggerBody(list, o); err != nil {
		return true, err
	}
	capture, err := lc.objOrTarget(ps, "", zBattlefield)
	if err != nil {
		return true, err
	}
	if capture.Kind != rTarget {
		return why("RememberObjects$ Targeted over " + capture.String())
	}
	ps.consume("Duration", "RememberObjects", "ExileOnMoved", "StaticAbilities", "Triggers")
	lc.emitOverride(capture, o, "Permanent")
	return true, nil
}

// effectSelfExile reports whether SVar name is exactly an Effect's exile of
// itself from the command zone: DB$ ChangeZone, Origin$ Command, Destination$
// Exile, and Defined$ Self or no Defined$, with no other key.
func effectSelfExile(svars map[string]string, name string) bool {
	return selfExileSA(cards.ResolveSVar(svars, name))
}

// selfExileSA is effectSelfExile over an already-resolved body.
func selfExileSA(sa *cards.SA) bool {
	if sa == nil || sa.API != "ChangeZone" || sa.Sub != nil {
		return false
	}
	for k, v := range sa.Params {
		v = strings.TrimSpace(v)
		switch k {
		case "DB", "AB", "SP":
		case "Origin":
			if v != "Command" {
				return false
			}
		case "Destination":
			if v != "Exile" {
				return false
			}
		case "Defined":
			if v != "Self" {
				return false
			}
		case "StackDescription", "SpellDescription", "TriggerDescription", "Description":
		default:
			return false
		}
	}
	return strings.TrimSpace(sa.Params["Origin"]) == "Command" && strings.TrimSpace(sa.Params["Destination"]) == "Exile"
}

// imprintTokenEffect lowers Ugin, the Ineffable's DB$ Effect:
//
//	Duration$ Permanent | RememberObjects$ RememberedLKI |
//	ForgetOnMoved$ Exile | ImprintCards$ Imprinted |
//	StaticAbilities$ MayLook | Triggers$ TrigLeavesBattlefield
//
// The MayLook static is view-only (engine/rules layers MayLookAt over
// Card.IsRemembered, no event, decision or legality reader), so it lowers to
// no static. The remembered set is the Dig's exiled card; ImprintCards$
// Imprinted is the source's ImprintTokens association, i.e. the 2/2 Spirit the
// Token SubAbility just made. The one trigger is a ChangesZone
// Battlefield->Any over Card.IsImprinted whose body returns the remembered
// card from exile to hand and then exiles the Effect itself. The kernel emblem
// captures the token as its trigger (Capture; RefImprinted at creation) and
// the exiled card as its remembered set (CaptureRem), so both outlive the
// resolution; its body is the many-object return of that set, and the emblem
// is dropped by Once (the self-exile's observable effect).
func (lc *lowerCtx) imprintTokenEffect(ps *params, tr, list string) (bool, error) {
	why := func(more string) (bool, error) {
		return true, refuse("Effect StaticAbilities$ %q beside an ending Triggers$ (%s)", list, more)
	}
	if strings.TrimSpace(ps.peek("ForgetOnMoved")) != "Exile" {
		return why("ForgetOnMoved$ " + ps.peek("ForgetOnMoved"))
	}
	if strings.TrimSpace(ps.peek("ImprintCards")) != "Imprinted" {
		return why("ImprintCards$ " + ps.peek("ImprintCards"))
	}
	ss, err := lc.effectStaticBodies(list)
	if err != nil {
		return true, err
	}
	if len(ss) != 1 || ss[0].Mode != "Continuous" {
		return why("one Continuous static")
	}
	sp := newParams(ss[0].Params)
	sp.consume("Mode")
	if sp.str("Affected") != "Card.IsRemembered" || sp.str("AffectedZone") != "Exile" || sp.str("MayLookAt") != "You" {
		return why("MayLook static")
	}
	sp.consume("Affected", "AffectedZone", "MayLookAt")
	if left := sp.unread(); len(left) > 0 {
		return why("static unread " + strings.Join(left, ","))
	}
	t, ok := cards.ParseTriggerLine(lc.face.SVars[strings.TrimSpace(tr)])
	if !ok {
		return why("trigger " + tr)
	}
	if t.Mode != "ChangesZone" || strings.TrimSpace(t.Params["Origin"]) != "Battlefield" ||
		strings.TrimSpace(t.Params["Destination"]) != "Any" ||
		strings.TrimSpace(t.Params["ValidCard"]) != "Card.IsImprinted" ||
		strings.TrimSpace(t.Params["TriggerZones"]) != "Command" {
		return why("ChangesZone trigger")
	}
	exec := strings.TrimSpace(t.Params["Execute"])
	body := cards.ResolveSVar(lc.face.SVars, exec)
	if !returnRememberedToHandSA(body) || !selfExileSA(body.Sub) {
		return why("Execute$ " + exec)
	}
	if t.Effect == nil {
		t.Effect = body
	}
	ps.consume("Duration", "RememberObjects", "ForgetOnMoved", "ImprintCards", "StaticAbilities", "Triggers")
	// The kernel emblem's Capture gate is the imprinted token, so the
	// trigger's Card.IsImprinted word is read by that gate, not the filter.
	cp := make(map[string]string, len(t.Params))
	for k, v := range t.Params {
		if k != "ValidCard" {
			cp[k] = v
		}
	}
	cp["ValidCard"] = "Card"
	t.Params = cp
	lc.granted++ // TriggerZones$ Command on an Effect-created trigger (emblem)
	saved := len(lc.out.Abilities)
	err = lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "Command") })
	lc.granted--
	if err != nil {
		return true, err
	}
	if len(lc.out.Abilities) != saved+1 {
		return true, refuse("imprint-token trigger lowering")
	}
	ab := lc.out.Abilities[saved]
	// The body is DBReturn (remembered Exile->Hand) then DBExile (Self
	// Command->Exile). The emblem's Once is the self-exile's observable end.
	if len(ab.Prog) != 2 || ab.Prog[1].Op != "Exile" || ab.Prog[1].Obj.Kind != rSelf {
		return true, refuse("imprint-token trigger body")
	}
	ab.Prog = ab.Prog[:1]
	ab.Prog[0].CS = strings.TrimPrefix(ab.Prog[0].CS+"|DelayedMany", "|")
	ab.Latent = true
	ab.Name = "EmblemTrigger"
	ab.EmblemRem = true
	lc.out.Emblems = append(lc.out.Emblems, lEmblem{
		Trigger:    saved,
		WhileHost:  true, // gorge's Duration$ Permanent Effect is |SB source-bounded
		Capture:    lRef{Kind: rImprinted},
		CaptureSet: true,
		CaptureRem: true,
		Once:       true,
	})
	lc.emit(lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rYou}, C: len(lc.out.Emblems) - 1})
	return true, nil
}

// returnRememberedToHandSA reports whether sa is exactly the imprint-token
// Effect's body move: DB$ ChangeZone, Defined$ RememberedLKI, Origin$ Exile,
// Destination$ Hand, with no other key but its SubAbility.
func returnRememberedToHandSA(sa *cards.SA) bool {
	if sa == nil || sa.API != "ChangeZone" {
		return false
	}
	for k, v := range sa.Params {
		v = strings.TrimSpace(v)
		switch k {
		case "DB", "AB", "SP", "SubAbility":
		case "Defined":
			if v != "RememberedLKI" {
				return false
			}
		case "Origin":
			if v != "Exile" {
				return false
			}
		case "Destination":
			if v != "Hand" {
				return false
			}
		case "StackDescription", "SpellDescription", "TriggerDescription", "Description":
		default:
			return false
		}
	}
	return strings.TrimSpace(sa.Params["Origin"]) == "Exile" && strings.TrimSpace(sa.Params["Destination"]) == "Hand"
}

// endingTriggerBody reads the Effect's one static into o (the record the
// Effect applies to its target). A body that adds only basic land types with
// RemoveLandTypes$ True is the CR 305.7 setter the Animate path lowers
// (animateLandTypeSetter); every other body is the StaticEffect$ reading
// (staticEffectOverride), the one a battlefield Continuous override shares.
func (lc *lowerCtx) endingTriggerBody(name string, ov *lOverride) error {
	name = strings.TrimSpace(name)
	body := lc.face.SVars[name]
	ss, ok := cards.ParseStaticLines(body)
	if !ok || len(ss) != 1 || ss[0].Mode != "Continuous" {
		return refuse("Effect static %q", body)
	}
	sp := newParams(ss[0].Params)
	sp.consume("Mode")
	if sp.str("Affected") != "Card.IsRemembered" {
		return refuse("Effect static affected %q", sp.str("Affected"))
	}
	var o *lOverride
	if sp.is("RemoveLandTypes", "True") {
		o = &lOverride{}
		words := strings.Split(sp.str("AddType"), "&")
		for _, w := range words {
			w = strings.TrimSpace(w)
			if w == "" || !lc.known(w) {
				return refuse("Effect RemoveLandTypes$ adds %q", w)
			}
			o.AddSubtypes = append(o.AddSubtypes, w)
		}
		sp.consume("Affected", "AddType", "RemoveLandTypes")
		if left := sp.unread(); len(left) > 0 {
			return refuse("Effect static unread %v", left)
		}
		if err := animateLandTypeSetter(o, false); err != nil {
			return err
		}
	} else {
		var err error
		if o, err = lc.staticEffectOverride(name); err != nil {
			return err
		}
	}
	o.UntilCtrlUntap, o.UntilCast = ov.UntilCtrlUntap, ov.UntilCast
	*ov = *o
	return nil
}
