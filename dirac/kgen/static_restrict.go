package kgen

import (
	"strconv"
	"strings"
)

// The bodies of the restriction statics (static_mode.go).
func init() {
	staticMode("CantBeCast", (*staticCtx).modeCantBeCast)
	staticMode("CantBeActivated", (*staticCtx).modeCantBeActivated)
	staticMode("CantTarget", (*staticCtx).modeCantTarget)
	staticMode("CantBeBeamedUp", (*staticCtx).modeCantBeBeamedUp)
	staticMode("CantGainLife", (*staticCtx).modeCantGainLife)
	staticMode("CantDraw", (*staticCtx).modeCantGainLife)
	staticMode("CantPreventDamage", (*staticCtx).modeCantPreventDamage)
	staticMode("CantPutCounter", (*staticCtx).modeCantPutCounter)
	staticMode("CantPhaseIn", (*staticCtx).modeCantPhaseIn)
	staticMode("CantSacrifice", (*staticCtx).modeCantSacrifice)
}

func (sc *staticCtx) modeCantBeCast() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	if err := lc.cantBeCast(sp, &sc.st); err != nil {
		return nil, true, err
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCantBeActivated() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	if err := lc.cantActivate(sp, &sc.st); err != nil {
		return nil, true, err // lower_cantact.go
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCantTarget() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	if err := lc.cantTarget(sp, &sc.st); err != nil {
		return nil, true, err // lower_canttarget.go
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCantBeBeamedUp() ([]lStatic, bool, error) {
	sp := sc.sp
	readF := sc.readF
	var err error
	sp.consume("Secondary")
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
	}
	if v, ok := sp.get("ValidSA"); ok {
		if v != "Spell.KeywordSource" {
			return nil, true, refuse("CantBeBeamedUp ValidSA %q", v)
		}
		sc.st.Flags = append(sc.st.Flags, "BeamGranted")
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCantGainLife() ([]lStatic, bool, error) {
	s := sc.s
	sp := sc.sp
	switch v := sp.str("ValidPlayer"); v {
	case "", "Player":
	case "Opponent", "Player.Opponent":
		// gorge lifeGainForbidden / drawForbidden: the spec is matched from
		// the static's controller, so Opponent is every other seat
		sc.st.Flags = append(sc.st.Flags, "BanOpp")
	default:
		return nil, true, refuse("%s ValidPlayer %q", s.Mode, v)
	}
	sp.consume("ValidPlayer", "Secondary")
	// DrawLimit$ N (Narset, Parter of Veils; Spirit of the Labyrinth): a
	// per-turn cap, gorge drawForbidden: the draw is forbidden once the
	// player has drawn N cards this turn. It rides Static.A as N+1 (0 = the
	// plain ban); the kernel's playerBanned compares the drawn-this-turn count.
	if raw, ok := sp.get("DrawLimit"); ok {
		if s.Mode != "CantDraw" {
			return nil, true, refuse("%s DrawLimit$", s.Mode)
		}
		n, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || n < 0 || n > 254 {
			return nil, true, refuse("CantDraw DrawLimit %q", raw)
		}
		sc.st.Amount = cst(n + 1)
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCantPreventDamage() ([]lStatic, bool, error) {
	sp := sc.sp
	// the blanket form, or IsCombat$ True (combat damage only: Frenzied Baloth); ValidSource$, EffectZone$, Affected$ stay unread (gorge cantPreventDamage)
	if c, ok := sp.get("IsCombat"); ok {
		if !strings.EqualFold(strings.TrimSpace(c), "True") {
			return nil, true, refuse("CantPreventDamage IsCombat %q", c)
		}
		sp.consume("IsCombat")
		sc.st.Flags = append(sc.st.Flags, "NoPrevCombat")
	}
	sp.consume("Secondary")
	// Banefire's "the damage can't be prevented" (X is 5 or more, CheckSVar$ read
	// above): a stack-zone static of the spell itself. Only the pair lowers.
	if vs, ok := sp.get("ValidSource"); ok {
		if strings.TrimSpace(vs) != "Spell.Self" || sp.str("EffectZone") != "Stack" {
			return nil, true, refuse("CantPreventDamage ValidSource$ %q", vs)
		}
		sc.st.Flags = append(sc.st.Flags, "NoPrevSpellSelf")
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCantPutCounter() ([]lStatic, bool, error) {
	lc := sc.lc
	sp := sc.sp
	readF := sc.readF
	var err error
	// gorge PutCounterBlocked's object form (rules/layers_restrict.go):
	// ValidCard$ (or ValidObject$) scopes the permanent, CounterType$
	// the kind (absent: every kind). The player form is a separate
	// ValidPlayer$ reader the kernel does not have; refuse it rather
	// than turn it into a no-op object static.
	if vp := sp.str("ValidPlayer"); vp != "" {
		return nil, true, refuse("CantPutCounter ValidPlayer %q (player counters are not lowered)", vp)
	}
	for _, k := range []string{"ActiveZones", "AffectedZone"} {
		if z, ok := sp.get(k); ok && strings.TrimSpace(z) != "Battlefield" {
			return nil, true, refuse("CantPutCounter %s$ %q", k, z)
		}
	}
	if vc := sp.str("ValidCard"); vc != "" {
		if sc.st.F, err = readF("ValidCard"); err != nil {
			return nil, true, err
		}
	} else if vo := sp.str("ValidObject"); vo != "" {
		if sc.st.F, err = readF("ValidObject"); err != nil {
			return nil, true, err
		}
	} else {
		// an unscoped line forbids every permanent (match-all filter)
		if sc.st.F, err = lc.filter("Card"); err != nil {
			return nil, true, err
		}
	}
	sc.st.Ctr = -1 // every kind until CounterType$ names one
	if ct, ok := sp.get("CounterType"); ok {
		if ct = strings.TrimSpace(ct); ct != "" && ct != "Any" {
			ck, err := counterKind(ct)
			if err != nil {
				return nil, true, refuse("CantPutCounter CounterType$ %q", ct)
			}
			sc.st.Ctr = int16(ck)
		}
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCantPhaseIn() ([]lStatic, bool, error) {
	sp := sc.sp
	// gorge phaseBlocked (rules/cantphase.go): only the blanket form,
	// every phased-out permanent (The City on the Edge of Forever's
	// token); a narrower ValidCard$ (Teferi's remembered set) needs the
	// kernel's remembered-set filter and is refused.
	if v := strings.TrimSpace(sp.str("ValidCard")); v != "Card.phasedOutPermanent" {
		return nil, true, refuse("CantPhaseIn ValidCard$ %q", v)
	}
	return nil, false, nil
}

func (sc *staticCtx) modeCantSacrifice() ([]lStatic, bool, error) {
	sp := sc.sp
	readF := sc.readF
	var err error
	// gorge sacrificeBlocked (rules/layers_restrict.go): ValidCard$ the
	// permanents, ActiveZones$ Battlefield the only zone it reads. The
	// cause-scoped lines (ValidCause$, ForCost$: Sigarda, Host of Herons)
	// need the sacrifice's cause and cost-or-effect provenance at every
	// choke point, which the kernel does not carry.
	for _, key := range []string{"ValidCause", "ForCost"} {
		if v, ok := sp.get(key); ok {
			return nil, true, refuse("CantSacrifice %s$ %q", key, v)
		}
	}
	if z, ok := sp.get("ActiveZones"); ok && strings.TrimSpace(z) != "Battlefield" {
		return nil, true, refuse("CantSacrifice ActiveZones$ %q", z)
	}
	sp.consume("Secondary")
	if sc.st.F, err = readF("ValidCard"); err != nil {
		return nil, true, err
	}
	return nil, false, nil
}
