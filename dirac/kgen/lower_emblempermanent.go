package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// emblemStatics lowers a no-target, no-RememberObjects$ Duration$ Permanent
// Effect with several StaticAbilities$ bodies as one emblem carrying every
// static in SVar-list order (Saheeli, Filigree Master). list is the raw
// StaticAbilities$ value. A target or a remembered set refuses with the
// list's own name: an emblem has no captured set. Each body must lower to
// exactly one static; any other body refuses by its own name.
func (lc *lowerCtx) emblemStatics(ps *params, list string, ss []cards.Static) error {
	if _, ok := ps.m["ValidTgts"]; ok || ps.m["RememberObjects"] != "" {
		return refuse("Effect static %q", list)
	}
	ps.consume("Name", "Image", "Stackable")
	var all []lStatic
	for _, s := range ss {
		if sp := newParams(s.Params); s.Mode == "Continuous" && sp.is("MayPlay", "True") && affectedRemembered(sp.str("Affected")) {
			return refuse("Effect MayPlay over the remembered cards, unsupported shape")
		}
		sts, err := lc.staticBody(s)
		if err != nil {
			return err
		}
		if len(sts) != 1 {
			return refuse("Effect static Mode %s lowers to %d statics beside others", s.Mode, len(sts))
		}
		if err := emblemFiltersRefuse("Emblem static", sts[0].F, sts[0].F2); err != nil {
			return err
		}
		all = append(all, sts[0])
	}
	lc.out.Emblems = append(lc.out.Emblems, lEmblem{Statics: all})
	lc.emit(lInstr{Op: "CreateEmblem", Obj: lRef{Kind: rYou}, C: len(lc.out.Emblems) - 1})
	return nil
}
