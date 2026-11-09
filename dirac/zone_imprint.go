package dirac

// The resolution's imprinted set (genState.imp): objects an Imprint$ writer
// (kop.MvImprint) set aside for a later instruction of the same resolution
// to read (kop.RefImprinted), emptied by kop.ExtClearImprinted and at the
// start of the next resolution. It is separate from the remembered set
// (gorge's Imprinted and Remembered are independent lists, and one object
// may be in both) and from the chosen set the older imprint shapes borrow.

// StallImprint: more than maxImp (gen.go) objects imprinted in one resolution.
const StallImprint = 145

func init() { stallNames[StallImprint] = "imprint" }

// imprint adds o to the imprinted set, once.
func (s *State) imprint(o Obj) {
	for i := 0; i < int(s.g.nimp); i++ {
		if s.g.imp[i] == o {
			return
		}
	}
	if s.g.nimp >= maxImp {
		s.capFull(StallImprint)
		return
	}
	s.g.imp[s.g.nimp] = o
	s.g.nimp++
}
