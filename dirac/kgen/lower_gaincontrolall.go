package kgen

import "strings"

// gainControlAll reads GainControl's AllValid$ subject (gorge
// effGainControl, effects/choose_control.go): with no Choices$, every
// battlefield permanent matching AllValid$ is listed in object order (the
// g.Objs walk) before any control moves, and each then gains the same grant
// (Varchild, Betrayer of Kjeldor's "gain control of all Survivors", Tevesh
// Szat's "gain control of all commanders"). It lowers to OpGainControl with
// Obj RefNone and the filter F (dirac/act_gaincontrolall.go). gorge reads
// Choices$ before AllValid$ and AllValid$ before Defined$, so a subject
// parameter beside it refuses by name rather than lowering a branch gorge
// never takes — except a lone ValidTgts$ Player beside a NewController$ You
// whose AllValid$ filter reads TargetedPlayerCtrl (Emrakul, the World Anew):
// there the player target is the subject the filter reads, so the target
// slot is kept. It returns nil when the node has no AllValid$.
func (lc *lowerCtx) gainControlAll(ps *params) (*lFilter, error) {
	spec, ok := ps.get("AllValid")
	if !ok {
		return nil, nil
	}
	for _, k := range []string{"Choices", "Defined"} {
		if ps.peek(k) != "" {
			return nil, refuse("GainControl AllValid$ beside %s$", k)
		}
	}
	// A player-only ValidTgts$ beside AllValid$ is Emrakul, the World Anew's
	// shape: the target slot the caller keeps is the read the AllValid$
	// filter's TargetedPlayerCtrl word needs (kop.F2TgtPlayerCtrl reads every
	// player target of the resolving entry; dirac/filter_heads.go), and
	// NewController$ You keeps the grant with the resolving controller, so
	// the player target is a subject, not a new controller. Any other
	// ValidTgts$ shape refuses by name.
	vt := ps.peek("ValidTgts")
	if vt != "" && strings.EqualFold(vt, "Player") && strings.EqualFold(ps.peek("NewController"), "You") {
		if !strings.Contains(spec, "TargetedPlayerCtrl") {
			return nil, refuse("GainControl AllValid$ %q beside ValidTgts$ %q: the filter does not read the targeted player", spec, vt)
		}
		tp := map[string]string{"ValidTgts": vt}
		for _, k := range []string{"TargetMin", "TargetMax"} {
			if v, ok := ps.get(k); ok {
				tp[k] = v
			}
		}
		r, err := lc.objOrTarget(newParams(tp), "", zBattlefield)
		if err != nil {
			return nil, err
		}
		if t := lc.ab.Targets[r.N].F; t == nil || t.Players == 0 || len(t.Clauses) != 0 {
			return nil, refuse("GainControl AllValid$ beside ValidTgts$ %q: not a lone player target", vt)
		}
		ps.consume("ValidTgts", "TgtPrompt", "ValidTgtsDesc")
	} else if vt != "" {
		return nil, refuse("GainControl AllValid$ beside ValidTgts$ %q", vt)
	}
	spec = strings.TrimSpace(spec)
	for _, w := range []string{"Remember", "Imprint", "Chosen", "Triggered", "Targeted", "Defined"} {
		// TargetedPlayerCtrl is the accepted targeted-player read, not a
		// referent the AllValid$ walk could mis-bind (the branch above).
		if strings.Contains(strings.ReplaceAll(spec, "TargetedPlayerCtrl", ""), w) {
			return nil, refuse("GainControl AllValid$ %q reads %s", spec, w)
		}
	}
	if strings.Contains(spec, "!IsCommander") {
		// the filter lowers a negated IsCommander to "always holds"
		// (cmdshortcut.go), which a Commander game -- where these sweeps
		// are dealt -- would see diverge (Subjugate the Hobbits)
		return nil, refuse("GainControl AllValid$ %q: negated IsCommander", spec)
	}
	f, err := lc.filter(spec)
	if err != nil {
		return nil, err
	}
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return nil, refuse("GainControl AllValid$ %q: not a card filter", spec)
	}
	return f, nil
}
