package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// ChooseSourceStats counts the lockstep tests' coverage: asks posed, the
// asks that offered a spell or a stack ability, and answers.
var ChooseSourceStats struct {
	Asked, Spell, Ability, Answered atomic.Int64
}

// ExtChooseSource (gorge effChooseSource, engine/effects/choose_control.go):
// "a source of your choice". The resolving controller picks one damage source
// among every battlefield permanent and every spell on the stack the pool
// admits (C, the kop.ChosenSrc* / colour mask; 0 admits every source), in
// object order (gorge scans g.Objs in registration order). The pick is the
// kernel's chosen set (s.g.chosen), which the shield that follows reads
// (Ext PreventNext with kop.PrevNextChosen, repl_prevchosen.go). The choice is
// mandatory: a pool with no candidate asks nothing and records nothing, so the
// shield is not made. A single candidate is still asked (gorge poses the
// KChoose for any non-empty pool). Option kind "card" (ChGenChooseCard), the
// action's source the chosen object, as gorge's.
//
// An emblem is no battlefield or stack object, so gorge's `Card,Emblem` pool
// adds no emblem option either.

// raChooseSource is the resolution ask kind of the pick (generated from
// dirac/manifest/ask/raChooseSource).

// chosenSrcMatches reports whether a source of colours cols and types ty
// meets the mask: any named colour or type qualifies, 0 admits all.
func chosenSrcMatches(mask int, cols kop.ColorMask, ty kop.TypeMask) bool {
	if mask == 0 {
		return true
	}
	return int(cols)&mask&int(kop.ColAny) != 0 ||
		mask&kop.ChosenSrcArtifact != 0 && ty&kop.TArtifact != 0 ||
		mask&kop.ChosenSrcLand != 0 && ty&kop.TLand != 0
}

// chooseSourceCands lists the sources the pool admits in object order,
// adding each to d when it is set; the count.
func (s *State) chooseSourceCands(k *prep, mask int, d *Decision) int {
	n := 0
	for o := Obj(0); int(o) < int(s.nobj); o++ {
		ob := &s.objs[o]
		var cols kop.ColorMask
		var ty kop.TypeMask
		switch ob.zone {
		case kop.ZoneBattlefield:
			cols, ty = s.colorsGen(k, ob.slot), s.typesAt(k, ob.slot)
		case kop.ZoneStack:
			cd := &k.t.Cards[ob.card]
			cols, ty = cd.Colors, cd.Types
		default:
			continue
		}
		if !chosenSrcMatches(mask, cols, ty) {
			continue
		}
		n++
		if d != nil {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
			if ob.zone == kop.ZoneStack {
				ChooseSourceStats.Spell.Add(1)
			}
		}
	}
	// gorge's pool also holds the stack's ability objects (the resolving
	// ability itself stays on the stack while it resolves), after the
	// instances: they are created after every card of the games. They are
	// admitted by a pool with no colour or type restriction only (a colour
	// source filter reads the object's own characteristics, which an ability
	// has none of), named by their stack index, the oracle's S<i>.
	if mask == 0 {
		for i := 0; i < int(s.sp) && i < maxStackTgt; i++ {
			if s.stack[i].kind == seSpell {
				continue
			}
			n++
			if d != nil {
				d.add(MakeAction(AkChoose, StackTarget0+TCode(i), 0, 0))
				ChooseSourceStats.Ability.Add(1)
			}
		}
	}
	return n
}

// chooseSourceExt is ExtChooseSource.
//
//kmanifest:ext ExtChooseSource
func (s *State) chooseSourceExt(k *prep, in *kop.Instr) (ok, done bool) {
	s.g.nchosen = 0
	if s.chooseSourceCands(k, int(in.C), nil) == 0 {
		return true, true
	}
	s.res.ask, s.res.askP = raChooseSource, s.res.e.ctrl
	return false, true
}

// chooseSourceOptions poses the pick.
//
//kmanifest:askopts raChooseSource
func (s *State) chooseSourceOptions(k *prep, d *Decision) {
	d.Sub = ChGenChooseCard
	s.chooseSourceCands(k, int(s.prog(k)[s.res.pc].C), d)
	ChooseSourceStats.Asked.Add(1)
}

// chooseSourceAnswer records the pick and moves on.
//
//kmanifest:askapply raChooseSource
func (s *State) chooseSourceAnswer(k *prep, a Action) {
	s.res.ask = raNone
	if c := a.Source(); isStackTarget(TCode(c)) {
		// a stack ability is a source gorge's replacement never matches (the
		// damage source it publishes is the card), and the chosen set holds
		// objects only: the answer is recorded as no source, and the shield
		// that would never apply is not made
		s.g.nchosen = 0
	} else {
		s.g.chosen[0], s.g.nchosen = Obj(c), 1
	}
	ChooseSourceStats.Answered.Add(1)
	s.continueRes(k, 0)
}
