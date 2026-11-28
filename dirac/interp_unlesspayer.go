package dirac

// tgtCtrlOrYou is kop.RefTgtCtrlOrYou: an unless payer over the head's one
// optional target slot. A chosen target names its controller (RefTargetController);
// a slot that answered empty names the resolving entry's controller, as gorge's
// unlessProceed falls back to c.Controller for the default target-derived selector
// over an empty target list (effects/unless.go unlessPayerControllerFallback).
// A chosen target that went illegal is not the empty answer: it names nobody, and
// fizzles the one-slot ability anyway (CR 608.2b).
func (s *State) tgtCtrlOrYou(k *prep) uint8 {
	if p := s.tgtCtrlSlot(k, 0); p != 255 {
		return p
	}
	if s.res.e.tgt[0] == NoTarget {
		return s.res.e.ctrl
	}
	return 255
}
