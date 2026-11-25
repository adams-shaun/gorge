package dirac

// A spell copy's NonLegendary$ True (The Sixth Doctor, Jackal, Genius
// Geneticist, Double Major, Storm of Saruman, The Clone Saga: "copy it,
// except the copy isn't legendary"; gorge stackCopyRiders folds it into the
// copy as Object.CopyNonLegendary, CR 707.9b). OpCopySpell's B names the
// table's RemoveLegendary override (index+1); the copy's stack entry keeps it
// in remVer bits 8-15 (a spell has no remembered object, and bits 0-7 hold
// its gained keywords and stamps). A copy of that copy inherits it, as gorge's
// copy of a stripped copy does (CR 707.2). As a permanent copy resolves, the
// override becomes the token's layer-1 copy exception (copy_exceptions.go),
// pushed by enterGen before its enters triggers look: the token's type line
// has no Legendary, so the legend rule never gathers it with its original
// (CR 704.5j). Cold: reached only behind genPrep.anyPermCopy.

const (
	spellCopyExcShift         = 8
	spellCopyExcMask  objWord = 0xff << spellCopyExcShift
)

// DebugSpellCopyNonLegKeep is the mutation: the copies stay legendary.
var DebugSpellCopyNonLegKeep bool

// spellCopyExcEnter arms the copy exception of the resolving permanent spell
// copy e for the slot it enters next (enterGen reads genState.entExc).
//
//go:noinline
func (s *State) spellCopyExcEnter(e *stackEntry) {
	if DebugSpellCopyNonLegKeep || e.kind != seSpell || !s.isSpellCopy(e.obj) {
		return
	}
	if x := e.remVer & spellCopyExcMask >> spellCopyExcShift; x != 0 {
		s.g.entExc = entExcOn | uint32(NoSlot)<<8 | uint32(x)
	}
}
