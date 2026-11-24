package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// reduceAbility lowers a ReduceCost static of activated abilities (Type$
// Ability, or no Type$ with ValidSpell$ Activated.Exhaust,
// Activated.PowerUp or Activated.Equip) and of Room unlock special actions (ValidSpell$
// Static.Unlock): "activated abilities of <permanents you control> cost {N}
// less", "unlock costs you pay cost {N} less" (Glimmer), and "power-up
// abilities of <permanents you control> cost {N} less" (Hulk, Gamma Goliath)
// (gorge statics_costapply.go costStaticGateFull). It reads exactly the shape
// kop.StatReduceAbility prices: the battlefield, the controller's own
// activations, ValidCard$ matched against the ability's source (or the
// unlocked Room), an Exhaust-, Power-up- or Equip-only restriction (the
// equip arm optionally gated on its target, ValidTarget$), and the unlock
// arm's B bit. Any other Activated.<class> and anything else refuses with a
// named reason.
func (lc *lowerCtx) reduceAbility(sp *params, st *lStatic, readF func(string) (*lFilter, error)) error {
	arm := 1
	if vs, ok := sp.get("ValidSpell"); ok {
		switch vs {
		case "Activated.Exhaust":
			arm = 2
		case "Static.Unlock":
			arm = 3
		case "Activated.PowerUp":
			arm = 4
		case "Activated.Equip":
			arm = 5
		default:
			return refuse("ReduceCost ValidSpell %q", vs)
		}
	}
	if t, ok := sp.get("Type"); ok && t != "Ability" {
		return refuse("ReduceCost Type %q", t)
	}
	if arm == 3 {
		if sp.str("Type") == "Ability" {
			return refuse("ReduceCost Type$ Ability with ValidSpell$ Static.Unlock")
		}
		// AffectedZone$ is optional for the unlock arm: the special action's
		// subject is always a battlefield Room. A present value must say so.
		if az := sp.str("AffectedZone"); az != "" && az != "Battlefield" {
			return refuse("ReduceCost AffectedZone %q", az)
		}
		if _, ok := sp.get("MinMana"); ok {
			return refuse("ReduceCost MinMana$ with ValidSpell$ Static.Unlock")
		}
	} else {
		// an absent AffectedZone$ is NOT simply the battlefield: gorge's
		// costStaticGateTail applies no zone gate on an absent key, while the
		// kernel prices battlefield activations only (cost_reduceability.go
		// abReduced's ActZone gate), so the lowering accepts the absent zone
		// only where every activation the ValidCard$ can match is a
		// battlefield one (reduceAbBattlefieldOnly). The equip arm's source
		// is always a battlefield permanent, so it needs no proof.
		if az := sp.str("AffectedZone"); az != "Battlefield" && (arm != 5 || az != "") {
			if az != "" {
				return refuse("ReduceCost AffectedZone %q", az)
			}
			if err := reduceAbBattlefieldOnly(sp.str("ValidCard")); err != nil {
				return err
			}
		}
		if v, ok := sp.get("MinMana"); ok {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 15 {
				return refuse("ReduceCost MinMana$ %q", v)
			}
			st.MinMana = uint8(n)
		}
	}
	st.AbilityRed = uint8(arm)
	for _, k := range []string{"Color", "EffectZone"} {
		if _, ok := sp.get(k); ok {
			return refuse("ReduceCost %s$", k)
		}
	}
	if a, ok := sp.get("Activator"); ok && a != "You" {
		return refuse("ReduceCost Activator %q", a)
	} else if !ok && arm == 1 {
		// no Activator$: any player's activation of a matching source is
		// reduced (Heartstone's opponent-controlled creatures; gorge
		// costActorMatches applies an actor-less static to every actor)
		st.Flags = append(st.Flags, "AnyCaster")
	}
	var err error
	if st.F, err = readF("ValidCard"); err != nil {
		return err
	}
	if vt, ok := sp.get("ValidTarget"); ok {
		// the reduction applies once the ability's chosen target matches
		// (Bladegraft Aspirant's "activated abilities of Equipment you
		// control that target CARDNAME"; gorge costStaticGateTail's
		// ValidTarget$ arm). kop.StTargetsFilter2, read with the static's
		// source as Self, the one read the equip arm already locksteps
		// (abReducedFor's tgts arm). A Spell head refuses: an equip targets
		// a creature, never a spell, and the same holds for an ability's
		// targets.
		if strings.Contains(vt, "Spell") {
			return refuse("ReduceCost ValidTarget$ %q", vt)
		}
		if st.F2, err = lc.filter(vt); err != nil {
			return err
		}
	}
	if arm == 5 && st.F == nil {
		// the equip arm, with or without a ValidTarget$ (Cloud, Planet's
		// Champion; Helitrooper): no ValidCard$ is any permanent's equip
		if st.F, err = lc.filter("Card"); err != nil {
			return err
		}
	}
	if st.Amount, err = lc.value(sp.str("Amount")); err != nil {
		return err
	}
	return nil
}

// reduceAbBattlefieldOnly refuses an absent AffectedZone$ whose ValidCard$
// can match a non-battlefield activation's source: the kernel prices
// battlefield activations only (cost_reduceability.go abReduced's ActZone
// gate), while gorge applies no zone gate on an absent key and prices the
// graveyard activation the filter matches (Embalmer's Tools' unearth:
// gorge charges {1} less, the kernel full price). An alternative is
// provably battlefield-only when it carries inZoneBattlefield, when its
// candidates are an attachment's host (an Aura's or Equipment's host is
// always a battlefield permanent: Power Artifact's Artifact.EnchantedBy) or
// when its base type is Equipment or Aura -- gorge offers a card's ability
// from outside the battlefield only through the graveyard grants and casts
// (unearth, embalm, eternalize, scavenge, escape, flashback, retrace), whose
// sources are creature and spell cards.
func reduceAbBattlefieldOnly(spec string) error {
	for _, alt := range strings.Split(spec, ",") {
		alt = strings.TrimSpace(alt)
		if strings.Contains(alt, "inZoneBattlefield") {
			continue
		}
		if strings.Contains(alt, ".EnchantedBy") || strings.Contains(alt, ".EquippedBy") {
			continue
		}
		base, _, _ := strings.Cut(alt, ".")
		if base == "Equipment" || base == "Aura" {
			continue
		}
		return refuse("ReduceCost AffectedZone \"\": %q can match a non-battlefield activation", alt)
	}
	return nil
}

// reduceAbBits is a StatReduceAbility static's B (the arm bit and the MinMana$
// floor) and StFlags (kop.StReduceAny when no Activator$ names the activator)
// from its lowered form.
func reduceAbBits(s lStatic) (b int16, fl kop.StaticFlags) {
	switch s.AbilityRed {
	case 2:
		b = 1
	case 3:
		b = kop.ReduceAbUnlock
	case 4:
		b = kop.ReduceAbPowerUp
	case 5:
		b = kop.ReduceAbEquip
	}
	b |= int16(s.MinMana) << kop.ReduceAbMinShift
	for _, f := range s.Flags {
		if f == "AnyCaster" {
			fl |= kop.StReduceAny // no Activator$: every activator (cost_reduceability.go)
		}
	}
	return b, fl
}
