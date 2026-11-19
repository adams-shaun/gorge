package oracle

import (
	"maps"
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// cloneKeptTriggerOf reports whether trigger i of o's face is one a Clone's
// GainThisAbility$ True appended to the copy (CR 707.9b; gorge effClone adds the
// resolving trigger after the copied face's own), as opposed to one of the
// copied card's printed triggers. The kernel lists the appended ones as granted
// (latent) triggers, whose stack index is 255.
//
// Every layer copies the template's face and the face keeps its name, so the
// printed card the chain bottoms out in is the one named by o's face; its
// triggers are the face's first len(root) and everything after is appended. A
// trigger whose Effect the root also carries is reported at the root's index
// (abilityIndex finds the first match), so the index i passed in is already the
// root's for those.
//
// A token template (Cryptoplasm copying a Hamster Token and keeping its own
// trigger) is named by no card of the universe: the match's token definitions
// are the root there, walked in sorted-script order so the answer never
// depends on the map's order.
func cloneKeptTriggerOf(g *state.Game, o *state.Object, i int) bool {
	if o == nil || len(o.CloneLayers) == 0 || o.Face() == nil || i < 0 || i >= len(o.Face().Triggers) {
		return false
	}
	if tok, face := rootTokenCard(g, o.Face().Name); tok != nil {
		return i >= len(face.Triggers)
	}
	ord, fi, ok := g.NameUniverse.FaceByName(o.Face().Name)
	if !ok {
		return false
	}
	return i >= len(g.NameUniverse.Card(ord).Faces[fi].Triggers)
}

// rootTokenCard is the match's token definition whose face is named name,
// with that face, when the universe carries no such card (tokens are
// compiled scripts beside it, keyed by script, so they are found by value).
func rootTokenCard(g *state.Game, name string) (*cards.Card, *cards.Face) {
	if name == "" {
		return nil, nil
	}
	if _, _, ok := g.NameUniverse.FaceByName(name); ok {
		return nil, nil
	}
	for _, k := range slices.Sorted(maps.Keys(g.Tokens)) {
		tok := g.Tokens[k]
		if tok == nil {
			continue
		}
		for _, f := range tok.Faces {
			if f != nil && f.Name == name {
				return tok, f
			}
		}
	}
	return nil, nil
}

// grantedTriggerOf reports whether stack ability ab (0x80|i: trigger i of o's
// face) is one the kernel lists as a granted (latent) trigger: an as-enters
// copy's "except it has myriad" (copyGrantedTriggerOf) or a trigger a Clone's
// GainThisAbility$ kept on the copy (cloneKeptTriggerOf).
func grantedTriggerOf(g *state.Game, o *state.Object, ab uint8) bool {
	if ab&0x80 == 0 || ab == 255 {
		return false
	}
	i := int(ab & 0x7F)
	return copyGrantedTriggerOf(o, i) || cloneKeptTriggerOf(g, o, i)
}
