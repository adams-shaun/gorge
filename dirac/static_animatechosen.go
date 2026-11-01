package dirac

// Chosen-colour animations (Puca's Eye, Mondo
// Gecko): an OpOverride whose B is kop.OvChosenColor applies, at
// resolution, the one of five consecutive override records whose SetColors
// is the colour the controller chose earlier in the same resolution
// (ExtChooseColorRes's res.chosen, 1..5 = WUBRG): record C + chosen - 1.
// The colour is baked into the applied record, so the later
// ClearChosenColor$ Cleanup (Mondo Gecko) and the resolution's end change
// nothing already granted, exactly as gorge's ClearChosenColor leaves the
// applied animate grant alone. An unset colour (res.chosen 0, or > 5) is
// gorge's fail-closed case: combatfx.go animateGrant registers no grant
// and the object keeps its printed colours, so the kernel applies nothing.

// DebugAnimChosenFirst (TestAnimchosenMutation) always applies the first
// (white) record: the baked colour is then wrong for four of five choices,
// so the lockstep diverges within a few seeds.
var DebugAnimChosenFirst bool

// ovChosenColor is OpOverride's chosen-colour arm (interp_gen.go); the hot
// path keeps just the one B test.
//
//go:noinline
func (s *State) ovChosenColor(k *prep, sl Slot, C int, card CardID, fl uint8) {
	c := s.res.chosen
	if DebugAnimChosenFirst {
		c = 1
	}
	if c == 0 || c > 5 {
		return
	}
	s.applyOverrideFl(k, sl, C+int(c)-1, card, fl)
}
