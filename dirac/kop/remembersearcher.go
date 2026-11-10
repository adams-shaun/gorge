package kop

// ExtRememberSearcher: the searching player joins the resolution's remembered
// set (ChangeZone RememberSearcher$ True on a library search, CR 701.23b: the
// search counts whatever it finds). The kernel's set holds objects, so the
// player is counted in the remembered count (genState.remCnt, the channel
// ExRememberedCount sums) and nowhere else; kgen lowers the flag only where
// that count is the only read. Emitted right before the search.
// dirac/zone_remembersearcher.go.
const ExtRememberSearcher ExtOp = 304

func init() { extOpNames[ExtRememberSearcher] = "RememberSearcher" }
