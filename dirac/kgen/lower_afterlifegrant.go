package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/rules/scriptfacts"
)

// splitAfterlifeGrant takes every "Afterlife:<N>" term (N a literal 1..20)
// out of a `&` keyword list: the rest of the list and the counts in list
// order. Each term is its own instance (CR 702.135a, 113.2c): gorge's granted
// walk (rules/trigger_granted.go checkGrantedStaticTriggersUsing) queues one
// trigger per registered AddKeywords line, beside any printed K:Afterlife's.
// Any other spelling of the head (a bare "Afterlife", a non-literal N) is
// refused: gorge's synthesizer returns no trigger for it either.
func splitAfterlifeGrant(kw string) (string, []int, error) {
	var rest []string
	var ns []int
	for _, k := range strings.Split(kw, "&") {
		k = strings.TrimSpace(k)
		if k != "Afterlife" && !strings.HasPrefix(k, "Afterlife:") {
			rest = append(rest, k)
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(k, "Afterlife:"))
		if err != nil || n < 1 || n > 20 || strconv.Itoa(n) != strings.TrimPrefix(k, "Afterlife:") {
			return "", nil, refuse("granted Afterlife %q", k)
		}
		ns = append(ns, n)
	}
	return strings.Join(rest, " & "), ns, nil
}

// afterlifeGrantTrigger lowers the trigger a granted Afterlife N gives its
// holder (CR 702.135a; Afterlife Insurance) as a latent dies ability and
// returns its index; one per distinct N on the card. It is gorge's granted
// body from the one synthesizer (scriptfacts.GrantedKeywordTrigger), the
// printed K:Afterlife expansion's shape: ChangesZone Battlefield to
// Graveyard of Self, DB$ Token wb_1_1_spirit_flying, TokenAmount$ N.
func (lc *lowerCtx) afterlifeGrantTrigger(n int) (int, error) {
	name := "GrantedAfterlife" + strconv.Itoa(n)
	for i, a := range lc.out.Abilities {
		if a.Latent && a.Name == name {
			return i, nil
		}
	}
	tp := scriptfacts.GrantedKeywordTrigger("Afterlife:" + strconv.Itoa(n))
	if tp == nil {
		return 0, refuse("granted Afterlife lowering")
	}
	t := *tp
	idx := len(lc.out.Abilities)
	if err := lc.withAbility(lc.ab, func() error { return lc.lowerTriggered(t, "") }); err != nil {
		return 0, err
	}
	if len(lc.out.Abilities) != idx+1 {
		return 0, refuse("granted Afterlife lowering")
	}
	lc.out.Abilities[idx].Latent = true
	lc.out.Abilities[idx].Name = name
	return idx, nil
}

// grantAfterlifeAll emits, for each granted instance, an end-of-turn
// OverrideAll record carrying the latent dies trigger over every permanent
// matching f (PumpAll KW$ Afterlife:N: gorge's PumpAll registers the grant on
// each match as it resolves, CR 611.2c).
func (lc *lowerCtx) grantAfterlifeAll(f *lFilter, ns []int) error {
	for _, n := range ns {
		idx, err := lc.afterlifeGrantTrigger(n)
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
