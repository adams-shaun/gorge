package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// extBar runs ExtBar: the fixture's annotated handler.
//
//kmanifest:ext ExtBar
func (s *State) extBar(k *prep, in *kop.Instr) (ok, done bool) {
	return true, true
}
