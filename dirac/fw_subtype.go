package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwSubtype: FwSubtype, the object has subtype index Arg (Table.Subtypes; the
// read of Filter.Sub). A permanent reads its current subtypes at the matcher's
// layer, a layer-walk view the walked ones, a card its printed ones; a
// face-down object has none (CR 708.2).
//
//kmanifest:fword FwSubtype
func (s *State) fwSubtype(c *fctx, l *kop.FLit) bool {
	if l.Arg < 0 {
		return false
	}
	i := uint8(l.Arg)
	switch {
	case c.walk:
		return c.perm() && hasSubCur(c.k, &c.cur, i)
	case c.perm():
		return s.permHasSub(c.k, c.sl, i, c.layer)
	case c.card():
		return !c.fd && int(i) < len(c.k.t.Subtypes) && c.k.t.Cards[c.c].Subtypes.Has(int(i))
	}
	return false
}
