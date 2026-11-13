package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Extort (CR 702.101a): "Whenever you cast a spell, you may pay {W/B}. If you
// do, each opponent loses 1 life and you gain that much life." gorge expands
// the printed keyword into a SpellCast trigger whose body is DB$ Extort
// (engine/cards/kw_extort.go), and synthesizes the same trigger for a
// layer-6 AddKeyword$ Extort grant (rules/scriptfacts). The printed line is
// lowered from the expansion itself; this file supplies the granted
// instance's latent trigger and the keyword line's no-op arm.

// splitExtortGrant takes every bare "Extort" term out of a `&` keyword list:
// the rest of the list and how many there were. Each term is its own trigger
// instance (CR 702.101b, 113.2c): the kernel lists one latent trigger per
// granting static (grantedAbilitiesN), beside a printed Extort's own line.
func splitExtortGrant(kw string) (string, int) {
	var rest []string
	n := 0
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if k == "Extort" {
			n++
			continue
		}
		rest = append(rest, k)
	}
	return strings.Join(rest, " & "), n
}

// extortGrantTrigger lowers the trigger a granted Extort gives its holder as
// a latent SpellCast ability and returns its index, one per card. It has the
// printed expansion's shape (engine/cards/kw_extort.go): Mode$ SpellCast |
// ValidActivatingPlayer$ You | TriggerZones$ Battlefield, DB$ Extort. The
// kernel lists it once per granting static among the holder's granted
// triggers, so a permanent holding the keyword through two grants, or one
// grant beside a printed line, triggers once per instance.
func (lc *lowerCtx) extortGrantTrigger() (int, error) {
	const name = "GrantedExtort"
	for i, a := range lc.out.Abilities {
		if a.Latent && a.Name == name {
			return i, nil
		}
	}
	t := cards.Trigger{
		Mode: "SpellCast",
		Params: map[string]string{
			"Mode": "SpellCast", "ValidActivatingPlayer": "You",
			"TriggerZones": "Battlefield", "TriggerDescription": "Extort",
		},
		Effect: &cards.SA{Kind: "DB", API: "Extort"},
	}
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) != idx+1 {
		return 0, refuse("granted Extort lowering")
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = name
	return idx, nil
}

// faceHasExtortTrigger reports whether f carries the printed K:Extort
// expansion's SpellCast trigger (Keyword$ Extort, body DB$ Extort).
func faceHasExtortTrigger(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for i := range f.Triggers {
		t := &f.Triggers[i]
		if t.Mode != "SpellCast" || t.ParamStr(cards.PKKeyword) != "Extort" {
			continue
		}
		// An unresolved Execute$ is still the expansion (the compiler
		// resolves every trigger's Effect before kgen sees the card).
		if t.Effect == nil || t.Effect.API == "Extort" {
			return true
		}
	}
	return false
}
