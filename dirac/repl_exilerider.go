package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The dies-to-exile rider: a ReplExileOnLeave ability that carries a Prog (the
// sub-ability chain of "instead exile that card and create a 2/2 black Zombie
// creature token", Kalitas, Traitor of Ghet). gorge resolves the replacement's
// ChangeZone body and its linked SubAbility$ chain in one walk with the
// replacing permanent as the source (rules runReplaceWith), so the exile move
// happens first and the chain after it, "You" being the source's controller
// and ReplacedCard the exiled card.
//
// The kernel splits that over two points. dieZone decides the exile and, when
// exactly one source applies and it has a rider, marks the dying slot with the
// source's slot (exileRiderMark; the cold region coldExRider, no State byte). removePerm,
// which every death ends in, reads the mark (exileRiderTake) and runs the
// source's prog once the card is in exile (exileRiderRun), in a synthetic
// resolution that names the source, so a mid-resolution caller's own
// resolution is restored after.
//
// Nothing here guesses: two or more competing replacements with a rider
// source stall instead of entering a CR 616.1 order (park), a source gone
// before the move (a simultaneous death) stalls, and a prog that asks or parks
// stalls (StallExileRider).

// useExileRider: some card's ReplExileOnLeave carries a rider prog; the
// table gets the cold mark region.
var useExileRider = newGenUse(tableHasExileRider)

func tableHasExileRider(t *kop.Table) bool {
	for i := range t.Cards {
		if exileRiderAb(&t.Cards[i]) >= 0 {
			return true
		}
	}
	return false
}

func init() { stallNames[StallExileRider] = "exilerider" }

// coldExRider is the rider source per dying slot (slot + 1, 0 none). It binds
// no view (a view is a slice header in State): the cold paths read it by of.
var coldExRider = newCold[uint8]("exRider", func(g *genPrep) int {
	if g.uses.has(useExileRider) {
		return MaxPerm
	}
	return 0
}).perSlot()

// exileRiderAb is the index of cd's ReplExileOnLeave ability that carries a
// rider prog, or -1.
func exileRiderAb(cd *kop.CardDef) int {
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbReplacement && a.Repl.Kind == kop.ReplExileOnLeave && len(a.Prog) != 0 {
			return i
		}
	}
	return -1
}

// exileRiderOn reports whether the table has riders (the mark region exists).
func exileRiderOn(k *prep) bool {
	return k.gen != nil && k.gen.uses.has(useExileRider) && len(k.gen.exileOnLeave) != 0
}

// exileRiderReset clears sl's mark: a death decision starts afresh, and a
// move that did not happen leaves none behind.
func (s *State) exileRiderReset(k *prep, sl Slot) {
	if !exileRiderOn(k) {
		return
	}
	if m := coldExRider.of(s); int(sl) < len(m) {
		m[sl] = 0
	}
}

// exileRiderMark records the rider source src of the death of sl that dieZone
// just replaced with exile; only a sole replacement (n 1) carries a rider (two
// or more stall before).
func (s *State) exileRiderMark(k *prep, sl Slot, n int, src Slot) {
	if n != 1 || src == NoSlot || !exileRiderOn(k) {
		return
	}
	if m := coldExRider.of(s); int(sl) < len(m) && exileRiderAb(&k.t.Cards[s.cardAt(src)]) >= 0 {
		m[sl] = uint8(src) + 1
	}
}

// exileRiderTake is the rider source slot of the departure of sl to z (NoSlot
// when none), clearing the mark. Only a move to exile runs it.
func (s *State) exileRiderTake(k *prep, sl Slot, z kop.Zone) Slot {
	if !exileRiderOn(k) {
		return NoSlot
	}
	marks := coldExRider.of(s)
	if int(sl) >= len(marks) || marks[sl] == 0 {
		return NoSlot
	}
	m := marks[sl]
	marks[sl] = 0
	if z != kop.ZoneExile {
		return NoSlot
	}
	return Slot(m - 1)
}

// exileRiderAmong stalls (StallExileRider) and reports true when one of the
// objects srcs, the competing dies-to-exile sources of a parked move, carries
// a rider: its order cannot be answered with the rider in it.
func (s *State) exileRiderAmong(k *prep, srcs []Obj) bool {
	if !exileRiderOn(k) {
		return false
	}
	for _, o := range srcs {
		if exileRiderAb(&k.t.Cards[s.objs[o].card]) >= 0 {
			s.capFull(StallExileRider)
			return true
		}
	}
	return false
}

// exileRiderRun runs the rider prog of the source permanent at src after o,
// the replaced card, moved to exile. The prog runs in a synthetic resolution
// of the source's replacement ability (the source's controller is "You",
// o is ReplacedCard); the enclosing resolution, if any, is restored.
func (s *State) exileRiderRun(k *prep, src Slot, o Obj) {
	if !s.occ.has(src) {
		s.capFull(StallExileRider) // the source left in the same batch
		return
	}
	c := s.cardAt(src)
	i := exileRiderAb(&k.t.Cards[c])
	if i < 0 {
		s.capFull(StallExileRider)
		return
	}
	sp := &s.perms[src]
	saved, savedRep := s.res, s.g.replaced
	s.g.replaced = o
	s.res = resCtx{
		e:      stackEntry{kind: seAbility, obj: sp.obj, srcVer: s.objs[sp.obj].ver, card: c, ab: uint8(i), ctrl: sp.ctrl, trigObj: NoObj},
		active: true, lastTok: NoObj, eachObj: NoObj,
	}
	s.res.e.tgt = noTargets
	ok := s.runPreCast(k, k.t.Cards[c].Abilities[i].Prog)
	s.res, s.g.replaced = saved, savedRep
	if !ok {
		s.capFull(StallExileRider)
	}
}
