package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Per-player this-turn facts that a table reads only through a generated
// static: genState.turnF keeps one bit per fact per player, cleared at the
// turn change (turn_gen.go newTurnGen). Writes and reads are gated on
// prep.anyTurnF, so a table without either reader hashes and sizes no
// differently.
//
//   - bit p: player p flipped a coin this turn (Edgar, King of Figaro's
//     FlipCoinMod: the first flip of the turn comes up heads);
//   - bit turnExhaustShift+p: player p activated an Exhaust ability this
//     turn (Elvish Refueler's Activations static).
//
// The two-seat class uses bits 0-1 for flips and 2-3 for Exhaust (4-7 free);
// the four-seat class uses 0-3 and 4-7.
const turnFlipShift = 0

func turnFlipMask(p uint8) uint8 {
	if p >= MaxSeats {
		return 0
	}
	return 1 << (turnFlipShift + p)
}

func turnExhaustMask(p uint8) uint8 {
	if p >= MaxSeats {
		return 0
	}
	return 1 << (turnExhaustShift + p)
}

// turnFlipBit reports whether p has flipped a coin this turn.
func (s *State) turnFlipBit(p uint8) bool { return s.g.turnF&turnFlipMask(p) != 0 }

// setTurnFlip marks p's flip for this turn.
func (s *State) setTurnFlip(p uint8) { s.g.turnF |= turnFlipMask(p) }

// turnExhaustBit reports whether p has activated an Exhaust ability this
// turn.
func (s *State) turnExhaustBit(p uint8) bool { return s.g.turnF&turnExhaustMask(p) != 0 }

// setTurnExhaust marks p's Exhaust activation for this turn.
func (s *State) setTurnExhaust(p uint8) { s.g.turnF |= turnExhaustMask(p) }

// noteTurnExhaust records an Exhaust ability a activated for the source at
// sl. Called from onceUse, the one payment mark every Exhaust activation
// already makes (CR 702.177a), so no activation site can miss it.
func (s *State) noteTurnExhaust(k *prep, sl Slot, a *kop.Ability) {
	if a.Exhaust && k != nil && k.gen != nil && k.gen.anyTurnF && sl != NoSlot {
		s.setTurnExhaust(s.perms[sl].ctrl)
	}
}

// flipForced reports whether p's coin flip is forced heads by a FlipCoinMod
// static: p has flipped no coin this turn and controls a permanent carrying
// the lowered static (gorge effects.FlipCoinForced, which reads the
// controller and the canonical CheckSVar/SVarCompare/Result shape; kgen
// validates that shape at lowering).
func (s *State) flipForced(k *prep, p uint8) bool {
	if k == nil || k.gen == nil || !k.gen.anyTurnF || s.turnFlipBit(p) {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		if st.ctrl == p && st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatFlipCoinMod && s.statCond(k, st) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// exhaustOnceWaived reports whether a's activated once-only mark is waived
// by an Activations static: a is an Exhaust ability, its controller p is the
// active player, p controls a permanent carrying the lowered
// StatActivationLimit static, and p has activated no Exhaust ability this
// turn (gorge legal_loyalty.go additionalActivationLimit's MinLimit$ -1
// arm). The waived activation still marks its own once record: the caller
// sets it at payment as usual (mana_window.go), so a second activation in
// the same turn is refused.
func (s *State) exhaustOnceWaived(k *prep, p uint8, a *kop.Ability) bool {
	if k == nil || k.gen == nil || !k.gen.anyTurnF || !a.Exhaust {
		return false
	}
	if s.active != p || s.turnExhaustBit(p) {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		if st.ctrl == p && st.a.Kind == kop.AbStatic && st.a.Static.Kind == kop.StatActivationLimit && s.statCond(k, st) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// thisTurnActExhaust evaluates ExThisTurnActExhaust: 1 if ctx's controller
// activated an Exhaust ability this turn, else 0. The kernel keeps presence,
// not a count, which is all the lowered static's LT1 comparison reads.
func (s *State) thisTurnActExhaust(ctx evalCtx) int {
	if s.turnExhaustBit(ctx.ctrl) {
		return 1
	}
	return 0
}
