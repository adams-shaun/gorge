package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ExtChangeText: a CR 612 text change (gorge api:ChangeText; Mind Bend's two
// Choices$ bodies). gorge asks the chooser one combined KChoose, Min=Max=2,
// over a "from" word list and a "to" word list (options changetext_from /
// changetext_to, either order), then appends a substitution to the target;
// its face reads the new word everywhere (engine/cards/textchange.go).
//
// The kernel holds no per-permanent text, so it asks the same two picks
// (ChGenTextWord, two sequential decisions: the first offers both lists, the
// second the other list) and models the change only where it is a no-op: the
// target card's face never reads the "from" word (CardDef.TextWords, kgen
// textreach.go). A target that does read it stalls the game with
// StallTextChange (unmodelled) rather than answering from the unchanged
// face. from == to is a no-op in gorge too.
//
// The target slot is parked in res.peek[0] and the first pick in res.chosen
// (pick mode + 1, 0 = none yet).

// raChangeText is the ask of the two word picks.
const raChangeText uint8 = 243

// StallTextChange: a ChangeText whose "from" word the target's face reads;
// gorge rewrites the face, the kernel has no text to rewrite.
const StallTextChange = 190

// ChangeTextStats counts the mechanism for the lockstep tests: Asked is each
// ChangeText resolved, Reads those whose target's face read the "from" word
// (stalled).
var ChangeTextStats struct{ Asked, Reads atomic.Int64 }

func init() {
	stallNames[StallTextChange] = "textchange"
}

// changeTextTo is the mode bit that marks a "to" pick.
const changeTextTo = 16

// changeTextStart is ExtChangeText: parks the target and poses the picks;
// true: nothing to ask (no target).
//
//go:noinline
func (s *State) changeTextStart(k *prep, in *kop.Instr) bool {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return true
	}
	s.res.peek[0] = Obj(sl) // code-ok: the slot the picks apply to
	s.res.chosen = 0
	s.res.ask, s.res.askP = raChangeText, s.res.e.ctrl
	return false
}

// changeTextBase is the first word index of the instruction's list: 0 the
// colours, 5 the basic land types.
func changeTextBase(in *kop.Instr) uint8 {
	if in.C == int16(kop.ChangeTextLand) {
		return 5
	}
	return 0
}

// askOptionsChangeText lists the picks (a link of the askOptionsUnlockDoor
// chain): first both lists, then the list the first pick left out.
//
//go:noinline
func (s *State) askOptionsChangeText(k *prep, d *Decision) bool {
	if s.res.ask != raChangeText {
		return s.askOptionsStackRewrite(k, d) // act_stackrewrite.go
	}
	d.Sub = ChGenTextWord
	base := changeTextBase(&s.prog(k)[s.res.pc])
	first := int(s.res.chosen) - 1 // the first pick's mode, -1 none yet
	for i := uint8(0); i < 5; i++ {
		if first < 0 || first >= changeTextTo {
			d.add(MakeAction(AkChoose, 0, 0, base+i)) // a "from" word
		}
		if first < 0 || first < changeTextTo {
			d.add(MakeAction(AkChoose, 0, 0, changeTextTo+base+i)) // a "to" word
		}
	}
	return true
}

// applyAskChangeText keeps the first pick, then on the second applies the
// change (a link of the applyAskUnlockDoor chain).
//
//go:noinline
func (s *State) applyAskChangeText(k *prep, a Action) bool {
	if s.res.ask != raChangeText {
		return s.applyAskStackRewrite(k, a) // act_stackrewrite.go
	}
	mode := int(a.Mode())
	if s.res.chosen == 0 {
		s.res.chosen = objWord(mode + 1)
		return true // the second pick follows
	}
	first := int(s.res.chosen) - 1
	s.res.ask, s.res.chosen = raNone, 0
	from := mode // the pick that is not the "to" word names the word replaced
	if first < changeTextTo {
		from = first
	}
	from %= changeTextTo
	sl := Slot(s.res.peek[0]) // code-ok: the slot changeTextStart parked
	ChangeTextStats.Asked.Add(1)
	if from < 10 && k.t.Cards[s.cardAt(sl)].TextWords&(1<<uint(from)) != 0 {
		ChangeTextStats.Reads.Add(1)
		s.capFull(StallTextChange)
		return true
	}
	s.continueRes(k, 0)
	return true
}
