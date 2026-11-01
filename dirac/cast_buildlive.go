package dirac

// buildLive reports whether s.b is the build in progress: the cast or
// activation being put on the stack. The build starts under ctlBuild
// (beginCast / beginActivation) and stays live while a generated-table ask
// suspends it (ctlGenAsk returning to ctlBuild). finishBuild pushes a copy
// of s.bld().e, so after it returns s.b holds only stale bytes: its li must not
// keep an LKI record referenced (lkiSweep), and its kind/obsSl/subCtr must
// not be read as if the entry were still being built (mana.go's cast-source
// taps, featview's wildcard ask).
func (s *State) buildLive() bool {
	return s.ctl == ctlBuild || s.ctl == ctlGenAsk && s.g.ctlRet == ctlBuild
}
