package kgen

import "strings"

// ctrPlayerAddedAll lowers Mode$ CounterPlayerAddedAll whose recipients
// are objects only (gorge counterPlayerAddedAllMatches, trigmatch/
// counters.go:206): the kernel's per-recipient CounterAddedOnce event.
func (lc *lowerCtx) ctrPlayerAddedAll(tp *params, tr *lTrigger) error {
	for _, k := range []string{"ValidObjectToSource", "ValidCard", "ValidPlayer"} {
		if _, ok := tp.get(k); ok {
			return refuse("CounterPlayerAddedAll %s$", k)
		}
	}
	vo, ok := tp.get("ValidObject")
	if !ok {
		return refuse("CounterPlayerAddedAll without ValidObject")
	}
	alts := strings.Split(vo, ",")
	for i, a := range alts {
		a = strings.TrimSpace(a)
		for _, p := range []string{"Player", "You", "Opponent", "Defined"} {
			if strings.HasPrefix(a, p) {
				return refuse("CounterPlayerAddedAll player recipient %q", a)
			}
		}
		// the recipient is always a battlefield permanent kernel-side
		if s, ok := strings.CutSuffix(a, "+inRealZoneBattlefield"); ok {
			a = s
		} else if s, ok := strings.CutSuffix(a, "+inZoneBattlefield"); ok {
			a = s
		}
		alts[i] = a
	}
	var err error
	if tr.F, err = lc.filter(strings.Join(alts, ",")); err != nil {
		return err
	}
	if vs := tp.str("ValidSource"); vs != "You" {
		return refuse("CounterPlayerAddedAll ValidSource %q", vs)
	}
	tr.Whose = whoYou
	if ct := tp.str("CounterType"); ct == "" {
		tr.Step = "*"
	} else {
		if _, err := counterKind(ct); err != nil {
			return refuse("CounterPlayerAddedAll CounterType %q", ct)
		}
		tr.Step = ct
	}
	tr.Event = "CounterAddedOnce"
	tr.CounterCell = true
	return nil
}
