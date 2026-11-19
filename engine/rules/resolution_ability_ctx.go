package rules

import (
	"maps"
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// The ability-object Ctx seeds resolveTop's ability branch and
// resumeResolution share. A suspended resolution
// re-enters through resumeResolution with a freshly built Ctx, so every seed
// the first pass derived from the stack object must be derived the same way
// on the resume, or the resumed chain reads a different Ctx than the one it
// suspended under. Each helper below is the one home of one such seed; the
// Ctx-field parity ratchet (resume_ability_ctx_test.go) holds the two builders
// to the same field set.

// abilityResolutionSVars is the SVar table an ability stack object's chain
// resolves against. The ability object has no Face, so the table comes from
// the permanent it belongs to: the face that OWNS the resolving trigger
// (findTriggerForAbilityFace -- a mutated pile's under-card trigger, CR
// 702.140d), else the face owning an activated ability of a pile
// (pileFaceForSA -- Porcuparrot's `NumDmg$ X` resolves X on ITS face), else
// the source's current face. A granted (AddTrigger$) or Effect-delivered
// delayed trigger's body is the GRANTOR's SVar-named line, so the table
// pushTrigger recorded with that line (Engine.triggerLineSVars) wins: the
// recipient's face does not define the body's SVars, and the grant may have
// ended before the ability resolves. A cross-object granted ACTIVATED ability
// (AddAbility$, events.GrantAbilityPush) reads the same way: its body is the
// grantor's SVar-named line, so the grantor's table comes first and the
// recipient's fills only the names the grantor does not define. A source that has left the battlefield degrades to a
// nil table.
func (e *Engine) abilityResolutionSVars(id state.ObjID, o *state.Object) map[string]string {
	if isStationAbility(o.Ability) {
		// The Station body (cards.KeywordPushAbility) reads only its bound X
		// (bindStationAmount); a Spacecraft's own SVar:X (Fell Gravship,
		// Warmaker Gunship) must not shadow it.
		return nil
	}
	var svars map[string]string
	if src := e.G.Obj(o.Source); src != nil {
		if _, mf, ok := e.findTriggerForAbilityFace(o.Source, o.Ability); ok && mf != nil {
			svars = mf.SVars
		} else if mf, ok := e.pileFaceForSA(o.Source, o.Ability); ok && mf != nil {
			svars = mf.SVars
		} else if pf := printedFaceOfActivated(src, o.Ability); pf != nil {
			svars = pf.SVars
		} else if sf := src.Face(); sf != nil {
			svars = sf.SVars
		}
	}
	if owned, ok := e.triggerLineSVars[id]; ok {
		svars = owned
	} else if o.GrantedBy != 0 && o.GrantedBy != o.Source {
		// Forge builds the granted ability with the granting static as its
		// SVar holder. Food Fight's granted damage names an X only Food Fight
		// defines: read off the artifact it dealt 0, and an X the recipient
		// happens to define must not stand in for it either. The grantor
		// keeps its id after it leaves the battlefield, and the ability
		// exists apart from it (CR 113.7a), so a gone grantor still names
		// its table.
		if g := e.G.Obj(o.GrantedBy); g != nil && g.Face() != nil {
			// Multi-face grantor: the mint (events.ResolveSVarAcrossFaces)
			// took the body from the first face whose table names it -- the
			// active face, then every face in index order, then what a copy
			// left behind. The body's own SVar references resolve against
			// THAT face's table, not whatever face the grantor shows now: a
			// DFC grantor that left the battlefield is front-face-up in its
			// zone (CR 711.5a) while the granted body is its source's text
			// (CR 113.7a). A one-face grantor picks its active face, exactly
			// as before.
			grantor := grantedBodySVars(g, o.Ability)
			if grantor == nil {
				grantor = g.Face().SVars
			}
			if len(grantor) > 0 {
				if len(svars) > 0 {
					merged := maps.Clone(svars)
					maps.Copy(merged, grantor)
					grantor = merged
				}
				svars = grantor
			}
		}
	}
	return svars
}

// grantedBodySVars is the multi-face read of a granted ability's grantor:
// the first face of the grantor's card, in the mint's own order (the active
// face, then every face in index order, then LeftCopyFace), whose SVar table
// defines a name the granted body's lines mention. nil when no face does --
// the caller then keeps the active face's table, the pre-multi-face read.
// The mention scan runs over the body's own Line texts (SubAbility$ chains
// included) with word boundaries, so a name that is a substring of another
// does not qualify a face by accident; the quantifier is existential, so the
// map iteration order is irrelevant.
func grantedBodySVars(g *state.Object, body *cards.SA) map[string]string {
	if body == nil {
		return nil
	}
	var lines []string
	for ab := body; ab != nil; ab = ab.Sub {
		if ab.Line != "" {
			lines = append(lines, ab.Line)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	mentions := func(svars map[string]string) bool {
		for name := range svars {
			for _, ln := range lines {
				if mentionsWord(ln, name) {
					return true
				}
			}
		}
		return false
	}
	if f := g.Face(); f != nil && mentions(f.SVars) {
		return f.SVars
	}
	if g.Card != nil {
		for _, cf := range g.Card.Faces {
			if cf != nil && mentions(cf.SVars) {
				return cf.SVars
			}
		}
	}
	if g.LeftCopyFace != nil && mentions(g.LeftCopyFace.SVars) {
		return g.LeftCopyFace.SVars
	}
	return nil
}

// mentionsWord reports whether line contains name between word boundaries.
func mentionsWord(line, name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i+len(name) <= len(line); i++ {
		if line[i:i+len(name)] != name {
			continue
		}
		if (i == 0 || !isWordByte(line[i-1])) && (i+len(name) == len(line) || !isWordByte(line[i+len(name)])) {
			return true
		}
	}
	return false
}

func isWordByte(b byte) bool {
	return b == '_' || (b >= '0' && b <= '9') || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// printedFaceOfActivated is the printed face owning sa for a source that became a
// copy after sa was activated: the ability keeps its own text (CR 113.7a), and Forge
// reads its SVars from the state it was created on (CardTraitBase.getSVar, read).
func printedFaceOfActivated(src *state.Object, sa *cards.SA) *cards.Face {
	if src.CopyFace == nil || src.Card == nil || sa == nil {
		return nil
	}
	for _, f := range src.Card.Faces {
		if f != nil && slices.Contains(f.Abilities, sa) {
			return f
		}
	}
	return nil
}

// bindNinjutsuDefender re-binds CR 702.49b's defender: a K:Ninjutsu
// permanent enters attacking the same player (planeswalker or battle) the
// returned creature was attacking. The activator captured that defender when
// the Return cost was paid (engine/rules/cast.go's returncost arm) and it rides the
// AbilityPush event's IDs, which events.Apply folded into o.Remembered as a
// player target (and the planeswalker/battle as a real id after it), so
// engine/effects/zone.go's Attacking$ True rider reads Ctx.DefendingPlayer /
// DefendingBattle. Only a ninjutsu activation carries the tag, so no other
// resolution's Remembered player is reinterpreted as a defending player.
func bindNinjutsuDefender(ctx *effects.Ctx, o *state.Object) {
	ab := o.Ability
	if ab == nil || !pay.SaHasKeyword(ab, "Ninjutsu") {
		return
	}
	ctx.DefenderFixed = true
	for _, rem := range o.Remembered {
		if rem.IsPlayer {
			ctx.DefendingPlayer = rem
			continue
		}
		if rem.Obj != 0 {
			ctx.DefendingBattle = rem.Obj
		}
	}
}

// bindStationAmount binds a resolving Station ability's X -- its CounterNum$
// -- to the tapped creature's tap-power value at resolution (CR 702.184a,
// engine/rules/station.go stationAmount). Both Ctx builders call it AFTER their X
// reads, so it wins over the stack object's (unpaid) X. Any other ability is
// untouched.
func (e *Engine) bindStationAmount(ctx *effects.Ctx, o *state.Object) {
	if n, ok := e.stationAmount(o); ok {
		ctx.X = n
	}
}

// abilityXAnnounced is an ability stack object's CR 107.3i announced-X flag:
// the stack object's own announcement, or a paid nonzero X recorded by
// commitCast's CastInfo.
func abilityXAnnounced(o *state.Object) bool {
	return stackXAnnounced(o) || o.X != 0
}
