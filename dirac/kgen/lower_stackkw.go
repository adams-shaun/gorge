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
//     entries (kernel kw_splitsecond.go). The printed keyword is
//     CardDef.SplitSecond and reads the same gate (lower_splitsecond.go).
//   - Conspire lowers to Kind "StackKw" too, read at cast time like Storm: the
//     conspired cast (kernel kw_conspire.go) taps two creatures sharing a
//     colour with the spell and copies it. A printed K:Conspire is CardDef.Conspire
//     (lower_squadreplicate.go), its copy the keyword's own trigger.
//   - Delve lowers to Kind "StackKw" too: the cast announces graveyard cards
//     to exile toward its generic cost (CR 702.66a; kernel kw_delve.go), read
//     as the spell is cast. A cast-from-hand restriction is legal; a printed
//     K:Delve is CardDef.Delve and reads the same kernel path.
//   - Demonstrate lowers to Kind "StackKw" too: gorge synthesizes the copy
//     trigger when the cast spell is put on the stack
//     (trigger_granted.go checkGrantedDemonstrateTriggers), so the kernel
//     queues it as the cast completes, like Storm (kw_demonstrate.go).
//   - A Condition$ / IsPresent$ / CheckSVar$ gate rides the cast-time heads
//     whose grant gorge reads with the in-flight cast excluded: Cascade,
//     Storm, Conspire, Demonstrate, Delve and Casualty (hasKeywordPriorCasts /
//     the cast trigger walk), so "the first spell you cast each turn" holds
//     for the very cast the grant is for (Elda, Conjurer of Spectacle;
//     Abaddon the Despoiler's PlayerTurn; Anhelo's ThisTurnCast count). The
//     kernel reads the same lowered condition with the in-flight push hidden
//     (Dirac's hideInflightCast). Affinity's gate rides the ReduceCost static
//     (the same cost-composition read). A gate body the condition lowering
//     cannot represent refuses with that lowering's own named reason.
//   - A gate on Convoke, Improvise, Lifelink or Split second refuses by name:
//     gorge reads the first two at the CR 601.2b announcement, after the
//     up-front CR 601.2a push, so a Count$ThisTurnCast gate counts the
//     in-flight spell and the first-spell grant never reaches it; the last two
//     are read at damage or priority time, with no cast-time carrier.
//   - "Cascade & Cascade" is two instances of the one head, two grants: the
//     kernel's per-static cascade count queues two triggers (CR 702.85c).
//     Any other mixture refuses.
//   - YouCtrl alone names the caster: under gorge's stack override the
//     proposed spell's controller is its caster (cast_provenance.go), the
//     same read wasCastByYou makes.
//   - wasCastFromHand becomes the CastHand flag (the kernel's fromHand read),
//     wasCastFromExile becomes the CastExile flag (the kernel's cast origin),
//     and CastSa provenance is refused by name (no per-payment carrier).
//   - Every other keyword has no printed form in the kernel and refuses by
//     name.
func (lc *lowerCtx) stackKwGrant(sp *params, st *lStatic, aff string) ([]lStatic, error) {
	kw := sp.str("AddKeyword")
	tokens, err := stackKwTokens(kw)
	if err != nil {
		return nil, err
	}
	head, _, _ := strings.Cut(tokens[0], ":")
	selfStack := false
	if ez, ok := sp.get("EffectZone"); ok {
		switch strings.TrimSpace(ez) {
		case "Battlefield":
		case "Stack":
			// a self grant that functions from the stack (Bloodbraid
			// Marauder's "this spell has cascade as long as ..."): the
			// carrier is the spell itself, its controller its caster
			if strings.TrimSpace(aff) != "Card.Self" {
				return nil, refuse("AffectedZone Stack grant with EffectZone %q", ez)
			}
			selfStack = true
		default:
			return nil, refuse("AffectedZone Stack grant with EffectZone %q", ez)
		}
	}
	switch head {
	case "Affinity", "Cascade", "Improvise", "Convoke", "Casualty", "Lifelink", "Storm", "Split second", "Conspire", "Delve", "Demonstrate":
	default:
		return nil, refuse("AffectedZone Stack grant of %q", kw)
	}
	if st.Cond.Kind != cNone {
		// A gate on the four heads the kernel cannot read at the same point
		// gorge does refuses by name. gorge reads a Convoke/Improvise grant
		// through stackKeywordPossibleH at the CR 601.2b announcement, after
		// the up-front CR 601.2a push, so a Count$ThisTurnCast gate counts
		// the in-flight spell itself and the first-spell grant never reaches
		// it; a Lifetime/lifelink or split-second grant is read at damage or
		// priority time, where the kernel has no cast-time carrier to match
		// gorge's point in the cast.
		switch head {
		case "Improvise", "Convoke", "Lifelink", "Split second":
			return nil, refuse("AffectedZone Stack %s grant with a condition", head)
		}
	}
	expr, prov, err := stackAffected(aff, head, selfStack)
	if err != nil {
		return nil, err
	}
	if selfStack && prov != stackProvNone {
		return nil, refuse("AffectedZone Stack self grant with a cast-origin word")
	}
	if st.F, err = lc.filter(expr); err != nil {
		return nil, err
	}
	out := make([]lStatic, 0, len(tokens))
	for _, tok := range tokens {
		one := *st
		one.Kw = []string{tok}
		h, p, _ := strings.Cut(tok, ":")
		switch h {
		case "Affinity":
			spec, _, _ := strings.Cut(p, ":")
			if strings.TrimSpace(spec) == "" {
				return nil, refuse("AffectedZone Stack affinity grant without a spec")
			}
			sep := "." // kw_affinity.go: a dotted spec joins with "+", a dot-less one with "."
			if strings.Contains(spec, ".") {
				sep = "+"
			}
			one.Kind = "ReduceCost"
			if one.Amount, err = lc.value("Count$Valid " + spec + sep + "YouCtrl"); err != nil {
				return nil, err
			}
		case "Cascade", "Improvise", "Convoke", "Lifelink", "Storm", "Split second", "Conspire", "Delve", "Demonstrate":
			if p != "" {
				return nil, refuse("AffectedZone Stack grant of %q", tok)
			}
			if (h == "Lifelink" || h == "Split second") && prov == stackProvHand {
				return nil, refuse("AffectedZone Stack %s grant restricted to spells cast from hand", strings.ToLower(h))
			}
			one.Kind = "StackKw"
		case "Casualty":
			if n, err := strconv.Atoi(p); err != nil || n < 1 || n > 127 || strconv.Itoa(n) != p {
				return nil, refuse("AffectedZone Stack grant of %q", tok)
			}
			one.Kind = "StackKw"
		}
		// the full token: build_ab.go reads the casualty threshold and the
		// affinity spec off it
		one.Flags = prov.flags()
		if selfStack {
			one.Flags = append(one.Flags, "SelfStack")
		}
		out = append(out, one)
	}
	return out, nil
}

// stackKwTokens splits an AddKeyword$ value into its "A & A" instances. The
// one supported mixture is two or more copies of the same token (Zhulodok's
// "Cascade & Cascade"); a mixture of different keywords has no kernel form
// and refuses under the grant's own name.
func stackKwTokens(kw string) ([]string, error) {
	if kw == "" {
		return nil, refuse("AffectedZone Stack grant of %q", kw)
	}
	if !strings.Contains(kw, " & ") {
		return []string{kw}, nil
	}
	tokens := strings.Split(kw, " & ")
	for _, tok := range tokens {
		if tok != tokens[0] {
			return nil, refuse("AffectedZone Stack grant of %q", kw)
		}
	}
	return tokens, nil
}

// stackProv is a stack grant's Affected$ cast-origin restriction.
type stackProv uint8

const (
	stackProvNone    stackProv = iota
	stackProvHand              // wasCastFromHand / wasCastFromYourHand[ByYou]
	stackProvNotHand           // !wasCastFromYourHand (anywhere but a hand)
	stackProvExile             // wasCastFromExile
)

// flags is the kgen static flag the restriction lowers to (build_ab.go).
func (p stackProv) flags() []string {
	switch p {
	case stackProvHand:
		return []string{"CastHand"}
	case stackProvNotHand:
		return []string{"CastNotHand"}
	case stackProvExile:
		return []string{"CastExile"}
	}
	return nil
}

// stackAffected rewrites a stack-grant Affected$ for the cast-time read.
// gorge judges the proposed spell, still in hand with no PutOnStack, under
// the stack override (cast_provenance.go): wasCastByYou holds iff the caster
// is the static's controller, bare wasCast always holds, and
// wasCastFromYourHand[ByYou] / wasCastFromHand hold iff the caster is the
// controller and the card is in its hand; wasCastFromExile holds iff the
// proposed cast comes from exile. The kernel tests "the caster is the static's
// controller" structurally (ReduceCost's stc.ctrl == p; stackKwN's caster
// argument), so those words and YouCtrl (the controller of the proposed
// spell IS its caster) drop out of the filter, and the cast-origin word
// becomes a flag. An alternative that never names its caster (a bare wasCast
// without YouCtrl would also match an opponent's spell) or carries any other
// provenance word refuses. atDamage (a lifelink grant, read when the
// resolving spell deals damage) inverts the provenance rule: the alternative
// must name YouCtrl and carry no cast word.
func stackAffected(aff, head string, selfStack bool) (string, stackProv, error) {
	atDamage := head == "Lifelink" || head == "Split second"
	var alts []string
	prov, plain := stackProvNone, false
	for _, alt := range strings.Split(aff, ",") {
		alt = strings.TrimSpace(alt)
		base, preds, _ := strings.Cut(alt, ".")
		caster, cast := false, false
		altProv := stackProvNone
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
				case "wasCastFromYourHand", "wasCastFromYourHandByYou", "wasCastFromHand":
					caster, cast, altProv = true, true, stackProvHand
				case "!wasCastFromYourHand":
					cast, altProv = true, stackProvNotHand
				case "wasCastFromExile":
					cast, altProv = true, stackProvExile
				default:
					if strings.Contains(p, "wasCast") || strings.Contains(p, "CastSa") {
						return "", stackProvNone, refuse("AffectedZone Stack grant: Affected$ word %q", p)
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
				return "", stackProvNone, refuse("AffectedZone Stack lifelink grant: Affected$ %q is not a plain YouCtrl spell filter", alt)
			}
		} else if !caster && !selfStack {
			// every cast-time head names its caster: YouCtrl alone is the
			// caster under the stack override (Storm, Delve, Cascade,
			// Improvise, Convoke, Casualty, Affinity); a stack self grant's
			// Card.Self is the spell itself, its controller the caster
			return "", stackProvNone, refuse("AffectedZone Stack grant: Affected$ %q does not name the caster of a cast spell", alt)
		}
		switch {
		case altProv == stackProvNone:
			plain = true
		case prov == stackProvNone:
			prov = altProv
		case prov != altProv:
			return "", stackProvNone, refuse("AffectedZone Stack grant: Affected$ mixes cast-origin alternatives")
		}
		if len(keep) > 0 {
			base += "." + strings.Join(keep, "+")
		}
		alts = append(alts, base)
	}
	if plain && prov != stackProvNone {
		return "", stackProvNone, refuse("AffectedZone Stack grant: Affected$ mixes a cast-origin alternative and a plain one")
	}
	return strings.Join(alts, ","), prov, nil
}
