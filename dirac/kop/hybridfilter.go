package kop

// HybridFilterManaAb reports whether mana ability a is "{c1/c2}, {T}: Add
// {c1}{c1}, {c1}{c2} or {c2}{c2}" (Fetid Heath and the Shadowmoor and Eventide
// filter lands): a fee of exactly one hybrid pip of two WUBRG colours beside
// {T}, nothing else, and a Combo of those two colours making two mana,
// unrestricted. It is a second mana ability beside a plain {T}: Add {C}
// (kernel mana_filterhybrid.go).
func HybridFilterManaAb(a *Ability) bool {
	m := &a.Cost.Mana
	h := m.Hyb[0]
	if a.Kind != AbMana || a.Cost.Flags != CostTap || m.Generic != 0 || m.X != 0 || m.Pips != ([NumManaKinds]uint8{}) ||
		m.Hyb[1] != 0 || m.Hyb[2] != 0 || m.Hyb[3] != 0 || m.Hyb[4] != 0 || h == 0 || h&^ColAny != 0 || h&(h-1) == 0 || h&(h-1)&(h&(h-1)-1) != 0 ||
		a.AFlags != AfManaChoice || a.ManaN != 2 || a.ManaColors != h || a.ManaRestrict != 0 || a.ManaEach || len(a.Prog) != 0 ||
		a.Gate.Ctr != 0 || a.Trig.Cond != 0 || a.ActZone != ZoneNone && a.ActZone != ZoneBattlefield {
		return false
	}
	return true
}
