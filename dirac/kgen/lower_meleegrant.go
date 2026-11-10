package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// splitMeleeGrant takes every bare "Melee" term out of a `&` keyword list:
// the rest of the list and how many there were. Each term is its own
// instance (CR 702.121b): gorge checkGrantedAttackPumpTriggers (rules/melee.go)
// queues one trigger per derived Melee entry beyond the printed lines.
func splitMeleeGrant(kw string) (string, int) {
	var rest []string
	n := 0
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if k == "Melee" {
			n++
			continue
		}
		rest = append(rest, k)
	}
	return strings.Join(rest, " & "), n
}

// meleeGrantTrigger lowers the trigger a granted Melee gives its holder
// (CR 702.121a) as a latent Attacks Self ability and returns its index, one
// per card. gorge's granted body is the printed expansion's
// (cards.kwMelee): DB$ Pump Defined$ Self with NumAtt$ and NumDef$ both
// cards.MeleePumpCount, the declaration's distinct attacked opponents
// captured as the trigger fires (mlTrigger: value.go reads it). The
// kernel lists it where gorge's keyword walk does (CardDef.AtkFace,
// dirac/kw_melee.go).
func (lc *lowerCtx) meleeGrantTrigger() (int, error) {
	const name = "GrantedMelee"
	for i, a := range lc.out.Abilities {
		if a.Latent && a.Name == name {
			return i, nil
		}
	}
	t := cards.Trigger{
		Mode:   "Attacks",
		Params: map[string]string{"Mode": "Attacks", "ValidCard": "Card.Self"},
		Effect: &cards.SA{Kind: "DB", API: "Pump", Params: map[string]string{
			"Defined": "Self", "NumAtt": cards.MeleePumpCount, "NumDef": cards.MeleePumpCount,
		}},
	}
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) != idx+1 {
		return 0, refuse("granted Melee lowering")
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = name
	return idx, nil
}

// faceObservesAttack is CardDef.AtkFace: gorge's trigger prefilter
// (rules/trigger_eligibility.go objectFaceMayTriggerHoisted, over the
// compiled catalog's per-face interests, cards/compiled_catalog.go) admits a
// declare-attackers event to the face's printed lines. A face outside the
// catalog takes gorge's textual path, which this does not repeat: it counts
// as observing whenever it has a trigger line.
func faceObservesAttack(f *cards.Face) bool {
	if f == nil || len(f.Triggers) == 0 {
		return false
	}
	in, ok := f.CompiledTriggerInterests()
	if !ok {
		return true
	}
	return in&(cards.TriggerInterestAny|cards.TriggerInterestAttackDeclaration) != 0
}

// isMeleeTrigger reports whether t is the trigger gorge's kwMelee expansion
// gives a printed Melee, or the one meleeGrantTrigger builds for a granted
// one: Attacks Self, its body the cards.MeleePumpCount pump.
func isMeleeTrigger(t cards.Trigger, svars map[string]string) bool {
	if t.Mode != "Attacks" || t.Params["ValidCard"] != "Card.Self" {
		return false
	}
	body := t.Effect
	if body == nil {
		body = cards.ResolveSVar(svars, t.Params["Execute"])
	}
	return body != nil && body.API == "Pump" && body.Params["NumAtt"] == cards.MeleePumpCount &&
		body.Params["NumDef"] == cards.MeleePumpCount
}

// faceHasMeleeTrigger reports whether f carries the printed Melee
// expansion's trigger (Keyword$ Melee).
func faceHasMeleeTrigger(f *cards.Face) bool {
	for _, t := range f.Triggers {
		if t.Params["Keyword"] == "Melee" && isMeleeTrigger(t, f.SVars) {
			return true
		}
	}
	return false
}
