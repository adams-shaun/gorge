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

// refuseEffectReturnKw refuses an Undying or Persist grant made by a
// resolving effect (Pump / PumpAll KW$: Cauldron of Souls, Endling, Haunted
// One). gorge lists a dying creature's granted triggers in its active()
// order, layer first and then timestamp, so a layer-6 Pump grant comes ahead
// of a static that also sets P/T (Mikaeus, the Unhallowed's +1/+1 and
// undying) whatever their timestamps, while the kernel lists static grants
// by timestamp and effect grants after them: a creature granted both dies
// with its two triggers offered in opposite orders (lockstep divergence on
// the trigger-order ask). Until the kernel mirrors that order, only static
// AddKeyword$ grants lower.
func refuseEffectReturnKw(lines []string) error {
	for _, l := range lines {
		if l == "Undying" || l == "Persist" {
			return refuse("effect-granted %s (trigger order beside a static grant)", l)
		}
	}
	return nil
}

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
