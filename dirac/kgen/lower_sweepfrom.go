package kgen

import "strings"

// DamageAll with DamageSource$ ParentTarget over
// "<spec>.NotDefinedTargeted" (CR 120.3; gorge effects/damage.go
// effDamageAll with damageSourceParam) as OpExt ExtSweepFrom: the chain's one
// earlier target creature deals the amount (read once) to every matching
// creature in one damage batch, the source's own slot excluded by the
// suffix. Anything beyond that shape refuses by name.
func (lc *lowerCtx) sweepFrom(ps *params, ds string, n lValue) error {
	for _, k := range []string{"ValidPlayers", "RememberDamaged", "ReplaceDyingDefined", "DamageMap", "ValidTgts"} {
		if _, ok := ps.m[k]; ok {
			return refuse("DamageAll DamageSource$ beside %s", k)
		}
	}
	if ds = strings.TrimSpace(ds); ds != "ParentTarget" && ds != "Targeted" {
		return refuse("DamageAll DamageSource$ %q", ds)
	}
	if len(lc.chainT) != 1 {
		return refuse("DamageAll DamageSource$ %s over %d target slots", ds, len(lc.chainT))
	}
	slot := lc.chainT[0]
	if t := lc.ab.Targets[slot]; t.Spell || t.F == nil || t.F.Players != 0 || t.Zone != zBattlefield {
		return refuse("DamageAll DamageSource$ %s names a slot that is not a battlefield object", ds)
	}
	vc, ok := ps.get("ValidCards")
	if !ok {
		return refuse("DamageAll DamageSource$ with no ValidCards$")
	}
	vc = strings.TrimSpace(vc)
	excl := 0
	for _, suf := range []string{".NotDefinedTargeted", "+NotDefinedTargeted"} {
		if rest, ex := strings.CutSuffix(vc, suf); ex {
			vc, excl = rest, 1
			break
		}
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	if f == nil || f.Players != 0 {
		return refuse("DamageAll DamageSource$ with a player scope in ValidCards$ %q", vc)
	}
	for _, c := range f.Clauses {
		if c.Types&^tCreature != 0 || c.Types == 0 {
			return refuse("DamageAll DamageSource$ over a non-creature filter %q", vc)
		}
	}
	lc.emit(lInstr{Op: "SweepFrom", Obj: lRef{Kind: rTarget, N: slot}, B: n, F: f, C: excl})
	return nil
}
