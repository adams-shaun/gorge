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
	sa := cards.ResolveSVar(svars, name)
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
