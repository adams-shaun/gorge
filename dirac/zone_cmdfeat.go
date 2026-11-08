package dirac

// FeatCommander is seat p's commander (CR 903.3), NoObj when the seat has none.
func (s *State) FeatCommander(p uint8) Obj { return s.pl[p&1].cmd }
