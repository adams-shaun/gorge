package oracle

import (
	"maps"
	"slices"
	"sync"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
)

// triggerOwners caches the card face a trigger's Effect belongs to, keyed by
// the compiled SA pointer. The universe is immutable; a pointer therefore has
// one owner for the life of the process.
var triggerOwners sync.Map // map[*cards.SA]string

// triggerOwnerName is the name of the card whose face owns trigger SA sa, or
// "" when no card or token definition carries it (a synthesized face).
func triggerOwnerName(g *state.Game, sa *cards.SA) string {
	if g == nil || sa == nil {
		return ""
	}
	if v, ok := triggerOwners.Load(sa); ok {
		return v.(string)
	}
	name := ""
	if u := g.NameUniverse; u != nil {
		for i := 0; i < u.Len() && name == ""; i++ {
			c := u.Card(i)
			for _, f := range c.Faces {
				for _, tr := range f.Triggers {
					if tr.Effect == sa {
						name = f.Name
						break
					}
				}
			}
		}
	}
	for _, k := range slices.Sorted(maps.Keys(g.Tokens)) {
		if name != "" {
			break
		}
		tok := g.Tokens[k]
		if tok == nil {
			continue
		}
		for _, f := range tok.Faces {
			for _, tr := range f.Triggers {
				if tr.Effect == sa {
					name = f.Name
					break
				}
			}
		}
	}
	triggerOwners.Store(sa, name)
	return name
}

// cloneKeptTriggerOf reports whether trigger i of o's face is one a Clone's
// GainThisAbility$ True appended to the copy (CR 707.9b; gorge effClone adds the
// resolving trigger after the copied face's own), as opposed to one of the
// copied card's printed triggers. The kernel lists the appended ones as granted
// (latent) triggers, whose stack index is 255.
//
// Every layer copies the template's face and, without a rename, the face keeps
// its name, so the printed card the chain bottoms out in is the one named by
// o's face; its triggers are the face's first len(root) and everything after is
// appended. A trigger whose Effect the root also carries is reported at the
// root's index (abilityIndex finds the first match), so the index i passed in
// is already the root's for those.
//
// Two cases the face name cannot answer: a GainThisAbility$ True copy appends
// the resolving trigger as the face's LAST trigger (even when the copied card
// prints a trigger of the same Effect, abilityIndex reports the earlier copied
// index), and a NewName$ copy (CR 707.9b) is named by the card it printed while
// its triggers are still the copied template's. The first is the top layer's
// GainThisAbility; the second is a trigger owned by the card the copy took,
// which the LCopy marker's CloneSource names.
//
// A token template (Cryptoplasm copying a Hamster Token and keeping its own
// trigger) is named by no card of the universe: the match's token definitions
// are the root there, walked in sorted-script order so the answer never
// depends on the map's order.
func cloneKeptTriggerOf(e *rules.Engine, o *state.Object, i int) bool {
	if e == nil || o == nil || len(o.CloneLayers) == 0 || o.Face() == nil || i < 0 || i >= len(o.Face().Triggers) {
		return false
	}
	g := e.G
	top := &o.CloneLayers[len(o.CloneLayers)-1]
	if top.GainThisAbility && i == len(o.Face().Triggers)-1 {
		return true
	}
	if tok, face := rootTokenCard(g, o.Face().Name); tok != nil {
		return i >= len(face.Triggers)
	}
	ord, fi, ok := g.NameUniverse.FaceByName(o.Face().Name)
	if !ok {
		return false
	}
	root := g.NameUniverse.Card(ord).Faces[fi]
	if i < len(root.Triggers) {
		return false
	}
	if newName := cloneLayerNewName(e, o); newName != "" {
		if src := cloneLayerSource(e, o); src != nil {
			if owner, cur := triggerOwnerName(g, o.Ability), cloneCurrentCardName(e, src); owner != "" && owner == cur {
				return false // the copied card's own trigger, under its new name
			}
		}
	}
	return true
}

// cloneLayerSource is the object the LCopy marker owning o's top layer-1 copy
// effect took the copy from, nil when no marker matches (a token copy made by
// another primitive, say).
func cloneLayerSource(e *rules.Engine, o *state.Object) *state.Object {
	if e == nil || o == nil || len(o.CloneLayers) == 0 {
		return nil
	}
	top := o.CloneLayers[len(o.CloneLayers)-1]
	for _, ce := range e.RegisteredContinuous() {
		if ce.Layer != state.LCopy || ce.CloneTarget != o.ID || ce.CloneSeq != top.Seq {
			continue
		}
		if ce.CloneSource == 0 || ce.CloneSource == o.ID {
			return nil
		}
		return e.G.Obj(ce.CloneSource)
	}
	return nil
}

// cloneLayerNewName is the NewName$ o's own top copy layer carries ("" when
// none): the LCopy marker's CloneName, the same rider the layer locked.
func cloneLayerNewName(e *rules.Engine, o *state.Object) string {
	if e == nil || o == nil || len(o.CloneLayers) == 0 {
		return ""
	}
	top := o.CloneLayers[len(o.CloneLayers)-1]
	for _, ce := range e.RegisteredContinuous() {
		if ce.Layer != state.LCopy || ce.CloneTarget != o.ID || ce.CloneSeq != top.Seq {
			continue
		}
		return ce.CloneName
	}
	return ""
}

// cloneCurrentCardName is the name of the card o currently shows to the
// kernel's stack projection: its face, unless its own top copy layer renamed
// it (NewName$, CR 707.9b), in which case the copied card is the source's.
func cloneCurrentCardName(e *rules.Engine, o *state.Object) string {
	if e == nil || o == nil {
		return ""
	}
	if len(o.CloneLayers) != 0 && cloneLayerNewName(e, o) != "" {
		if src := cloneLayerSource(e, o); src != nil {
			return cloneCurrentCardName(e, src)
		}
	}
	face := o.Face()
	if o.Zone != state.ZBattlefield && o.LeftCopyFace != nil {
		face = o.LeftCopyFace
	}
	if face == nil {
		return ""
	}
	return face.Name
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
func grantedTriggerOf(e *rules.Engine, o *state.Object, ab uint8) bool {
	if ab&0x80 == 0 || ab == 255 {
		return false
	}
	i := int(ab & 0x7F)
	return copyGrantedTriggerOf(o, i) || cloneKeptTriggerOf(e, o, i)
}
