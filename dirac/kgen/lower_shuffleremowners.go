package kgen

import "strings"

// shuffleRememberedOwners lowers Shuffle Defined$ RememberedOwner (gorge
// effShuffle over definedSpecRememberedOwner = ownersOf the source's
// remembered set, CR 108.3, 701.24): every distinct owner of a remembered
// object shuffles their library, in remembered order (Elixir of Immortality:
// the Elixir and your graveyard went to their owners' libraries with
// RememberChanged$, then each owner shuffles). It lowers only where the
// kernel's per-resolution remembered set is the one gorge reads: this chain
// already wrote it (a Remember move before this node) and ends with a
// Cleanup that clears it, so no earlier resolution's set is carried over.
func (lc *lowerCtx) shuffleRememberedOwners(ps *params) error {
	ps.get("Defined") // read
	if _, tgt := ps.get("ValidTgts"); tgt {
		return refuse("Shuffle Defined$ RememberedOwner with a target")
	}
	if lc.curNode == nil || lc.curNode.Sub == nil {
		return refuse("Shuffle Defined$ RememberedOwner without a clearing Cleanup after it")
	}
	n := lc.curNode.Sub
	if n.API != "Cleanup" || n.Sub != nil || len(n.Params) != 1 || !strings.EqualFold(n.Params["ClearRemembered"], "True") {
		return refuse("Shuffle Defined$ RememberedOwner not followed by the chain's clearing Cleanup")
	}
	written := false
	for i := len(lc.ab.Prog) - 1; i >= 0; i-- {
		in := &lc.ab.Prog[i]
		if in.Op == "ClearRemembered" {
			break
		}
		if !strings.Contains(in.CS, "Remember") {
			continue
		}
		// only the writes the kernel honors: the source's own Origin$
		// Battlefield move to the library's top (kop.LibTopRememberBF) and a
		// ChangeZoneAll sweep (kop.MvRemember). Serene Remembrance's spell
		// moved from the stack and its targeted graveyard cards drop the
		// flag, so the set the kernel shuffles would be wrong.
		switch {
		case in.Op == "ToLibrary" && in.Obj.Kind == rSelf && in.C == 0 && strings.Contains(in.CS, "from=Battlefield"):
		case in.Op == "ChangeZoneAll":
		default:
			return refuse("Shuffle Defined$ RememberedOwner after a %s Remember move the kernel does not honor", in.Op)
		}
		written = true
	}
	if !written {
		return refuse("Shuffle Defined$ RememberedOwner with no Remember move earlier in the chain")
	}
	lc.emit(lInstr{Op: "Shuffle", Obj: lRef{Kind: rRememberedOwner}, C: 1})
	return nil
}
