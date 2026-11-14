package kgen

import (
	"strconv"
	"strings"
)

// stackKwGrant lowers a Mode$ Continuous static whose AffectedZone$ is Stack
// and which grants a keyword to the spells its controller casts (Sami,
// Wildcat Captain's "Spells you cast have affinity for artifacts"; Quandrix's
// cascade). gorge derives the proposed spell under a stack-zone override
// (rules keywordmay.go stackKeywordPossibleH) and reads the granted keyword
// at each cast-time reader (cascadeInstances), so the grant is read live,
// per spell, off the static's controller (kernel stackKwN).
//
//   - Affinity is not a keyword read: gorge mints the ReduceCost static the
//     printed expander would (statics_costscan.go affinityGrantCostStatics:
//     generic only, Count$Valid <spec><sep>YouCtrl), so it lowers to an
//     ordinary ReduceCost over the rewritten filter, live from the battlefield
//     only (CR 113.6).
//   - Cascade lowers to Kind "StackKw" (kernel StatStackKw).
//   - Improvise and Casualty:<n> lower to Kind "StackKw" too (kernel
//     tapPays / isImprovise / casualtyN). gorge offers a tap-paid or casualty
//     cast only against a floating pool; the stackkw grant lockstep builds
//     one (Iridescent Tiger's ETB mana). Casualty:X, Casualty:0 and a rider
//     refuse.
//   - Convoke lowers to Kind "StackKw" too (Chief Engineer, We Ride at Dawn,
//     Invasion of Segovia): it reads wherever the printed Convoke does (kernel
//     hasConvoke), the creatures the cast taps pay a pip of their colour or
//     {1} (CR 702.51a). A granted tap-paid keyword beside an Improvise source,
//     or on an {X} cost, poses gorge's mixed or pre-X announcement (kernel
//     convokeOptions); only a Harmonize or alternative-cost route stalls
//     unmodelled (StallStackKwGrant, kw_convoke.go).
//   - Lifelink lowers to Kind "StackKw" too: gorge reads the keyword off the
//     spell object when its damage is dealt (effects/damage.go HasKeyword),
//     so the kernel reads the grant at damage time (srcKw, stackLifelinkKw).
//     A cast-from-hand restriction refuses (no cast-time carrier).
//   - Storm lowers to Kind "StackKw" too: the cast trigger is kernel-native
//     (kw_storm.go), queued when the grant reaches the spell as it is cast.
//     A spell has the keyword or not, so two granting statics give one
//     trigger; the Affected$ filter may name the caster (YouCtrl, the Instant
//     and Sorcery emblem) or the cast (wasCastByYou).
//   - Split second lowers to Kind "StackKw" too: gorge reads the derived
//     keyword off each spell on the stack at every priority offer
//     (rules split_second.go), so the kernel reads the grant off the stack
//     entries (kernel kw_splitsecond.go). The printed keyword has no form in
//     the kernel.
//   - Conspire lowers to Kind "StackKw" too, read at cast time like Storm: the
//     conspired cast (kernel kw_conspire.go) taps two creatures sharing a
//     colour with the spell and copies it. The printed K:Conspire stays refused.
//   - Delve lowers to Kind "StackKw" too: the cast announces graveyard cards
//     to exile toward its generic cost (CR 702.66a; kernel kw_delve.go), read
//     as the spell is cast. A cast-from-hand restriction is legal; a printed
//     K:Delve is CardDef.Delve and reads the same kernel path.
//   - Demonstrate lowers to Kind "StackKw" too: gorge synthesizes the copy
//     trigger when the cast spell is put on the stack
//     (trigger_granted.go checkGrantedDemonstrateTriggers), so the kernel
//     queues it as the cast completes, like Storm (kw_demonstrate.go). It is
//     the one grant a Condition$ / CheckSVar$ gate may ride (Elda, Conjurer of
//     Spectacle's "the first nonlegendary creature spell you cast each turn"):
//     gorge reads the gate against the casts PRIOR to the in-flight one
//     (hasKeywordPriorCasts), and the kernel evaluates the static's condition
//     with that push hidden (queueDemonstrate).
//   - Every other keyword has no printed form in the
//     kernel and refuses by name.
func (lc *lowerCtx) stackKwGrant(sp *params, st *lStatic, aff string) error {
	kw := sp.str("AddKeyword")
	if kw == "" || strings.Contains(kw, " & ") {
		return refuse("AffectedZone Stack grant of %q", kw)
	}
	head, param, _ := strings.Cut(kw, ":")
	if st.Cond.Kind != cNone && head != "Demonstrate" {
		return refuse("AffectedZone Stack grant with a Condition$ / IsPresent$ / CheckSVar$ gate")
	}
	if ez, ok := sp.get("EffectZone"); ok && strings.TrimSpace(ez) != "Battlefield" {
		return refuse("AffectedZone Stack grant with EffectZone %q", ez)
	}
	switch head {
	case "Affinity", "Cascade", "Improvise", "Convoke", "Casualty", "Lifelink", "Storm", "Split second", "Conspire", "Delve", "Demonstrate":
	default:
		return refuse("AffectedZone Stack grant of %q", kw)
	}
	expr, castHand, err := stackAffected(aff, head)
	if err != nil {
		return err
	}
	if st.F, err = lc.filter(expr); err != nil {
		return err
	}
	switch head {
	case "Affinity":
		if castHand {
			return refuse("AffectedZone Stack affinity grant restricted to spells cast from hand")
		}
		spec, _, _ := strings.Cut(param, ":")
		if strings.TrimSpace(spec) == "" {
			return refuse("AffectedZone Stack affinity grant without a spec")
		}
		sep := "." // kw_affinity.go: a dotted spec joins with "+", a dot-less one with "."
		if strings.Contains(spec, ".") {
			sep = "+"
		}
		st.Kind = "ReduceCost"
		if st.Amount, err = lc.value("Count$Valid " + spec + sep + "YouCtrl"); err != nil {
			return err
		}
		return nil
	case "Cascade", "Improvise", "Convoke", "Lifelink", "Storm", "Split second", "Conspire", "Delve", "Demonstrate":
		if param != "" {
			return refuse("AffectedZone Stack grant of %q", kw)
		}
		if (head == "Lifelink" || head == "Split second") && castHand {
			return refuse("AffectedZone Stack %s grant restricted to spells cast from hand", strings.ToLower(head))
		}
	case "Casualty":
		if n, err := strconv.Atoi(param); err != nil || n < 1 || n > 127 || strconv.Itoa(n) != param {
			return refuse("AffectedZone Stack grant of %q", kw)
		}
	}
	st.Kind = "StackKw"
	st.Kw = []string{kw} // the full token: build_ab.go reads the casualty threshold off it
	if castHand {
		st.Flags = append(st.Flags, "CastHand")
	}
	return nil
}

// stackAffected rewrites a stack-grant Affected$ for the cast-time read.
// gorge judges the proposed spell, still in hand with no PutOnStack, under
// the stack override (cast_provenance.go): wasCastByYou holds iff the caster
// is the static's controller, bare wasCast always holds, and
// wasCastFromYourHand[ByYou] holds iff the caster is the controller and the
// card is in its hand. The kernel tests "the caster is the static's
// controller" structurally (ReduceCost's stc.ctrl == p; stackKwN's caster
// argument), so those words and YouCtrl (the controller of the proposed
// spell IS its caster) drop out of the filter, and the cast-from-hand word
// becomes a flag. An alternative that never names its caster (a bare wasCast
// without YouCtrl would also match an opponent's spell) or carries any other
// provenance word refuses. atDamage (a lifelink grant, read when the
// resolving spell deals damage) inverts the provenance rule: the alternative
// must name YouCtrl and carry no cast word.
func stackAffected(aff, head string) (string, bool, error) {
	atDamage, atCast := head == "Lifelink" || head == "Split second", head == "Storm" || head == "Conspire" || head == "Delve" || head == "Demonstrate"
	var alts []string
	hand, plain := false, false
	for _, alt := range strings.Split(aff, ",") {
		alt = strings.TrimSpace(alt)
		base, preds, _ := strings.Cut(alt, ".")
		caster, cast, fromHand := false, false, false
		var keep []string
		if preds != "" {
			for _, p := range strings.Split(preds, "+") {
				switch p {
				case "wasCastByYou":
					caster, cast = true, true
				case "wasCast":
					cast = true
				case "YouCtrl":
					caster = true
				case "wasCastFromYourHand", "wasCastFromYourHandByYou":
					caster, cast, fromHand = true, true, true
				default:
					if strings.Contains(p, "wasCast") || strings.Contains(p, "CastSa") {
						return "", false, refuse("AffectedZone Stack grant: Affected$ word %q", p)
					}
					keep = append(keep, p)
				}
			}
		}
		if atDamage {
			// a lifelink grant is read off the resolving spell at damage
			// time (srcKw): its controller must be named, a cast-time
			// provenance word has no meaning there
			if cast || !caster {
				return "", false, refuse("AffectedZone Stack lifelink grant: Affected$ %q is not a plain YouCtrl spell filter", alt)
			}
		} else if atCast {
			// a storm grant is read as the spell is cast, off the spell on
			// the stack: its controller is its caster, so YouCtrl names
			// the caster as wasCastByYou does
			if !caster {
				return "", false, refuse("AffectedZone Stack grant: Affected$ %q does not name the caster of a cast spell", alt)
			}
		} else if !cast || !caster {
			return "", false, refuse("AffectedZone Stack grant: Affected$ %q does not name the caster of a cast spell", alt)
		}
		if fromHand {
			hand = true
		} else {
			plain = true
		}
		if len(keep) > 0 {
			base += "." + strings.Join(keep, "+")
		}
		alts = append(alts, base)
	}
	if hand && plain {
		return "", false, refuse("AffectedZone Stack grant: Affected$ mixes cast-from-hand and plain alternatives")
	}
	return strings.Join(alts, ","), hand, nil
}
