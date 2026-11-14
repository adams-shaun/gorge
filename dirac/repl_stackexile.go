package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ExtStackExileMark: an Effect's "if that spell would be put into a graveyard
// from the stack, exile it instead" replacement over one named spell (Lilah,
// Undefeated Slickshot, which also makes the card plotted; Nita, Forum
// Conciliator's recast of an opponent's spell). gorge registers a live
// Event$ Moved | Origin$ Stack | Destination$ Graveyard replacement carrying
// the card, ExileOnMoved$ Stack ending it as the spell leaves the stack.
//
// The kernel keeps the replacement as bits of the spell's own stack entry,
// remVer bits 4-6 (a spell has no remembered object; spellGainedDeathtouch
// is the family): spellExileLeave makes every graveyard exit of the entry
// (resolved, fizzled, countered, held) an exile (stackEntry.exilesOnLeave, the
// one predicate those exits read); spellExileResolved does so for the
// resolution move alone, as gorge's Fizzle$ False line does (a fizzle and a
// counter go to the graveyard: replacement_match.go stackLeaveByFizzle);
// spellExilePlot also makes the exiled card plotted once it is there. A copy
// keeps none (copy_gen zeroes remVer).
//
// The trigger form marks the triggering spell while it is on the stack. The
// grant form (Nita: the card is in exile and is cast later) marks the card's
// may-play permission record, until-byte bit 6, and the cast that spends that
// record carries the mark onto its entry (stampMayExile). gorge's Effect
// follows the card across zones until it leaves the stack; the kernel's mark
// lives as long as the permission (this turn), so a cast of the card by a
// later permission would differ.
const (
	spellExileLeave objWord = 1 << 4
	spellExilePlot  objWord = 1 << 5
	// spellExileResolved: spellExileLeave for the resolution exit only.
	spellExileResolved objWord = 1 << 6
)

// mayUntilStackExile marks a permission record whose cast carries
// spellExileLeave. It rides the until byte's bit 6, a limited grant's used
// count, so the record carries no cap (the grant form stalls on one).
const mayUntilStackExile uint8 = 0x40

// stackExileMark is ExtStackExileMark.
//
//go:noinline
func (s *State) stackExileMark(k *prep, in *kop.Instr) {
	c := in.C
	if c&kop.StackExileDream != 0 {
		s.capFull(StallExileOnMoved) // a dream counter on the exiled card: not held
		return
	}
	if c&kop.StackExileGrant != 0 {
		s.stackExileGrant(k)
		return
	}
	o := s.res.e.trigObj
	if o == NoObj || s.objs[o].of&ofCopy != 0 {
		return
	}
	for i := int(s.sp) - 1; i >= 0; i-- {
		e := &s.stack[i]
		if e.kind != seSpell || e.obj != o {
			continue
		}
		if s.objs[o].zone != kop.ZoneStack || c&kop.StackExileOwn != 0 && s.objs[o].owner() != s.res.e.ctrl {
			return
		}
		if e.remVer&(spellExileLeave|spellExileResolved) != 0 {
			s.capFull(StallExileOnMoved) // a second replacement on one spell: gorge asks which applies (CR 616.1)
			return
		}
		if c&kop.StackExileResolved != 0 {
			e.remVer |= spellExileResolved
		} else {
			e.remVer |= spellExileLeave
		}
		if c&kop.StackExilePlot != 0 {
			e.remVer |= spellExilePlot
		}
		return
	}
}

// stackExileGrant marks the permission record of each remembered card in
// exile.
func (s *State) stackExileGrant(k *prep) {
	for j := 0; j < int(s.g.nrem); j++ {
		o := s.g.rem[j]
		if o == NoObj || s.objs[o].zone != kop.ZoneExile {
			continue
		}
		for i := 0; i < int(s.g.nmay); i++ {
			r := s.mayAt(i)
			if r.obj != o || r.ver != s.objs[o].ver || ownWalkerRecord(*r) {
				continue
			}
			if r.kind&kop.MayPlayLimitMask != 0 || r.until&mayUntilStackExile != 0 {
				s.capFull(StallExileOnMoved) // a capped grant shares the bit; a second mark is a second replacement
				return
			}
			r.until |= mayUntilStackExile
		}
	}
}

// stampMayExile carries permission record i's mark onto the entry of the spell
// being built; the caller spends the record. A land is played, not cast.
func (s *State) stampMayExile(k *prep, i int) {
	r := s.mayAt(i)
	if r.until&mayUntilStackExile == 0 || k.t.Cards[s.objs[r.obj].card].Types&kop.TLand != 0 {
		return
	}
	s.bld().e.remVer |= spellExileLeave
}

// markExiles reports whether a spell entry's mark sends a graveyard exit of the
// given cause to exile: the any-exit mark always, the resolution-only mark
// unless the spell fizzled (a counter never asks: counterAt reads
// exilesOnLeave alone).
func (e *stackEntry) markExiles(fizzled bool) bool {
	return e.kind == seSpell && (e.remVer&spellExileLeave != 0 || e.remVer&spellExileResolved != 0 && !fizzled)
}

// exiledByMark reports whether a resolved spell entry that left the stack for
// exile did so through the plotting replacement and no other (a flashback or
// free-cast exile is the card's own).
func (e *stackEntry) exiledByMark() bool {
	return e.kind == seSpell && e.remVer&spellExilePlot != 0 && e.flags&sfFlashback == 0 &&
		!(e.flags&sfFree != 0 && e.flags&sfFreeGY != 0)
}

// markedPlot makes the card of entry e plotted once its resolution has put it
// in exile (gorge: the replacement's ChangeZone, then AlterAttribute Plotted
// on the remembered card).
func (s *State) markedPlot(k *prep, e *stackEntry) {
	if k.gen == nil || !e.exiledByMark() || s.objs[e.obj].zone != kop.ZoneExile {
		return
	}
	s.plotOne(k, e.obj)
}
