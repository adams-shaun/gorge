package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kop.StatColorWords (Swirl the Mists, CR 612.5): while the source is on the
// battlefield with a recorded colour, every colour word in the text of the
// spells on the stack and the permanents on the battlefield is that colour
// word. The kernel holds no per-object text, so it rewrites the colour words
// where the faces are read and stalls where it does not, exactly as
// ExtChangeText does (act_changetext.go).
//
// The rewrite is a substitution over the colour words a face names, applied
// where gorge's is (rules colorwords_sync.go): a target spec's filter masks
// and a printed Protection keyword's colours of an AFFECTED face -- a spell
// on the stack or a permanent on the battlefield (the lowering pins
// Affected$ Spell,Permanent in AffectedZone$ Stack,Battlefield,
// lower_colorwords.go) -- read with every word they name replaced by the
// composed colour (cwMask). A face in another zone, and a mask bit that is
// not one of the face's words (CardDef.TextWords), is read unchanged; a
// source with no recorded colour contributes nothing, as gorge's
// colourWordEffects fails closed on an unanswered ask.
//
// What the rewrite cannot reach keeps the stall: a card whose rewritten word
// also sits on a surface the model does not read (CardDef.CWUnmodelled, the
// type line, the SVars, any ability, trigger, static or replacement parameter
// beside a target spec) stalls StallTextChange (unmodelled) at the same two
// checkpoints as before -- pushBuild (a spell takes the change as it reaches
// the stack, before its targets are asked) and the priority grant (before a
// player may act or a trigger's targets are asked; the walk covers every
// permanent and spell already in play). Both sit behind prep.anyColorWords,
// so a pool with no such static pays one branch.

// ColorWordsStats counts the mechanism for the lockstep tests: Live is each
// priority grant that found a StatColorWords source with a recorded colour,
// Stalls those that stalled on a card reading another colour word on a
// surface the model does not rewrite, Casts the spells whose cast stalled,
// Reads the mask reads the model rewrote, Installs the word-list Animate
// Colors$ applications that read a frozen colour-word colour (ovCwRow).
var ColorWordsStats struct{ Live, Stalls, Casts, Reads, Installs atomic.Int64 }

// colorWordsBites is the colour words (CardDef.TextWords bits 0..4) the live
// StatColorWords sources rewrite: every word but a source's chosen one, over
// all the sources (the union is a superset of the words gorge's chained
// substitutions change, so a stall where gorge changes nothing is safe).
// 0 with no live source.
//
//go:noinline
func (s *State) colorWordsBites(k *prep) uint16 {
	var bites uint16
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatColorWords || st.sl == NoSlot {
			return true
		}
		if ch := uint8(s.perms[st.sl].chosen); ch < 5 {
			bites |= 0x1f &^ (1 << ch)
		}
		return true
	})
	return bites
}

// cwTo is the colour the live colour-word statics compose to. gorge applies
// each static's substitutions to the evolving text in active-effect order
// (chars/derive.go's LText arm: the battlefield list's timestamp order), and
// each static's substitution set sends every word it replaces to its own
// chosen word, so every colour word of an affected face reads the colour of
// the live source with the greatest timestamp (a slot reused after a
// departure breaks slot order, so the timestamp decides); a source with no
// recorded choice contributes none, and with no live source the reads are
// unchanged.
func (s *State) cwTo(k *prep) (uint8, bool) {
	to, bestSl := uint8(255), NoSlot
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatColorWords || st.sl == NoSlot {
			return true
		}
		ch := uint8(s.perms[st.sl].chosen)
		if ch >= 5 {
			return true
		}
		// gorge's active effects run in battlefield-list (timestamp) order, a
		// slot reused after a departure breaks slot order (eachStaticTS)
		if bestSl == NoSlot || s.perms[st.sl].ts > s.perms[bestSl].ts || s.perms[st.sl].ts == s.perms[bestSl].ts && st.sl > bestSl {
			to, bestSl = ch, st.sl
		}
		return true
	})
	return to, to < 5
}

// cwFace is the card whose face object o reads in its zone: its battlefield
// face, else its card as a spell (or a copy of one) on the stack, else (an
// ability's source that left) the source card as the ability recorded it.
func (s *State) cwFace(k *prep, o Obj) CardID {
	ob := &s.objs[o]
	if ob.zone == kop.ZoneBattlefield {
		return s.cardAt(ob.slot)
	}
	if ob.zone != kop.ZoneStack {
		if e := s.curEntry(); e != nil && e.kind != seSpell && e.obj == o {
			return e.card
		}
	}
	return ob.card
}

// cwAffected reports whether object o carries the colour-word rewrite: a
// spell on the stack, a permanent on the battlefield, or the resolving
// ability whose source has left (gorge keeps an ability's text as it was put
// on the stack, so the source's rewritten face outlives the source).
func (s *State) cwAffected(o Obj) bool {
	z := s.objs[o].zone
	if z == kop.ZoneBattlefield || z == kop.ZoneStack {
		return true
	}
	e := s.curEntry()
	return e != nil && e.kind != seSpell && e.obj == o
}

// cwMask rewrites the colour mask a face read attributes to card c on object
// o as the live StatColorWords statics rewrite that face: every mask bit that
// is one of the face's colour words (CardDef.TextWords bits 0..4) is replaced
// by the composed colour, every other bit is read unchanged. o must be an
// affected object (a spell on the stack or a permanent on the battlefield,
// CR 612.5); the second result reports whether the mask moved.
func (s *State) cwMask(k *prep, c CardID, o Obj, m kop.ColorMask) (kop.ColorMask, bool) {
	if !k.anyColorWords || m == 0 || o == NoObj {
		return m, false
	}
	if !s.cwAffected(o) {
		return m, false
	}
	to, _ := s.cwTo(k)
	return s.cwMaskAt(k, c, m, to)
}

// cwEachColour is the mask of all five colours: the lowering of Colorless and
// nonColorless (kgen filter.go), a word-free spelling no colour-word static
// rewrites, so a read of it stays as printed.
const cwEachColour = kop.ColW | kop.ColU | kop.ColB | kop.ColR | kop.ColG

// cwMaskAt is cwMask's rewrite over an already-affected face.
func (s *State) cwMaskAt(k *prep, c CardID, m kop.ColorMask, to uint8) (kop.ColorMask, bool) {
	if m == 0 || to >= 5 {
		return m, false
	}
	if m == cwEachColour {
		return m, false // Colorless / nonColorless: no colour word names it
	}
	aff := kop.ColorMask(k.t.Cards[c].TextWords & 0x1f)
	if aff == 0 {
		return m, false
	}
	if m&aff == 0 {
		return m, false
	}
	ColorWordsStats.Reads.Add(1)
	return (m &^ aff) | kop.ColorMask(1)<<to, true
}

// cwMasks is cwMaskAt over a filter's colour pair, the face of the object
// textObj reads: a spec may carry only one of the two (Doom Blade's
// Creature.nonBlack is NotColors alone), so each is rewritten on its own.
func (s *State) cwMasks(k *prep, textObj Obj, a, b kop.ColorMask) (kop.ColorMask, kop.ColorMask) {
	if !k.anyColorWords || textObj == NoObj || a == 0 && b == 0 {
		return a, b
	}
	if !s.cwAffected(textObj) {
		return a, b
	}
	c := s.cwFace(k, textObj)
	to := s.cwToFor(k, textObj)
	a, _ = s.cwMaskAt(k, c, a, to)
	b, _ = s.cwMaskAt(k, c, b, to)
	return a, b
}

// cwFilterMasks is a card filter's colour pair as the colour-word statics
// rewrite the face the read is attributed to: ctx.cwObj, set only where the
// filter is that face's own (a target spec's funnel, matchCardTgt and
// matchCardAtTgt; protMatch's printed quality: the protected card's face). A
// read with no such face -- another permanent's cost static or any filter the
// entry being built or resolved does not own -- reads its masks as printed.
func (s *State) cwFilterMasks(k *prep, ctx *evalCtx, a, b kop.ColorMask) (kop.ColorMask, kop.ColorMask) {
	if !k.anyColorWords || a == 0 && b == 0 {
		return a, b
	}
	if ctx == nil {
		return a, b
	}
	return s.cwMasks(k, ctx.cwObj, a, b)
}

// cwFreeze is the colour the colour-word statics composed to as an ability
// is put on the stack, stored in the entry's prov byte (a spell's cast
// provenance, unused by an ability): 1 = no live colour, 2+c = colour c. gorge
// builds an ability from its source's face as it is when the ability is put
// on the stack (CR 112.7a, 608.2h) and the stack object keeps those rewritten
// parameters whatever happens to the source or the static afterwards.
func (s *State) cwFreeze(k *prep) uint8 {
	if to, live := s.cwTo(k); live {
		return 2 + to
	}
	return 1
}

// cwToFor is the composed colour a read of object o's face uses: the frozen
// colour of the ability on the stack o is the source of (cwFreeze), else the
// live one.
func (s *State) cwToFor(k *prep, o Obj) uint8 {
	if e := s.curEntry(); e != nil && e.kind != seSpell && e.obj == o && e.prov != 0 {
		if e.prov >= 2 {
			return e.prov - 2
		}
		return 255
	}
	to, _ := s.cwTo(k)
	return to
}

// cwFrozen is the colour-word colour the resolving ability's text was frozen
// at (cwFreeze, the entry's prov byte): 255 when none. gorge reads an
// ability's parameters from its face as it was put on the stack (CR 112.7a,
// 608.2h), so a spell's (SP$) has no frozen colour and a later static change
// moves nothing already resolved.
func (s *State) cwFrozen(k *prep) uint8 {
	e := s.curEntry()
	if e == nil || e.kind == seSpell || e.prov < 2 {
		return 255
	}
	return e.prov - 2
}

// ovCwRow is the OpOverride / OpOverrideAll word-list row pick
// (interp_gen.go, dispatch.go): B = OvCwWord's C names the FIRST of six
// override records, the printed colours then one colour each (W, U, B, R, G;
// kgen emitCwWordAnimate / emitCwWordAll), and the colour-word colour frozen
// as the ability was put on the stack (cwFrozen) picks the row, so an Animate
// Colors$ reads the colour its text was frozen at and a later static change
// moves nothing installed. The printed row applies with none. It counts the
// pick for the lockstep tests and returns the instruction to run, a per-colour
// copy in the caller's ci when one was picked (the program is shared, so the
// table instr is never written).
func (s *State) ovCwRow(k *prep, in *kop.Instr, ci *kop.Instr) *kop.Instr {
	to := s.cwFrozen(k)
	if to >= 5 {
		return in
	}
	ColorWordsStats.Installs.Add(1)
	*ci = *in
	ci.C += int16(1 + to)
	return ci
}

// cwTextObj is the object whose face a filter read at source slot src
// interprets under the colour-word statics: the card at src (an ability's, a
// trigger's or a static's source on the battlefield; a source that left reads
// its unchanged face, gorge's sync keeps the substitutions on no other
// object). None with no source slot: a read that names no face reads as
// printed (cwSpecObj derives the entry's own face for a target spec).
func (s *State) cwTextObj(k *prep, src Slot) Obj {
	if src == NoSlot || !s.occ.has(src) {
		return NoObj
	}
	return s.perms[src].obj
}

// cwSpecObj is the object whose face a TARGET SPEC read interprets: the card
// at src (cwTextObj), else the spell being resolved or built, whose own spec is
// read with NoSlot (srcSlot: cast.go) -- or, for an ability or trigger whose
// source left (and perhaps returned: xtLegal's NoSlot), that source as it left
// or as it is now on the battlefield. None elsewhere and with no colour-word
// static in the pool.
func (s *State) cwSpecObj(k *prep, src Slot) Obj {
	if !k.anyColorWords {
		return NoObj
	}
	if src != NoSlot {
		return s.cwTextObj(k, src)
	}
	if e := s.curEntry(); e != nil {
		if e.kind == seSpell {
			if s.objs[e.obj].zone == kop.ZoneStack {
				return e.obj
			}
			return NoObj
		}
		return e.obj // an ability: its source's face, on the battlefield or as it left
	}
	return NoObj
}

// matchPermTgt is matchPerm for a target spec's own read: a spec with no source
// slot (a spell's, an ability whose source left) reads the colour masks of the
// entry's face (cwSpecObj); every other perm read keeps matchPerm's, which
// rewrites only the face of its source slot.
func (s *State) matchPermTgt(k *prep, f kop.FilterID, sl Slot, ctrl uint8, src Slot) bool {
	if src == NoSlot && f != 0 && k.anyColorWords {
		if o := s.cwSpecObj(k, NoSlot); o != NoObj {
			return s.matchPermGenCw(k, f, sl, ctrl, src, layerFull, o)
		}
	}
	return s.matchPerm(k, f, sl, ctrl, src)
}

// matchCardAtTgt is matchCardAt for a spell target spec's own read (cwSpecObj's
// face).
func (s *State) matchCardAtTgt(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o Obj, src Slot) bool {
	if f != 0 && k.anyColorWords {
		if to := s.cwSpecObj(k, src); to != NoObj {
			ctx := s.ctxCur()
			ctx.cwObj = to
			return s.matchCardGenAt(k, f, c, owner, ctrl, o, &ctx)
		}
	}
	return s.matchCardAt(k, f, c, owner, ctrl, o)
}

// colorWordsStall stalls the game when a permanent or a spell on the stack
// reads a rewritten colour word on a surface the model does not rewrite (the
// priority checkpoint). It reports whether it stalled.
//
//go:noinline
func (s *State) colorWordsStall(k *prep) bool {
	bites := s.colorWordsBites(k)
	if bites == 0 {
		return false
	}
	ColorWordsStats.Live.Add(1)
	hit := false
	for m := s.occ; m.any() && !hit; m = m.rest() {
		hit = k.t.Cards[s.cardAt(m.first())].CWUnmodelled&bites != 0
	}
	for i := 0; i < int(s.sp) && !hit; i++ {
		if e := &s.stack[i]; e.kind == seSpell {
			hit = k.t.Cards[e.card].CWUnmodelled&bites != 0
		}
	}
	if hit {
		ColorWordsStats.Stalls.Add(1)
		s.capFull(StallTextChange)
	}
	return hit
}

// colorWordsCast is the cast checkpoint: pushBuild calls it as the spell
// reaches the stack, before its modes and targets are asked. A pool with no
// colour-word static pays this one inlined branch.
func (s *State) colorWordsCast(k *prep, c CardID) {
	if k.anyColorWords {
		s.colorWordsCastStall(k, c)
	}
}

// colorWordsCastStall stalls the game when the card being cast reads a
// rewritten colour word on a surface the model does not rewrite.
//
//go:noinline
func (s *State) colorWordsCastStall(k *prep, c CardID) {
	bites := s.colorWordsBites(k)
	if bites == 0 || k.t.Cards[c].CWUnmodelled&bites == 0 {
		return
	}
	ColorWordsStats.Casts.Add(1)
	s.capFull(StallTextChange)
}
