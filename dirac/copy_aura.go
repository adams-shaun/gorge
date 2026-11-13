package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A copy of an Aura spell elects a new target (CR
// 707.10c, 608.3f, 303.4f). gorge's rules/target_modes.go
// copyRootDeclarations reads the copied spell's SpellAbility(), and an Aura
// spell's is its attach ability with the "Enchant" restriction as its
// target, so a copy made with MayChooseTarget$ (Slick Imitator's "{1},
// sacrifice: copy target spell you control") poses AskCopyTargets before it
// resolves: the inherited host first (kept even when illegal), then the
// other legal hosts. The kernel's permanent spell (ab == 255) carries that
// target in the card's Enchant spec, slot 0 (tspec), declared when kgen
// marked it TgfDecl like any spell's root node.

// DebugAuraCopyOld restores the old behaviour, a copied Aura spell electing
// nothing (the kauracopy mutation test).
var DebugAuraCopyOld bool

// auraCopyDeclSlot is the copy declaration slot of a permanent spell e
// (copyDeclSlot's ab == 255 arm): 0 for an Aura spell cast to enchant its
// target, -1 for any other permanent spell (no spell ability to declare). A
// bestowed spell is not an Aura spell with a spell ability in gorge, so it
// asks nothing.
//
//go:noinline
func (s *State) auraCopyDeclSlot(k *prep, e *stackEntry) int {
	cd := &k.t.Cards[e.card]
	if DebugAuraCopyOld || cd.Types&kop.TAura == 0 || cd.Enchant.TFlags&kop.TgfDecl == 0 ||
		e.mode&(seFaceDown|modeBestowed) == modeBestowed {
		return -1
	}
	return 0
}

// setAuraCopyTarget writes the Aura copy's new host (setCopyTargetsQ's
// ab == 255 arm): the one pick replaces the cast-time host.
//
//go:noinline
func (s *State) setAuraCopyTarget(e *stackEntry, picks []Obj) {
	e.tgt[0] = uint8(NoTarget)
	for _, o := range picks[:min(len(picks), 1)] {
		c := uint8(o)
		var ver uint8
		if isObjTarget(TCode(c)) {
			ver = s.objs[c].ver
		}
		e.tgt[0], e.tver[0] = c, ver
	}
	if e.nt == 0 {
		e.nt = 1
	}
	s.touch(e.obj)
}
