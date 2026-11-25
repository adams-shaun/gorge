package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// damageInstead lowers the printed DamageDone replacements whose body puts the
// damage to another use (gorge rules/replacement.go applyNonMoveReplacements:
// the body runs at the proposal and hands back a stopped event, so nothing is
// dealt, no lifelink, no damage trigger; PreventionEffect$ True only matters
// under a can't-prevent effect, CR 615.12, where the body still runs and the
// damage stands):
//
//   - Angel of Suffering: ValidTarget$ You, Mill | Defined$ ReplacedTarget
//     with twice the damage (ReplaceCount$DamageAmount/Twice);
//   - Gloom Surgeon: combat damage to itself, Dig | ChangeNum$ All |
//     DestinationZone$ Exile of its controller's library;
//   - Force Bubble / Panther Habit / Soul-Scar Mage: PutCounter on Self, on
//     Equipped, on ReplacedTarget;
//   - Swans of Bryn Argoll: Draw | Defined$ ReplacedSourceController;
//   - Szadek, Undead Alchemist: combat damage by a source to a player, a
//     PutCounter on Self and a Mill of ReplacedTarget;
//   - Dralnu, Lich Lord: Sacrifice | Defined$ You | SacValid$ Permanent |
//     Amount$ X, its controller's choice inside the damage event;
//   - Phyrexian Vindicator: an ImmediateTrigger with RememberSVarAmount$ X
//     whose Execute is a DealDamage of Count$TriggerRememberAmount at
//     ValidTgts$ Creature.Other,Player,Planeswalker.Other,Battle.Other — the
//     CR 603.12 reflexive trigger the body queues (Repl.Trig), its target
//     chosen as the trigger is put on the stack.
//
// ValidTarget$ is the words of kop.ReplDmgYou / ReplDmgSelf / ReplDmgPlayer or
// one object filter alternative (Repl.Filter, read with the host as its
// source); ValidSource$ one filter (Repl.Filter2); IsCombat$ the ReplDmgCombat
// and ReplDmgNonCombat bits. false: not this shape (the doublers and the
// other body shapes read their own).
func (lc *lowerCtx) damageInstead(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	if w == nil || !insteadAPI[w.API] {
		return false, nil
	}
	if rp.str("ActiveZones") != "Battlefield" {
		return true, refuse("repl-hard: DamageDone instead ActiveZones")
	}
	body, ctr, trig, err := lc.insteadBody(w)
	if err != nil {
		return true, err
	}
	if rp.is("PreventionEffect", "True") {
		if body&kop.ReplDmgInsSac != 0 {
			return true, refuse("repl-hard: DamageDone instead sacrifice body with PreventionEffect") // the runtime's sacrifice body has no CR 615.12 stand
		}
		body |= kop.ReplDmgInsPrevent
	} else if _, ok := rp.get("PreventionEffect"); ok {
		return true, refuse("repl-hard: DamageDone instead PreventionEffect %q", rp.str("PreventionEffect"))
	}
	rp.consume("AlwaysReplace") // gorge reads neither AlwaysReplace$ nor ExecuteMode$ (a combat event is one per source and recipient)
	rp.consume("ExecuteMode")
	bits := 0
	targets, ok := rp.get("ValidTarget")
	if !ok {
		return true, refuse("repl-hard: DamageDone instead without ValidTarget")
	}
	switch tt := strings.TrimSpace(targets); tt {
	case "You":
		bits |= kop.ReplDmgYou
	case "Card.Self":
		bits |= kop.ReplDmgSelf
	case "Player":
		bits |= kop.ReplDmgPlayer
	default:
		// one permanent filter alternative: the damaged permanent matches it
		// with the replacement's host as its source (Creature.OppCtrl,
		// Card.EquippedBy)
		if strings.Contains(tt, ",") {
			return true, refuse("repl-hard: DamageDone instead ValidTarget %q", targets)
		}
		if rl.F, err = lc.filter(tt); err != nil {
			return true, err
		}
		if rl.F.Players != 0 || len(rl.F.Clauses) == 0 {
			return true, refuse("repl-hard: DamageDone instead ValidTarget %q", targets)
		}
	}
	if vs, ok := rp.get("ValidSource"); ok {
		if rl.F2, err = lc.filter(vs); err != nil {
			return true, err
		}
		if rl.F2.Players != 0 || len(rl.F2.Clauses) == 0 {
			return true, refuse("repl-hard: DamageDone instead ValidSource %q", vs)
		}
	}
	if c, ok := rp.get("IsCombat"); ok {
		switch strings.TrimSpace(c) {
		case "True":
			bits |= kop.ReplDmgCombat
		case "False":
			bits |= kop.ReplDmgNonCombat
		default:
			return true, refuse("repl-hard: DamageDone instead IsCombat %q", c)
		}
	}
	// a body that acts on the damaged player or permanent needs a recipient of
	// that kind: the player bodies a player, the permanent counters a permanent
	if body&kop.ReplDmgInsMill != 0 && bits&(kop.ReplDmgYou|kop.ReplDmgPlayer) == 0 {
		return true, refuse("repl-hard: DamageDone instead mill of a non-player recipient")
	}
	if body&kop.ReplDmgInsCtrTarget != 0 && rl.F == nil {
		return true, refuse("repl-hard: DamageDone instead counters on a non-permanent recipient")
	}
	rl.Kind, rl.Bits, rl.Body, rl.Ctr, rl.Trig = "DamageInstead", bits, body, ctr, trig
	return true, nil
}

// insteadAPI are the body APIs damageInstead reads. DealDamage is not one:
// it reaches the lowering only as an ImmediateTrigger's Execute (below); a
// SubAbility$-chained DealDamage node is off the shape.
var insteadAPI = map[string]bool{"Mill": true, "Dig": true, "PutCounter": true, "Draw": true, "Sacrifice": true, "ImmediateTrigger": true}

// insteadAmount reads a body's amount param (an SVar or inline) as the event's
// damage: ReplaceCount$DamageAmount, or with /Twice twice that. ok false: any
// other amount.
func (lc *lowerCtx) insteadAmount(v string) (twice, ok bool) {
	v = strings.TrimSpace(v)
	if sv, has := lc.face.SVars[v]; has {
		v = strings.TrimSpace(sv)
	}
	switch v {
	case "ReplaceCount$DamageAmount":
		return false, true
	case "ReplaceCount$DamageAmount/Twice":
		return true, true
	}
	return false, false
}

// insteadBody reads the body chain of a damageInstead replacement: its effect
// bits (kop.ReplDmgIns*), counter kind name and ask-body reflexive trigger.
// Each effect kind appears once; the kernel applies them in a fixed order
// (counters, mill, exile, draw), the order of the printed chains.
func (lc *lowerCtx) insteadBody(w *cards.SA) (body int, ctr string, trig int, err error) {
	seen := map[string]bool{}
	for sa := w; sa != nil; sa = sa.Sub {
		if !insteadAPI[sa.API] || seen[sa.API] {
			return 0, "", 0, refuse("repl-hard: DamageDone instead body")
		}
		seen[sa.API] = true
		bp := newParams(sa.Params)
		bp.consume("SubAbility")
		fail := func() (int, string, int, error) {
			return 0, "", 0, refuse("repl-hard: DamageDone instead %s body", sa.API)
		}
		switch sa.API {
		case "Mill":
			bp.consume("Defined", "NumCards")
			twice, ok := lc.insteadAmount(sa.Params["NumCards"])
			if !ok || bp.str("Defined") != "ReplacedTarget" {
				return fail()
			}
			body |= kop.ReplDmgInsMill
			if twice {
				body |= kop.ReplDmgInsDouble
			}
		case "Dig":
			bp.consume("DigNum", "ChangeNum", "ChangeValid", "DestinationZone")
			if twice, ok := lc.insteadAmount(sa.Params["DigNum"]); !ok || twice || bp.str("ChangeNum") != "All" ||
				bp.str("ChangeValid") != "Card" || bp.str("DestinationZone") != "Exile" {
				return fail()
			}
			body |= kop.ReplDmgInsExileTop
		case "Draw":
			bp.consume("Defined", "NumCards")
			if twice, ok := lc.insteadAmount(sa.Params["NumCards"]); !ok || twice || bp.str("Defined") != "ReplacedSourceController" {
				return fail()
			}
			body |= kop.ReplDmgInsDrawSrc
		case "PutCounter":
			bp.consume("Defined", "CounterType", "CounterNum")
			twice, ok := lc.insteadAmount(sa.Params["CounterNum"])
			if !ok || twice {
				return fail()
			}
			switch bp.str("Defined") {
			case "Self":
				body |= kop.ReplDmgInsCtrHost
			case "Equipped":
				body |= kop.ReplDmgInsCtrEquipped
			case "ReplacedTarget":
				body |= kop.ReplDmgInsCtrTarget
			default:
				return fail()
			}
			ctr = bp.str("CounterType")
			if _, err := counterKind(ctr); err != nil {
				return fail()
			}
		case "Sacrifice":
			// Dralnu, Lich Lord: its controller sacrifices that many permanents
			// of their choice (forge's eligible count is the ask's size, so the
			// body never asks when fewer remain: gorge startSac's cnt > n).
			bp.consume("Defined", "SacValid", "Amount")
			if bp.str("Defined") != "You" || bp.str("SacValid") != "Permanent" {
				return fail()
			}
			if twice, ok := lc.insteadAmount(sa.Params["Amount"]); !ok || twice {
				return fail()
			}
			body |= kop.ReplDmgInsSac
		case "ImmediateTrigger":
			// Phyrexian Vindicator: the damage is prevented (PreventionEffect$
			// True, read by damageInstead) and the Execute's DealDamage is the
			// CR 603.12 reflexive trigger the body queues (gorge
			// effImmediateTrigger's QueueReflexiveTrigger: its target is chosen
			// as it is put on the stack, the ordinary trigger target ask). The
			// Execute lowers as the latent's own program (its target walk is a
			// normal ability target, its amount the trigger's own), and the
			// body bit carries the latent's ability index for the runtime's
			// enqueue. Another shape is off the body.
			bp.consume("Execute", "RememberSVarAmount")
			if twice, ok := lc.insteadAmount(sa.Params["RememberSVarAmount"]); !ok || twice {
				return fail()
			}
			ex := cards.ResolveSVar(lc.face.SVars, sa.Params["Execute"])
			if ex == nil || strings.TrimSpace(ex.API) != "DealDamage" {
				return fail()
			}
			dp := newParams(ex.Params)
			dp.consume("TgtPrompt")
			if tf, ferr := lc.filter(dp.str("ValidTgts")); ferr != nil {
				return fail()
			} else if tf.Players != plBoth || !anyOtherTarget(tf) {
				return fail()
			}
			dp.consume("ValidTgts") // the chain lowers it (the latent's own target walk)
			n := strings.TrimSpace(dp.str("NumDmg"))
			if sv, has := lc.face.SVars[n]; has {
				n = strings.TrimSpace(sv)
			}
			if n != "Count$TriggerRememberAmount" {
				return fail()
			}
			if len(dp.unread()) != 0 {
				return fail()
			}
			dp.m["NumDmg"] = "TriggerAmount" // the reflexive trigger's own amount
			lat := lc.newLatent("ReflexiveTrigger")
			if err := lc.withAbility(lat, func() error { return lc.chain(ex, dp) }); err != nil {
				return 0, "", 0, err
			}
			body |= kop.ReplDmgInsVind
			trig = lat.Index
		}
		if len(bp.unread()) != 0 {
			return fail()
		}
	}
	return body, ctr, trig, nil
}

// replInstead builds a DamageInstead replacement: C the recipient bits and
// the ask bodies above them (kop.ReplDmgInsSac / ReplDmgInsVind), Filter the
// damaged permanent's filter, Filter2 the damage source's, Trig the
// queued reflexive trigger (ReplDmgInsVind), A the effect bits with the
// counter kind above kop.ReplDmgInsKindShift.
func (cb *cardBuilder) replInstead(r lRepl, rp *kop.Repl) (err error) {
	rp.Kind = kop.ReplDamageInstead
	const bodyMask = 0xff | kop.ReplDmgInsSac | kop.ReplDmgInsVind
	if r.Body&^bodyMask != 0 || r.Body&^kop.ReplDmgInsPrevent == 0 {
		return fmt.Errorf("damage instead body %#x", r.Body)
	}
	rp.A, rp.C = int16(r.Body), int16(r.Bits)|int16(r.Body&^0xff)
	if r.Ctr != "" {
		k, err := counterKind(r.Ctr)
		if err != nil {
			return err
		}
		if int(k) >= 1<<(15-kop.ReplDmgInsKindShift) {
			return fmt.Errorf("damage instead counter kind %d", k)
		}
		rp.A |= int16(k) << kop.ReplDmgInsKindShift
	}
	if r.F != nil {
		if rp.Filter, err = cb.filter(r.F, kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	if r.F2 != nil {
		if rp.Filter2, err = cb.filter(r.F2, kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	if r.Trig != 0 {
		rp.Trig = uint8(r.Trig)
	}
	return nil
}
