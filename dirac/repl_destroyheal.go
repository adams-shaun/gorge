package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// DebugDestroyHealOff is the mutation switch: a ReplDestroyHeal replacement
// is lowered but never applied, so a destroy takes its victim (gorge's
// Destroy ReplaceWith$ body does not run).
var DebugDestroyHealOff bool

// DebugDestroyRegenOff is the mutation switch: a ReplDestroyRegenerate
// replacement is lowered but never applied, so a destroy takes its host
// (gorge's Regeneration$ destroy line does not run).
var DebugDestroyRegenOff bool

// DestroyRegenStats counts the destructions a ReplDestroyRegenerate line
// replaced (the oracle's proof a pool reaches the line).
var DestroyRegenStats struct{ Hits atomic.Int64 }

// destroyRepl is the R:Event$ Destroy line destroyLine applied.
type destroyRepl uint8

const (
	drNone  destroyRepl = iota
	drHeal              // ReplDestroyHeal: the counters and damage left (destroyHeal)
	drRegen             // ReplDestroyRegenerate: the host regenerated
)

// rbDestroyLine is either R:Event$ Destroy line.
const rbDestroyLine = rbDestroyHeal | rbDestroyRegen

// destroyLine offers victim sl's destruction to the R:Event$ Destroy lines,
// before the regeneration shield and the shield counter (gorge
// ReplaceDestroyLine): the first line, in statM scan order, that matches
// applies (CR 616.1's choice, the deterministic stand-in gorge's
// continueDestroyReplacements takes). A ReplDestroyHeal matches another
// creature its source's controller controls with a +1/+1 counter; a
// ReplDestroyRegenerate matches its own source when regen holds (the
// destruction is not NoRegen$) and the host can be regenerated
// (replacementMatchesDestroy: no can't-be-regenerated mark, CR 701.19c, and a
// creature's toughness above 0).
func (s *State) destroyLine(k *prep, sl Slot, regen bool) destroyRepl {
	g := k.gen
	if g == nil || g.anyRepl&rbDestroyLine == 0 {
		return drNone
	}
	for m := s.statM; m.any(); m = m.rest() {
		b := m.first()
		bits := g.replBits[s.cardAt(b)] & rbDestroyLine
		if bits == 0 || s.genLost(k, b) {
			continue
		}
		if bits&rbDestroyRegen != 0 && b == sl && regen && !DebugDestroyRegenOff && s.selfRegenOK(k, sl) {
			s.touchSl(sl)
			s.regenerateBody(sl)
			DestroyRegenStats.Hits.Add(1)
			return drRegen
		}
		if bits&rbDestroyHeal != 0 && b != sl && !DebugDestroyHealOff && g.destroyHeal != nil &&
			s.matchPerm(k, g.destroyHeal[s.cardAt(b)], sl, s.perms[b].ctrl, b) {
			s.destroyHeal(k, sl)
			return drHeal
		}
	}
	return drNone
}

// selfRegenOK is replacementMatchesDestroy's Regeneration$ gate on the host
// at sl: no can't-be-regenerated mark (gorge RegenerationDisallowed) and, on
// a creature, toughness above 0.
func (s *State) selfRegenOK(k *prep, sl Slot) bool {
	if s.mSl(k, mechNoRegen, sl) {
		return false
	}
	if !s.isCreature(k, sl) {
		return true
	}
	_, t := s.pt(k, sl)
	return t > 0
}

// destroyHeal replaces victim sl's destruction: remove all its +1/+1
// counters and heal all damage marked on it (Dr. McCoy, Surly Surgeon).
// destroyLine runs it before regeneration and the shield counter (gorge
// effDestroy's ReplaceDestroyLine, then ReplaceDestruction and
// ReplaceShieldCounter).
//
//go:noinline
func (s *State) destroyHeal(k *prep, sl Slot) {
	p := &s.perms[sl]
	s.touchSl(sl)
	if p.flags&pfDeathtouched != 0 {
		p.cseen |= cseenDT // gorge lists the removed mark at 0
	}
	p.seeCtr(s)
	p.subCtrN(s, int(kop.CtrP1P1), p.ctrN(s, int(kop.CtrP1P1)))
	p.damage = 0
	p.flags &^= pfDeathtouched | pfDtMore
}

// destroyLineAll is OpDestroyAll's destroy-line pass (before regeneration):
// each victim that an R:Event$ Destroy line replaces leaves hit (regen as
// destroyLine's).
func (s *State) destroyLineAll(k *prep, hit slotMask, regen bool) slotMask {
	if k.gen == nil || k.gen.anyRepl&rbDestroyLine == 0 {
		return hit
	}
	var out slotMask
	for m := hit; m.any(); m = m.rest() {
		sl := m.first()
		if s.destroyLine(k, sl, regen) == drNone {
			out = out.with(sl)
		}
	}
	return out
}
