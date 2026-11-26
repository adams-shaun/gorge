package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// effectPlayLocks lowers a remembered Effect whose statics are one may-play
// permission over the remembered exiled cards and the hand locks beside it
// (Memory Vessel's STPlay,NoLand,NoCast with Duration$ UntilYourNextTurn):
// the permission lowers to the MayPlay instruction the single-static
// remembered route emits (effectTargetStatic's Continuous case, the per-card
// records the emblem's IsRemembered filter cannot make -- the chain's
// DBCleanup empties the remembered set an emblem would read), the locks to
// one emblem (effectEmblemStaticHost's statics) whose lifetime is the
// permission's own Duration$ (effectStaticEOT's next-turn emblem for
// UntilYourNextTurn, the EOT emblem for the absent Duration$). gorge's
// effEffect registers all three statics on one effect, so the locks bind
// while the permission lives.
//
// Only this shape returns true: one Continuous MayPlay$ static over the
// remembered cards in exile, every other static a CantPlayLand or
// CantBeCast lock, Duration$ absent or UntilYourNextTurn. Anything else
// returns false and keeps the multi-static list refusal; a shape that
// matches the route but whose details a lowering cannot read refuses by
// name.
func (lc *lowerCtx) effectPlayLocks(ps *params, list string, ss []cards.Static) (bool, error) {
	if _, hasTgt := ps.m["ValidTgts"]; hasTgt {
		return false, nil
	}
	ro := ps.str("RememberObjects")
	if ro != "Remembered" && ro != "RememberedCard" {
		// gorge effectRememberedCodes: RememberedCard is Remembered
		return false, nil
	}
	d := ps.str("Duration")
	switch d {
	case "", "UntilYourNextTurn":
	default:
		return true, refuse("Effect play locks Duration$ %q", d)
	}
	perm := -1
	for i := range ss {
		s := ss[i]
		if s.Mode == "Continuous" {
			sp := newParams(s.Params)
			if sp.is("MayPlay", "True") && sp.str("AffectedZone") == "Exile" {
				if perm >= 0 {
					return false, nil // two permissions: not this route's shape
				}
				perm = i
				continue
			}
			return false, nil // another Continuous body: the list refusal keeps naming it
		}
		switch s.Mode {
		case "CantPlayLand", "CantBeCast":
		default:
			return false, nil
		}
	}
	if perm < 0 {
		return false, nil // locks without the permission: effectStaticEOT's route is not this one
	}
	obj, err := lc.defined(ro)
	if err != nil {
		return true, err
	}
	var locks []lStatic
	for i := range ss {
		if i == perm {
			continue
		}
		lst, err := lc.effectEmblemStaticHost(ss[i], false)
		if err != nil {
			return true, err
		}
		if lst.Kind == "" {
			return true, refuse("Effect static %s lock lowered to no static", ss[i].Mode)
		}
		if err := emblemFiltersRefuse("Effect static", lst.F, lst.F2); err != nil {
			return true, err
		}
		locks = append(locks, lst)
	}
	if err := lc.effectTargetStatic(ps, ss[perm], obj); err != nil {
		return true, err
	}
	emb := lEmblem{Statics: locks, UntilEOT: d == "", UntilNextTurn: d == "UntilYourNextTurn"}
	lc.out.Emblems = append(lc.out.Emblems, emb)
	lc.emit(lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rYou}, C: len(lc.out.Emblems) - 1})
	return true, nil
}
