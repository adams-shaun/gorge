package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/rules/scriptfacts"
)

// Granted attack-trigger keywords: Annihilator N (CR 702.86a) and Battle cry
// (CR 702.91a), and the granted dies-return keywords Undying (CR 702.93a) and
// Persist (CR 702.79a; Mikaeus, the Unhallowed's AddKeyword$ Undying,
// Cauldron of Souls' Pump KW$ Persist), whose look-back dies trigger rides
// the same latent-grant path a granted Afterlife does. A layer-6 grant of either (Idol of False Gods' AddKeyword$
// Annihilator:2, Reckless Pyrosurfer's Pump KW$ Battle cry) gives the holder
// the printed keyword's Attacks Self trigger. gorge's granted walk
// (rules/trigger_granted.go checkGrantedStaticTriggersUsing) synthesizes it
// from the keyword line (scriptfacts.GrantedKeywordTrigger) and queues one per
// registered grant, beside any printed K: line's own. Each lowers here to a
// latent ability from that one synthesizer, listed once per grant.

// An Undying or Persist grant made by a resolving effect (Pump / PumpAll KW$:
// Cauldron of Souls, Endling, Haunted One) rides an end-of-turn override
// record. gorge lists a dying creature's granted keyword triggers in its
// active() order, layer then timestamp, and a Pump's KW$ grant and a static's
// AddKeyword$ grant (Mikaeus, the Unhallowed's undying) are both layer 6, so
// they come out by timestamp. The latent ability is kop.Ability.GrantedKw
// (build_ab.go), so the kernel's grant walk (grantedAbilitiesOwn) lists the
// record by its timestamp among the static grants, not after them.

// splitAttackKwGrant takes every "Annihilator:<N>" (N a literal 1..20) and
// bare "Battle cry", "Undying" or "Persist" term out of a `&` keyword list: the rest of the list and
// the taken lines in list order. Any other spelling of either head (a bare
// "Annihilator", Ulamog, the Defiler's "Annihilator:X") is refused: gorge's
// synthesizer returns no trigger for it either.
func splitAttackKwGrant(kw string) (string, []string, error) {
	var rest, lines []string
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		switch {
		case k == "Battle cry" || k == "Undying" || k == "Persist":
			lines = append(lines, k)
		case k == "Annihilator" || strings.HasPrefix(k, "Annihilator:"):
			s := strings.TrimPrefix(k, "Annihilator:")
			n, err := strconv.Atoi(s)
			if err != nil || n < 1 || n > 20 || strconv.Itoa(n) != s {
				return "", nil, refuse("granted Annihilator %q", k)
			}
			lines = append(lines, k)
		default:
			rest = append(rest, k)
		}
	}
	return strings.Join(rest, " & "), lines, nil
}

// attackKwGrantTrigger lowers the trigger a granted attack keyword line gives
// its holder as a latent ability and returns its index, one per distinct
// line on the card: gorge's synthesized trigger, the printed expansion's
// shape (cards.kwAnnihilator: Sacrifice by TriggeredDefendingPlayer with the
// Annihilator$ count; cards.kwBattleCry: PumpAll Creature.attacking+
// StrictlyOther +1/+0).
func (lc *lowerCtx) attackKwGrantTrigger(line string) (int, error) {
	name := "Granted" + strings.ReplaceAll(line, " ", "")
	for i, a := range lc.out.Abilities {
		if a.Latent && a.Name == name {
			return i, nil
		}
	}
	tp := scriptfacts.GrantedKeywordTrigger(line)
	if tp == nil {
		return 0, refuse("granted %q lowering", line)
	}
	t := *tp
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) != idx+1 {
		return 0, refuse("granted %q lowering", line)
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = name
	return idx, nil
}

// grantAttackKwAll emits, for each granted line, an end-of-turn OverrideAll
// record carrying the latent trigger over every permanent matching f
// (PumpAll KW$ Annihilator:N: gorge's PumpAll registers the grant on each
// match as it resolves, CR 611.2c), the grantAfterlifeAll shape.
func (lc *lowerCtx) grantAttackKwAll(f *lFilter, lines []string) error {
	for _, line := range lines {
		idx, err := lc.attackKwGrantTrigger(line)
		if err != nil {
			return err
		}
		o := &lOverride{GrantAbility: []int{idx}}
		in := lInstr{Op: "OverrideAll", F: f, CS: "EOT", Note: overrideString(o)}
		lc.out.overrides = append(lc.out.overrides, o)
		in.C = len(lc.out.overrides) - 1
		lc.emit(in)
	}
	return nil
}

// gainControlAddKWs lowers a GainControl node's AddKWs$ beside LoseControl$:
// the additive flag it contributes ("kw=Haste" for the older bare-haste arm)
// and, for the end-of-turn grant the kernel models, the keywords and
// attack-trigger lines that follow the control record
// (emitGainControlGrants). The Flayer of Loyalties shape,
// `AddKWs$ Haste & Trample & Annihilator:2` beside LoseControl$ EOT: gorge
// keeps an AddKWs$ grant for as long as the control effect lives; on an
// end-of-turn record the pump's until-end-of-turn keyword grant
// (OpGrantKeyword) and the latent Attacks trigger riding an end-of-turn
// override record (the Reckless Pyrosurfer arm) expire at exactly the same
// moment, so each listed keyword and attack-trigger line rides those two
// proven arms. Any other lifetime keeps the refusal.
func (lc *lowerCtx) gainControlAddKWs(k, dur, lcv string) (extra, keywords, attackLines []string, err error) {
	rest, lines, err := splitAttackKwGrant(k)
	if err != nil {
		return nil, nil, nil, err
	}
	attackLines = lines
	var ks []string
	if strings.TrimSpace(rest) != "" {
		if ks, err = pumpKeywords(rest); err != nil {
			return nil, nil, nil, err
		}
	}
	if len(attackLines) > 0 {
		if dur != "EOT" {
			return nil, nil, nil, refuse("GainControl AddKWs %q with LoseControl %q", k, lcv)
		}
		for _, s := range ks {
			if !simpleKeywords[s] {
				return nil, nil, nil, refuse("GainControl AddKWs %q: keyword %q", k, s)
			}
		}
		return nil, ks, attackLines, nil
	}
	if len(ks) != 1 || ks[0] != "Haste" {
		// gorge grants AddKWs$ for as long as the control effect lasts;
		// the kernel has haste only: until end of turn (GcHasteEOT) or
		// for as long as the control record lives (GcHasteEOT on a longer duration)
		return nil, nil, nil, refuse("GainControl AddKWs %q with LoseControl %q", k, lcv)
	}
	return []string{"kw=" + strings.Join(ks, "&")}, nil, nil, nil
}

// emitGainControlGrants emits the AddKWs$ terms beside a GainControl record:
// each end-of-turn keyword grant (the pump's own arm, instr_pump.go) and each
// granted attack keyword's latent-ability end-of-turn override.
func (lc *lowerCtx) emitGainControlGrants(obj lRef, keywords, attackLines []string) error {
	if len(keywords) > 0 {
		// the AddKWs$ terms live until end of turn -- the same lifetime the
		// end-of-turn control record has
		lc.emit(lInstr{Op: "GrantKeyword", Obj: obj, CS: strings.Join(keywords, "&")})
	}
	for _, line := range attackLines {
		// the granted Annihilator N's latent Attacks Self trigger rides an
		// end-of-turn override record beside the control record
		idx, err := lc.attackKwGrantTrigger(line)
		if err != nil {
			return err
		}
		lc.emitOverride(obj, &lOverride{GrantAbility: []int{idx}}, "EOT")
	}
	return nil
}
