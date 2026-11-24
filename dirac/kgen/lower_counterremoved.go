package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// ctrRemovedTrigger lowers Mode$ CounterRemoved / CounterRemovedOnce on the
// card itself (gorge trigmatch counterRemovedMatches; kernel
// dirac/trig_counterremoved.go): ValidCard$ Card.Self, a named CounterType$,
// TriggerZones$ Battlefield or Exile, an optional ValidPlayer$ You (the
// controller of the removed-from card: always the trigger's controller for a
// Card.Self line). Mode$ CounterRemoved is one trigger per counter removed
// (kop.Tf4PerCounter), or with NewCounterAmount$ 0 one trigger when the kind
// is emptied; CounterRemovedOnce one per removal batch. A time counter is the
// suspend upkeep removal and so only an exile line; +1/+1 and -1/-1 lines
// (removed by damage and annihilation, which the kernel does not fire) refuse.
// ctrRemovedOrPlayerAddedAll routes lowerTriggered's shared
// "CounterAddedAll", "CounterPlayerAddedAll", "CounterRemoved",
// "CounterRemovedOnce" arm (the function sits at its line ceiling):
// CounterAddedAll's batch watcher goes to ctrAddedAllTrigger, PlayerAddedAll's
// player-counter batch to ctrPlayerAddedAll, the counter-removed modes to
// ctrRemovedTrigger.
func (lc *lowerCtx) ctrRemovedOrPlayerAddedAll(tp *params, tr *lTrigger, t cards.Trigger, readF func(string) (*lFilter, error)) error {
	if t.Mode == "CounterAddedAll" {
		return lc.ctrAddedAllTrigger(tp, tr, readF)
	}
	if t.Mode == "CounterPlayerAddedAll" {
		return lc.ctrPlayerAddedAll(tp, tr)
	}
	return lc.ctrRemovedTrigger(tp, tr, t, readF)
}

func (lc *lowerCtx) ctrRemovedTrigger(tp *params, tr *lTrigger, t cards.Trigger, readF func(string) (*lFilter, error)) error {
	f, err := readF("ValidCard")
	if err != nil {
		return err
	}
	tr.F = f
	if vc := tp.str("ValidCard"); vc != "Card.Self" {
		return refuse("%s ValidCard$ %q", t.Mode, vc)
	}
	ct := tp.str("CounterType")
	if ct == "" {
		return refuse("%s without CounterType", t.Mode)
	}
	switch ct {
	case "P1P1", "M1M1":
		return refuse("%s CounterType$ %s: its removal by damage and annihilation is not fired", t.Mode, ct)
	}
	if _, err := counterKind(ct); err != nil {
		return refuse("%s CounterType$ %q", t.Mode, ct)
	}
	switch {
	case tr.Zone == zExile && ct == "TIME":
	case tr.Zone == zExile:
		return refuse("%s from exile CounterType$ %s: only the suspend time counter is removed there", t.Mode, ct)
	case tr.Zone != 0 && tr.Zone != zBattlefield:
		return refuse("%s TriggerZones %d", t.Mode, tr.Zone)
	case ct == "TIME" && t.Params["Keyword"] == "Vanishing":
		// K:Vanishing's last-counter sacrifice (lower_vanishing.go): the only
		// battlefield time-counter removal line that lowers
	case ct == "TIME":
		return refuse("%s CounterType$ TIME on the battlefield", t.Mode)
	}
	if vp, ok := tp.get("ValidPlayer"); ok && vp != "You" {
		return refuse("%s ValidPlayer$ %q", t.Mode, vp)
	}
	last := false
	if v, ok := tp.get("NewCounterAmount"); ok {
		if v != "0" || t.Mode != "CounterRemoved" {
			return refuse("%s NewCounterAmount$ %q", t.Mode, v)
		}
		last = true
	}
	tr.Step = ct
	tr.PerCounter = t.Mode == "CounterRemoved" && !last
	return nil
}
