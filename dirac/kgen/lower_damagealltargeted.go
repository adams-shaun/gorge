package kgen

import "strings"

// damageAllTargetedPlayers lowers DamageAll ValidPlayers$ Targeted: the sweep
// also damages the player the resolution targeted (gorge validPlayers: the
// players among the chosen targets), after the permanents. The player is
// read through one player target slot of the ability, which rides Obj, as
// PumpAll's player scope does.
//
//   - A ValidTgts$ of the node's own, player-kind, declares that slot and
//     scopes the object sweep to that player's permanents (gorge
//     targetPlayerKindScope: Chandra, Bold Pyromancer; CS "Scope").
//   - With none, the slot is the chain's one player slot (Cerebral Eruption's
//     PeekAndReveal target) and the sweep is the filter alone.
//
// Any other shape -- no ValidCards$ (a player-only sweep), a second player
// slot, a ValidTgts$ naming a permanent -- refuses.
func (lc *lowerCtx) damageAllTargetedPlayers(ps *params, n lValue) error {
	vc, ok := ps.get("ValidCards")
	if !ok {
		return refuse("DamageAll ValidPlayers$ Targeted without ValidCards$")
	}
	slot, flags := -1, "TgtPlayer"
	if vt, ok := ps.get("ValidTgts"); ok {
		pf, err := lc.filter(vt)
		if err != nil {
			return err
		}
		if len(pf.Clauses) != 0 || pf.Players == 0 {
			return refuse("DamageAll ValidTgts$ %q: not a player target", vt)
		}
		slot, flags = lc.addTarget(lTarget{F: pf}), flags+"|Scope"
	} else {
		for i, t := range lc.ab.Targets {
			if t.F == nil || t.F.Players == 0 {
				continue
			}
			if slot >= 0 {
				return refuse("DamageAll ValidPlayers$ Targeted over two player target slots")
			}
			slot = i
		}
		if slot < 0 {
			return refuse("DamageAll ValidPlayers$ Targeted without a player target")
		}
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("DamageAll ValidCards$ %q beside ValidPlayers$ Targeted", strings.TrimSpace(vc))
	}
	lc.emit(lInstr{Op: "DamageAll", A: n, F: f, Obj: lRef{Kind: rTarget, N: slot}, CS: flags})
	return nil
}
