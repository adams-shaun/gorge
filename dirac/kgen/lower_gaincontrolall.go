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
// never takes. It returns nil when the node has no AllValid$.
func (lc *lowerCtx) gainControlAll(ps *params) (*lFilter, error) {
	spec, ok := ps.get("AllValid")
	if !ok {
		return nil, nil
	}
	for _, k := range []string{"Choices", "Defined", "ValidTgts"} {
		if ps.peek(k) != "" {
			return nil, refuse("GainControl AllValid$ beside %s$", k)
		}
	}
	spec = strings.TrimSpace(spec)
	for _, w := range []string{"Remember", "Imprint", "Chosen", "Triggered", "Targeted", "Defined"} {
		if strings.Contains(spec, w) {
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
