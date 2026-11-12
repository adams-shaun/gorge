package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// PutCounter's Choices$ pick and RememberPut$ (gorge
// effects/counters_put.go putCounterChooseTitled and rememberPlaced). The
// pick is an OpChooseCard with C = choosePutCounterPick, always followed by
// the OpPutCounter on RefChosen: gorge evaluates CounterNum$ first and does
// nothing for an amount <= 0, poses no ask for an empty pool, takes a lone
// candidate unasked, and otherwise asks one mandatory KChoose (Min 1, Max 1,
// counter_pick) over the battlefield permanents in seat order. RememberPut$
// is OpPutCounter B=2: the recipient is remembered when the CounterNum$
// value is > 0, before replacements (not the landed count).
const choosePutCounterPick = 6

// DebugPutCtrPickAlwaysAsk poses the ask even for a lone candidate, which
// gorge takes unasked. Test-only (TestPutctrparamsMutation).
var DebugPutCtrPickAlwaysAsk bool

// DebugRemPutNever drops RememberPut$'s remember. Test-only
// (TestPutctrparamsMutation).
var DebugRemPutNever bool

// putCounterPick is OpChooseCard C choosePutCounterPick for chooser p.
func (s *State) putCounterPick(k *prep, in *kop.Instr, p uint8) (ok, done bool) {
	if s.value(k, s.prog(k)[int(s.res.pc)+1].A) <= 0 {
		return true, true // gorge putCounterChooseTitled: n <= 0 returns first
	}
	n := 0
	first := NoObj
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.matchPermGen(k, in.F, sl, p, NoSlot, layerFull) {
			if n == 0 {
				first = s.perms[sl].obj
			}
			n++
		}
	}
	switch {
	case n == 0:
		return true, true
	case n == 1 && !DebugPutCtrPickAlwaysAsk:
		s.g.chosen[0], s.g.nchosen = first, 1
		return true, true
	}
	s.res.ask, s.res.askP = raGenChooseCard, p
	return false, true
}

// putCounterRemPut is OpPutCounter's RememberPut$ arm for the recipient sl
// when placeCounters landed m counters: gorge rememberPlaced names only the
// recipients whose counter actually landed (a prohibition or a zeroing
// replacement remembers nothing).
//
//go:noinline
func (s *State) putCounterRemPut(k *prep, sl Slot, m int) {
	if DebugRemPutNever || m <= 0 {
		return
	}
	s.remember(s.perms[sl].obj)
}
