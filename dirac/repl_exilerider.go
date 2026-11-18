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
// exactly one source applies and it has a rider, records the source's identity
// on the dying slot (exileRiderMark; the cold region coldExRider, no State
// byte), while the batch's pre-move scan still sees it: a source may leave
// before a later member of the same batch moves (two opposing Kalitas dying
// together), and its slot may be reused by a token since. removePerm, which
// every death ends in, reads the record (exileRiderTake) and runs the source's
// prog once the card is in exile (exileRiderRun), in a synthetic resolution
// that names the source, so a mid-resolution caller's own resolution is
// restored after.
//
// Nothing here guesses: two or more competing replacements with a rider
// source stall instead of entering a CR 616.1 order (park), and a prog that
// asks or parks stalls (StallExileRider).

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

// exRiderMark is one dying slot's rider record: the source slot + 1 (0 none)
// and the source's identity as the pre-move scan saw it — object, controller
// ("You" of the prog), object version and card. The source may be gone, or its
// slot reused, by the time the rider runs.
type exRiderMark struct {
	src  uint8  // the source slot + 1; 0 none
	obj  Obj    // the source's object
	ctrl uint8  // the source's controller
	ver  uint8  // the source object's ver at the mark
	card CardID // the source's card (last known information)
}

// coldExRider is the rider record per dying slot. It binds no view (a view is
// a slice header in State): the cold paths read it by of.
var coldExRider = newCold[exRiderMark]("exRider", func(g *genPrep) int {
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

// exileRiderReset clears sl's record: a death decision starts afresh, and a
// move that did not happen leaves none behind.
func (s *State) exileRiderReset(k *prep, sl Slot) {
	if !exileRiderOn(k) {
		return
	}
	if m := coldExRider.of(s); int(sl) < len(m) {
		m[sl] = exRiderMark{}
	}
}

// exileRiderMark records the rider source src of the death of sl that dieZone
// just replaced with exile; only a sole replacement (n 1) carries a rider (two
// or more stall before). The source is on the battlefield here (dieZone's
// scan runs before any member of the batch moves), so the record holds what
// the rider needs even after it leaves.
func (s *State) exileRiderMark(k *prep, sl Slot, n int, src Slot) {
	if n != 1 || src == NoSlot || !exileRiderOn(k) {
		return
	}
	m := coldExRider.of(s)
	if int(sl) >= len(m) {
		return
	}
	c := s.cardAt(src)
	if exileRiderAb(&k.t.Cards[c]) < 0 {
		return
	}
	sp := &s.perms[src]
	m[sl] = exRiderMark{src: uint8(src) + 1, obj: sp.obj, ctrl: sp.ctrl, ver: s.objs[sp.obj].ver, card: c}
}

// exileRiderTake is the rider record of the departure of sl to z (src 0 when
// none), clearing it. Only a move to exile runs it.
func (s *State) exileRiderTake(k *prep, sl Slot, z kop.Zone) exRiderMark {
	if !exileRiderOn(k) {
		return exRiderMark{}
	}
	marks := coldExRider.of(s)
	if int(sl) >= len(marks) || marks[sl].src == 0 {
		return exRiderMark{}
	}
	m := marks[sl]
	marks[sl] = exRiderMark{}
	if z != kop.ZoneExile {
		return exRiderMark{}
	}
	return m
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

// exileRiderRun runs the rider prog recorded for the dying slot after o, the
// replaced card, moved to exile. The record holds the source's identity as
// the pre-move scan saw it, so the prog runs even when the source left in the
// same batch (two opposing Kalitas dying together; gorge holds the
// replacement's source the same way) or its slot was reused since. The prog
// runs in a synthetic resolution of the source's replacement ability (the
// source's controller is "You", o is ReplacedCard); the enclosing resolution,
// if any, is restored.
func (s *State) exileRiderRun(k *prep, m exRiderMark, o Obj) {
	c := m.card
	i := exileRiderAb(&k.t.Cards[c])
	if i < 0 {
		s.capFull(StallExileRider)
		return
	}
	saved, savedRep := s.res, s.g.replaced
	s.g.replaced = o
	s.res = resCtx{
		e:      stackEntry{kind: seAbility, obj: m.obj, srcVer: m.ver, card: c, ab: uint8(i), ctrl: m.ctrl, trigObj: NoObj},
		active: true, lastTok: NoObj, eachObj: NoObj,
	}
	s.res.e.tgt = noTargets
	ok := s.runPreCast(k, k.t.Cards[c].Abilities[i].Prog)
	s.res, s.g.replaced = saved, savedRep
	if !ok {
		s.capFull(StallExileRider)
	}
}
