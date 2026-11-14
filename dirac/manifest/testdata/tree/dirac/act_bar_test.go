package dirac

// A test file's directive is not a handler.
//
//kmanifest:ext ExtTestOnly
func (s *State) extTestOnly(k *prep, in *kop.Instr) (ok, done bool) { return true, true }
