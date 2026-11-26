package kgen

import "strings"

// DamageAll with DamageSource$ over two sweep shapes (CR 120.3; gorge
// effects/damage.go effDamageAll with damageSourceParam), both OpExt
// ExtSweepFrom:
//
//   - ParentTarget/Targeted over "<spec>.NotDefinedTargeted": the chain's one
//     earlier target creature deals the amount (read once) to every matching
//     creature in one damage batch, the source's own slot excluded by the
//     suffix.
//   - Remembered in a capturing emblem's DamageDealtOnce body
//     (lowerCtx.capRem; Arcbond's emblem shape): the captured object is the
//     damage source (rTriggeredSource), and ValidPlayers$ Player (exactly
//     that value) adds the player arm after the creatures in the same batch
//     (C&2). The body's Creature.!IsRemembered folds into the C&1 own-slot
//     exclusion: the captured object is the sweep source, and the kernel's
//     emblem does not seed the remembered set a filter flag would read
//     (emblemCaptureFilter). A plain IsRemembered clause beside the capture
//     names exactly the captured object (the sweep would hit only its own
//     source) and refuses; a body that wrote its own set first keeps the
//     plain Remembered read the definedSpec census owns, so it refuses too.
//   - ParentTarget/Targeted beside ValidPlayers$ Player.Opponent (Chandra's
//     Ignition): the targeted creature deals its power to every other
//     creature and, C&4, to each living opponent of the resolving controller,
//     after the creatures in the same batch.
//
// Anything beyond those shapes refuses by name.
func (lc *lowerCtx) sweepFrom(ps *params, ds string, n lValue) error {
	for _, k := range []string{"RememberDamaged", "ReplaceDyingDefined", "DamageMap", "ValidTgts"} {
		if _, ok := ps.m[k]; ok {
			return refuse("DamageAll DamageSource$ beside %s", k)
		}
	}
	cap := lc.capRem && strings.TrimSpace(ds) == "Remembered" && !lc.hasRememberWriter()
	players, opps := false, false
	vp, hasVP := ps.get("ValidPlayers")
	if hasVP && cap {
		if strings.TrimSpace(vp) != "Player" {
			return refuse("DamageAll DamageSource$ beside ValidPlayers")
		}
		players = true
	}
	var src lRef
	if cap {
		// the captured object, the trigger's damage source, wherever it is
		// now (a gone one deals from the resolving effect, as ExtSweepFrom's
		// NoSlot arm already does)
		src = lRef{Kind: rTriggeredSource}
	} else {
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
		src = lRef{Kind: rTarget, N: slot}
		if hasVP {
			// the player arm beside a targeted source (Chandra's Ignition:
			// DamageSource$ ParentTarget, ValidPlayers$ Player.Opponent): the
			// sweep's player hits name each opponent of the resolving
			// controller, after the creatures in the same batch (C bit 4).
			// Only that one spelling lowers; gorge's ValidPlayers read is the
			// same filter spec the plain DamageAll arm lowers, and no other
			// corpus carrier pairs a targeted source with a player scope.
			// The check sits after the source's resolution, so a source the
			// sweep cannot lower refuses by its DamageSource$ value, not by
			// the player scope beside it.
			if strings.TrimSpace(vp) != "Player.Opponent" {
				return refuse("DamageAll DamageSource$ beside ValidPlayers")
			}
			opps = true
		}
	}
	vc, ok := ps.get("ValidCards")
	if !ok {
		return refuse("DamageAll DamageSource$ with no ValidCards$")
	}
	vc = strings.TrimSpace(vc)
	excl := 0
	if !cap {
		for _, suf := range []string{".NotDefinedTargeted", "+NotDefinedTargeted"} {
			if rest, ex := strings.CutSuffix(vc, suf); ex {
				vc, excl = rest, 1
				break
			}
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
	if cap {
		nNot := 0
		for i := range f.Clauses {
			if f.Clauses[i].Flags&fNotRemembered != 0 {
				nNot++
			}
		}
		if nNot != 0 && nNot != len(f.Clauses) {
			// The clauses are an OR: C&1 skips the capture's slot for all of
			// them, but a clause without !IsRemembered still matches it in
			// gorge. Only a filter excluding it in every clause folds.
			return refuse("DamageAll DamageSource$ over a mixed IsRemembered filter %q", vc)
		}
		for i := range f.Clauses {
			if c := &f.Clauses[i]; c.Flags&fRemembered != 0 {
				// IsRemembered with no qualifier beside it: the remember set
				// names exactly the captured object, so gorge's sweep hits
				// only the source itself -- not a sweep over the battlefield
				// at all, and not expressible as one without the remembered
				// set. Fail closed by name.
				return refuse("DamageAll DamageSource$ over an IsRemembered filter %q", vc)
			}
			if c := &f.Clauses[i]; c.Flags&fNotRemembered != 0 {
				// !IsRemembered: not the captured object, which is the sweep
				// source
				excl = 1
				c.Flags &^= fNotRemembered
			}
		}
	}
	c := excl
	if players {
		c |= 2
	}
	if opps {
		c |= 4
	}
	lc.emit(lInstr{Op: "SweepFrom", Obj: src, B: n, F: f, C: c})
	return nil
}
