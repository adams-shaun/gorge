package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwGoaded: FwGoaded, the battlefield permanent is goaded by any player
// (gorge engine/effects/filter.go positiveRecognised / matchPositive's
// IsGoaded: an event-backed Goads relationship, or a live Goad$ True static).
// goadedAt is both halves: the OpGoad marks and the static read. An object
// elsewhere carries no goad, so it fails closed. A layer-walk view is not read
// (the static half walks the layers; the manifest entry declares no "cur").
//
//kmanifest:fword FwGoaded
func (s *State) fwGoaded(c *fctx, l *kop.FLit) bool {
	return c.perm() && !c.walk && s.goadedAt(c.k, c.sl)
}
