package rules

import (
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// This file owns the face-aware plumbing for a MUTATED pile (CR 702.140d).
// state.Object's PileAbilityCount/At, PileFaceCount/At and PileStaticCount/At
// give the one flat view of a permanent's rules text (index-based, because
// every one of those walks runs per object per legal-actions pass and the
// repo's allocation budgets forbid a per-object slice); these helpers thread the owning face's SVar table
// through the rules-side reads that used to assume the top face, and resolve a
// pendingCast's flat ability index back to its SA and owning face.
//
// The merged ordinal convention is state's: 0 is the top face, i+1 is the i-th
// under-card.

// pileSVars returns the SVar table a pile position resolves against, with any
// AddSVar$ grant layered under the printed table (the same precedence
// sVarGateOK documents: a printed SVar of the same name wins). merged 0 is the
// top face, so every non-mutated read is unchanged -- grantedSVarsFor is only
// consulted for the top face, exactly as before.
func (e *Engine) pileSVars(id state.ObjID, merged int) map[string]string {
	o := e.G.Obj(id)
	if o == nil {
		return nil
	}
	if merged <= 0 {
		f := o.Face()
		if f == nil {
			return nil
		}
		svars := f.SVars
		if gr := e.grantedSVarsFor(id); gr != nil {
			merged := make(map[string]string, len(gr)+len(f.SVars))
			for k, v := range gr {
				merged[k] = v
			}
			for k, v := range f.SVars {
				merged[k] = v
			}
			svars = merged
		}
		return svars
	}
	return o.PileSVars(merged)
}

// pendingAbilitySA resolves a pendingCast's flat pile-ability index to the SA
// and the merged ordinal of the face that carries it. ok is false for a spell
// (ability < 0), a missing source, or a stale index -- every caller degrades
// to its previous no-op behaviour rather than panicking the one driver
// goroutine.
func (e *Engine) pendingAbilitySA(pc *pendingCast) (*cards.SA, int, bool) {
	if pc == nil || pc.ability < 0 {
		return nil, 0, false
	}
	o := e.G.Obj(pc.card)
	if o == nil {
		return nil, 0, false
	}
	pa, ok := o.PileAbilityAt(pc.ability)
	if !ok {
		return nil, 0, false
	}
	return pa.SA, pa.Merged, true
}

// pileFaceForSA recovers the face that carries an activated ability of the
// permanent source, by SA pointer identity, walking the pile top-first. It is
// the activated-ability counterpart of findTriggerForAbilityFace: a resolving
// stack object carries o.Ability (the exact SA pointer events.Apply minted
// from the pile), and the face that owns it is the face whose SVar table the
// ability's body must resolve against. ok is false when the SA is not a
// printed ability of the source (a granted ability, a trigger body, or a
// source that left the battlefield) -- callers then keep today's top-face
// fallback.
func (e *Engine) pileFaceForSA(source state.ObjID, sa *cards.SA) (*cards.Face, bool) {
	if sa == nil {
		return nil, false
	}
	o := e.G.Obj(source)
	if o == nil {
		return nil, false
	}
	for i := 0; i < o.PileFaceCount(); i++ {
		pf, ok := o.PileFaceAt(i)
		if !ok {
			continue
		}
		for _, ab := range pf.Face.Abilities {
			if ab == sa {
				return pf.Face, true
			}
		}
	}
	return nil, false
}

// pileAbilityRefOf maps a printed ability pointer back to its flat pile index
// and merged ordinal. It is the inverse of Object.PileAbilityAt used by the
// mana path, which carries SA pointers rather than indices. ok is false for an
// SA the source does not print.
func pileAbilityRefOf(o *state.Object, sa *cards.SA) (idx, merged int, ok bool) {
	if o == nil || sa == nil {
		return 0, 0, false
	}
	for i, n := 0, o.PileAbilityCount(); i < n; i++ {
		pa, at := o.PileAbilityAt(i)
		if !at {
			continue
		}
		if pa.SA == sa {
			return i, pa.Merged, true
		}
	}
	return 0, 0, false
}
